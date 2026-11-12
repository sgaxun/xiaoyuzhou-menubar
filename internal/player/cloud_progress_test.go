package player

import (
	"net/http"
	"testing"
	"time"
)

func TestCloudResumeMergeProtectsLocalAndPlayback(t *testing.T) {
	s := cloudTestService(t)
	s.state["queue"] = []any{object{"eid": "remote", "duration": 1000}, object{"eid": "local"}, object{"eid": "playing"}, object{"eid": "pending"}, object{"eid": "zero"}}
	s.state["progress"] = object{"remote": object{"position": 900, "updatedAt": "2026-11-10T00:00:00Z"}, "local": object{"position": 50, "updatedAt": "2026-11-12T00:00:00Z"}, "zero": object{"position": 500, "updatedAt": "2026-11-10T00:00:00Z"}}
	s.cloudPlayback = "playing"
	s.cloudPlaybackEvent = "playing"
	s.cloudStore()["tasks"] = object{"a:pending": object{"position": 40}}
	records := []any{}
	for _, id := range []string{"remote", "local", "playing", "pending", "zero", "outside"} {
		pos := 100.0
		if id == "zero" {
			pos = 0
		}
		records = append(records, object{"eid": id, "progress": pos, "playedAt": "2026-11-11T00:00:00Z"})
	}
	s.cloudPlaylistPreview = object{"id": "test", "epoch": s.accountEpoch, "createdAt": nowISO(), "records": records}
	next := clone(s.state)
	applied, err := s.mergeCloudPlaylistProgress(next, object{"cloudImportId": "test"})
	if err != nil || len(applied) != 2 || num(obj(applied["remote"])["position"]) != 100 || num(obj(applied["zero"])["position"]) != 0 {
		t.Fatal(applied, err)
	}
	if num(obj(obj(next["progress"])["local"])["position"]) != 50 {
		t.Fatal("new local progress replaced")
	}
	if len(list(next["history"])) != 0 || len(obj(obj(next["cloudSync"])["tasks"])) != 1 {
		t.Fatal("import triggered history/upload")
	}
	if num(obj(obj(s.state["progress"])["remote"])["position"]) != 900 {
		t.Fatal("preview mutated store")
	}
	s.cloudPlaylistPreview["epoch"] = s.accountEpoch + 1
	if _, err = s.mergeCloudPlaylistProgress(next, object{"cloudImportId": "test"}); err == nil {
		t.Fatal("cross account import")
	}
	s.cloudPlaylistPreview["epoch"] = s.accountEpoch
	s.cloudPlaylistPreview["createdAt"] = time.Now().Add(-6 * time.Minute).UTC().Format(time.RFC3339Nano)
	if _, err = s.mergeCloudPlaylistProgress(next, object{"cloudImportId": "test"}); err == nil {
		t.Fatal("stale import")
	}
}

func TestCloudResumeCommitProtectsLoadedMediaAndTimestamp(t *testing.T) {
	s := cloudTestService(t)
	s.cloudPlaylistPreview = object{"id": "test", "epoch": s.accountEpoch, "createdAt": nowISO(), "records": []any{object{"eid": "one", "progress": 90.0, "playedAt": "2026-11-11T00:00:00Z"}}}
	r := call(t, s, "saveQueue", object{"queue": []any{object{"eid": "one"}}, "cloudImportId": "test", "protectedEid": "one"})
	if !flag(r["ok"]) || len(obj(r["importedProgress"])) != 0 {
		t.Fatal(r)
	}
	r = call(t, s, "saveQueue", object{"queue": []any{object{"eid": "one"}}, "cloudImportId": "test"})
	if num(obj(obj(r["importedProgress"])["one"])["position"]) != 90 {
		t.Fatal(r)
	}
	if str(obj(obj(s.state["progress"])["one"])["updatedAt"]) != "2026-11-11T00:00:00Z" {
		t.Fatal("import restamped as local listening")
	}
}

func TestCloudPlaylistProgressReadAndFailureFallback(t *testing.T) {
	for _, fail := range []bool{false, true} {
		s := cloudTestService(t)
		const id = "abcdef0123456789abcdef01"
		const pid = "abcdef0123456789abcdef02"
		s.state["queue"] = []any{object{"eid": id, "pid": pid, "duration": 1000}}
		s.client.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/v1/episode-played/list-history":
				return reply(200, `{"data":[]}`), nil
			case "/v1/playlist/pull":
				return reply(200, `{"data":{"kind":"FULL_LIST","list":["`+id+`"]}}`), nil
			case "/v1/playback-progress/list":
				if fail {
					return reply(503, `{}`), nil
				}
				return reply(200, `{"data":[{"eid":"`+id+`","pid":"`+pid+`","progress":123,"playedAt":"2026-11-11T00:00:00Z"}]}`), nil
			default:
				t.Fatalf("unexpected write/network call: %s", r.URL.Path)
				return nil, nil
			}
		})}
		preview := call(t, s, "pullCloudPlaylist", nil)
		if !flag(preview["ok"]) || len(list(preview["queue"])) != 1 || (str(preview["progressWarning"]) != "") != fail {
			t.Fatal(preview)
		}
		if len(obj(s.state["progress"])) != 0 {
			t.Fatal("preview mutated progress")
		}
		saved := call(t, s, "saveQueue", object{"queue": preview["queue"], "cloudImportId": preview["cloudImportId"]})
		count := len(obj(saved["importedProgress"]))
		if !flag(saved["ok"]) || (fail && count != 0) || (!fail && count != 1) {
			t.Fatal(saved)
		}
		if len(obj(s.cloudStore()["tasks"])) != 0 || len(list(s.state["history"])) != 0 {
			t.Fatal("read created history or upload")
		}
	}
}
