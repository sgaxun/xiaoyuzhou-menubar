package player

import (
	"net/http"
	"testing"
)

func TestCloudFinishedRemovalIgnoresOriginAndSelectedState(t *testing.T) {
	const gone = "abcdef0123456789abcdef01"
	const manual = "abcdef0123456789abcdef02"
	const kept = "abcdef0123456789abcdef03"
	for _, playing := range []bool{false, true} {
		s := cloudTestService(t)
		s.state["queue"] = []any{object{"eid": gone}, object{"eid": manual}, object{"eid": kept}}
		s.state["cloudQueueSources"] = object{"a": object{gone: true}}
		s.cloudPlaylistPreview = object{"id": "test", "epoch": s.accountEpoch, "createdAt": nowISO(), "records": []any{}, "removeIDs": []any{gone, manual}}
		body := object{"queue": s.state["queue"], "cloudImportId": "test", "currentEid": gone, "protectedEid": gone}
		if playing {
			body["playingEid"] = gone
		}
		result := call(t, s, "saveQueue", body)
		found := map[string]bool{}
		for _, v := range list(result["queue"]) {
			found[str(obj(v)["eid"])] = true
		}
		if !flag(result["ok"]) || found[gone] != playing || found[manual] || !found[kept] {
			t.Fatal(result)
		}
		if _, ok := s.state["cloudQueueSources"]; ok {
			t.Fatal("obsolete source tracking retained")
		}
		if !playing && num(result["index"]) != -1 {
			t.Fatal("removed selected item left stale index")
		}
	}
}

func TestCloudFinishedRemovalReadOnlyPreviewAndFailure(t *testing.T) {
	const gone = "abcdef0123456789abcdef01"
	const manual = "abcdef0123456789abcdef02"
	for _, scenario := range []string{"finished", "stillRemote", "unfinished", "failure", "unknown"} {
		s := cloudTestService(t)
		s.state["queue"] = []any{object{"eid": gone}, object{"eid": manual}}
		// No cloud source markers: legacy and manually-added episodes must work too.
		historyCalls := 0
		s.client.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/v1/playlist/pull":
				if scenario == "stillRemote" {
					return reply(200, `{"data":{"kind":"FULL_LIST","list":["`+gone+`"]}}`), nil
				}
				return reply(200, `{"data":{"kind":"FULL_LIST","list":[]}}`), nil
			case "/v1/playback-progress/list":
				return reply(200, `{"data":[]}`), nil
			case "/v1/episode-played/list-history":
				historyCalls++
				switch scenario {
				case "failure":
					return reply(503, `{}`), nil
				case "unknown":
					return reply(200, `{"data":[]}`), nil
				case "finished", "stillRemote":
					return reply(200, `{"data":[{"episode":{"eid":"`+gone+`","isFinished":true}},{"episode":{"eid":"`+manual+`","isFinished":true}}]}`), nil
				default:
					return reply(200, `{"data":[{"episode":{"eid":"`+gone+`","isFinished":false}}]}`), nil
				}
			default:
				t.Fatalf("unexpected network request %s", r.URL.Path)
				return nil, nil
			}
		})}
		preview := call(t, s, "pullCloudPlaylist", object{"currentEid": manual})
		if !flag(preview["ok"]) || historyCalls != 1 || len(list(s.state["queue"])) != 2 {
			t.Fatal(preview)
		}
		result := call(t, s, "saveQueue", object{"queue": preview["queue"], "cloudImportId": preview["cloudImportId"]})
		expected := 2
		if scenario == "finished" || scenario == "stillRemote" {
			expected = 0
		}
		if len(list(result["queue"])) != expected {
			t.Fatal(scenario, result)
		}
		if scenario == "failure" && str(preview["progressWarning"]) == "" {
			t.Fatal("missing failure notice")
		}
	}
}
