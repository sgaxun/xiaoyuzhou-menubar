package player

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func cloudTestService(t *testing.T) *Service {
	t.Helper()
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.state["user"] = object{"uid": "a"}
	if e = s.writeTokens("token", ""); e != nil {
		t.Fatal(e)
	}
	call(t, s, "cloudSyncSettings", object{"enabled": true})
	return s
}
func event(t *testing.T, s *Service, kind, id string, pos float64) {
	t.Helper()
	r := call(t, s, "playbackEvent", object{"event": kind, "episode": object{"eid": id, "pid": "pod"}, "position": pos, "playedAt": "2026-11-11T01:02:03Z"})
	if !flag(r["ok"]) {
		t.Fatal(r)
	}
}
func TestCloudPauseSwitchResumeAndStaleEvents(t *testing.T) {
	s := cloudTestService(t)
	event(t, s, "playing", "one", 1)
	event(t, s, "paused", "one", 2)
	first := clone(obj(obj(s.cloudStore()["tasks"])["a:one"]))
	event(t, s, "paused", "one", 3)
	if str(obj(obj(s.cloudStore()["tasks"])["a:one"])["revision"]) != str(first["revision"]) {
		t.Fatal("duplicate pause reset timer")
	}
	event(t, s, "playing", "two", 0)
	event(t, s, "ended", "one", 99)
	if len(obj(s.cloudStore()["tasks"])) != 1 {
		t.Fatal("switch removed prior pause")
	}
	event(t, s, "playing", "one", 3)
	if len(obj(s.cloudStore()["tasks"])) != 0 {
		t.Fatal("resume retained own timer")
	}
	event(t, s, "ended", "one", 4)
	event(t, s, "paused", "one", 4)
	task := obj(obj(s.cloudStore()["tasks"])["a:one"])
	if str(task["event"]) != "ended" || str(task["playedAt"]) != "2026-11-11T01:02:03Z" {
		t.Fatal(task)
	}
}
func TestCloudAccountQuarantineAndManualRetry(t *testing.T) {
	s := cloudTestService(t)
	event(t, s, "playing", "one", 1)
	event(t, s, "paused", "one", 2)
	task := obj(obj(s.cloudStore()["tasks"])["a:one"])
	task["error"] = "offline"
	s.cancelCloudAccount()
	s.state["user"] = object{"uid": "b"}
	call(t, s, "retryCloudSync", nil)
	if str(obj(obj(s.cloudStore()["tasks"])["a:one"])["error"]) != "offline" || num(s.cloudSyncView()["pending"]) != 0 {
		t.Fatal("cross account retry")
	}
	s.state["user"] = object{"uid": "a"}
	call(t, s, "retryCloudSync", nil)
	if str(obj(obj(s.cloudStore()["tasks"])["a:one"])["error"]) != "" {
		t.Fatal("retry failed")
	}
}
func TestCloudRestartRecoversActiveProgress(t *testing.T) {
	s := cloudTestService(t)
	event(t, s, "playing", "one", 1)
	call(t, s, "progress", object{"eid": "one", "position": 42.0})
	s2, e := New(s.dir)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s2.StartCloudSync(ctx); close(done) }()
	deadline := time.Now().Add(time.Second)
	for {
		s2.mu.Lock()
		task := clone(obj(obj(s2.cloudStore()["tasks"])["a:one"]))
		s2.mu.Unlock()
		if len(task) > 0 {
			if num(task["position"]) != 42 {
				t.Fatal(task)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no recovered pause")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
}
func TestCloudCompletedProgressDedup(t *testing.T) {
	s := cloudTestService(t)
	s.cloudStore()["completed"] = object{"a:one": object{"position": 12, "historyDone": true}}
	event(t, s, "playing", "one", 12)
	event(t, s, "ended", "one", 12)
	if len(obj(s.cloudStore()["tasks"])) != 0 {
		t.Fatal("completed progress uploaded again")
	}
}

func TestCloudNetworkPartialFailureExplicitRetry(t *testing.T) {
	s := cloudTestService(t)
	const eid = "abcdef0123456789abcdef01"
	const pid = "abcdef0123456789abcdef02"
	progress, history := 0, 0
	s.client.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/playback-progress/update":
			progress++
			return reply(200, `{}`), nil
		case "/v1/episode-played/create":
			history++
			if history == 1 {
				return reply(500, `{"error":"offline"}`), nil
			}
			return reply(200, `{}`), nil
		}
		t.Fatalf("unexpected endpoint %s", r.URL.Path)
		return nil, nil
	})}
	call(t, s, "playbackEvent", object{"event": "playing", "episode": object{"eid": eid, "pid": pid}, "position": 12.0})
	call(t, s, "playbackEvent", object{"event": "paused", "episode": object{"eid": eid, "pid": pid}, "position": 12.0})
	task := clone(obj(obj(s.cloudStore()["tasks"])["a:"+eid]))
	s.runCloudTask(context.Background(), "token", task, s.accountEpoch)
	if progress != 1 || history != 1 {
		t.Fatalf("calls %d %d", progress, history)
	}
	event(t, s, "playing", eid, 12)
	event(t, s, "paused", eid, 12)
	task = clone(obj(obj(s.cloudStore()["tasks"])["a:"+eid]))
	if !flag(task["progressDone"]) || str(task["error"]) == "" {
		t.Fatal("resume lost partial failure", task)
	}
	s2, e := New(s.dir)
	if e != nil {
		t.Fatal(e)
	}
	s2.client = s.client
	call(t, s2, "retryCloudSync", nil)
	task = clone(obj(obj(s2.cloudStore()["tasks"])["a:"+eid]))
	s2.runCloudTask(context.Background(), "token", task, s2.accountEpoch)
	if history != 1 {
		t.Fatal("manual retry bypassed persisted cooldown")
	}
	cloudAccount(s2.cloudStore(), "a")["nextAllowedAt"] = time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)
	s2.runCloudTask(context.Background(), "token", task, s2.accountEpoch)
	if progress != 1 || history != 2 || len(obj(s2.cloudStore()["tasks"])) != 0 {
		t.Fatalf("partial retry repeated progress: %d %d", progress, history)
	}
}
func TestCloudFreshPauseDeadlineAndPreflight(t *testing.T) {
	s := cloudTestService(t)
	event(t, s, "playing", "one", 1)
	before := time.Now()
	event(t, s, "paused", "one", 2)
	task := obj(obj(s.cloudStore()["tasks"])["a:one"])
	due, e := time.Parse(time.RFC3339Nano, str(task["dueAt"]))
	if e != nil || due.Sub(before) < 5*time.Minute || due.Sub(before) > 5*time.Minute+time.Second {
		t.Fatal("pause deadline", task)
	}
	called := false
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if s.cloudAuthorized(ctx, s.accountEpoch, "token", func(string) error { called = true; return nil }) == nil || called {
		t.Fatal("cancelled request sent")
	}
	if s.cloudAuthorized(context.Background(), s.accountEpoch+1, "token", func(string) error { called = true; return nil }) == nil || called {
		t.Fatal("stale account request sent")
	}
}

func TestCloudPullOrderMetadataAndUnlockedRequest(t *testing.T) {
	s := cloudTestService(t)
	const a = "abcdef0123456789abcdef01"
	const b = "abcdef0123456789abcdef02"
	s.state["queue"] = []any{object{"eid": a, "title": "existing"}, object{"eid": "local"}, object{"eid": a}}
	s.state["history"] = []any{object{"eid": b, "title": "history"}}
	s.state["episodeUpdates"] = object{"episodes": []any{object{"eid": b}, object{"eid": "other"}}}
	s.client.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if !s.mu.TryLock() {
			t.Error("playlist network held mutex")
		} else {
			s.mu.Unlock()
		}
		if r.URL.Path != "/v1/playlist/pull" {
			t.Fatalf("metadata was not reused: %s", r.URL.Path)
		}
		return reply(200, `{"data":{"kind":"FULL_LIST","list":["`+b+`","`+a+`","`+b+`"]}}`), nil
	})}
	r := call(t, s, "pullCloudPlaylist", object{"currentEid": a})
	q := list(r["queue"])
	if !flag(r["ok"]) || len(q) != 3 || str(obj(q[0])["title"]) != "history" || str(obj(q[1])["title"]) != "existing" || num(r["index"]) != 1 {
		t.Fatal(r)
	}
	if len(list(obj(s.state["episodeUpdates"])["episodes"])) != 2 {
		t.Fatal("preview mutated updates")
	}
	call(t, s, "saveQueue", object{"queue": q, "index": r["index"]})
	if len(list(obj(s.state["episodeUpdates"])["episodes"])) != 1 {
		t.Fatal("committed updates retained")
	}
}

func TestCloudResumeDuringUpload(t *testing.T) {
	s := cloudTestService(t)
	const id = "abcdef0123456789abcdef01"
	const pid = "abcdef0123456789abcdef02"
	entered, release := make(chan struct{}), make(chan struct{})
	calls := 0
	s.client.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/v1/playback-progress/update" {
			t.Error("unexpected history after resume")
		}
		close(entered)
		<-release
		if r.Context().Err() == nil {
			t.Error("HTTP context not canceled")
		}
		return reply(200, "{}"), nil
	})}
	for _, kind := range []string{"playing", "paused"} {
		call(t, s, "playbackEvent", object{"event": kind, "episode": object{"eid": id, "pid": pid}, "position": 12.0})
	}
	task := clone(obj(obj(s.cloudStore()["tasks"])["a:"+id]))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.cloudCancel = cancel
	s.cloudActive = id
	done := make(chan struct{})
	go func() { s.runCloudTask(ctx, "token", task, s.accountEpoch); close(done) }()
	<-entered
	event(t, s, "playing", id, 12)
	close(release)
	<-done
	live := obj(obj(s.cloudStore()["tasks"])["a:"+id])
	if calls != 1 || !flag(live["progressDone"]) || flag(live["historyDone"]) || str(live["error"]) != "" || flag(live["inFlight"]) {
		t.Fatal(live)
	}
	if v := s.cloudSyncView(); num(v["waiting"]) != 0 || str(v["nextUploadAt"]) != "" {
		t.Fatal(v)
	}
	event(t, s, "paused", id, 12)
	live = obj(obj(s.cloudStore()["tasks"])["a:"+id])
	if !flag(live["progressDone"]) || flag(live["suspended"]) {
		t.Fatal(live)
	}
}

func TestCloudChangedPositionFailureLatchAndCooldownRestart(t *testing.T) {
	s := cloudTestService(t)
	const id = "abcdef0123456789abcdef01"
	const pid = "abcdef0123456789abcdef02"
	calls := 0
	s.client.HTTPClient = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { calls++; return reply(500, "{}"), nil })}
	for _, kind := range []string{"playing", "ended"} {
		call(t, s, "playbackEvent", object{"event": kind, "episode": object{"eid": id, "pid": pid}, "position": 12.0})
	}
	task := clone(obj(obj(s.cloudStore()["tasks"])["a:"+id]))
	s.runCloudTask(context.Background(), "token", task, s.accountEpoch)
	for _, kind := range []string{"playing", "ended"} {
		call(t, s, "playbackEvent", object{"event": kind, "episode": object{"eid": id, "pid": pid}, "position": 13.0})
	}
	if v := s.cloudSyncView(); !flag(v["blocked"]) || num(v["failed"]) != 1 {
		t.Fatal(v)
	}
	s2, err := New(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	s2.client = s.client
	call(t, s2, "retryCloudSync", nil)
	task = clone(obj(obj(s2.cloudStore()["tasks"])["a:"+id]))
	s2.runCloudTask(context.Background(), "token", task, s2.accountEpoch)
	if calls != 1 {
		t.Fatal("retry bypassed persisted cooldown")
	}
	cloudAccount(s2.cloudStore(), "a")["nextAllowedAt"] = time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)
	s2.runCloudTask(context.Background(), "token", task, s2.accountEpoch)
	if calls != 2 || !flag(s2.cloudSyncView()["blocked"]) {
		t.Fatal("new failure did not latch")
	}
	cloudAccount(s2.cloudStore(), "a")["nextAllowedAt"] = ""
	s2.runCloudTask(context.Background(), "token", task, s2.accountEpoch)
	if calls != 2 {
		t.Fatal("latch bypassed")
	}
	s2.state["user"] = object{"uid": "b"}
	if v := s2.cloudSyncView(); flag(v["blocked"]) || str(v["lastError"]) != "" {
		t.Fatal(v)
	}
}

func TestCloudRecoveryDeadlineAndCompletedBound(t *testing.T) {
	s := cloudTestService(t)
	at := time.Now().Add(-4 * time.Minute).UTC().Format(time.RFC3339Nano)
	call(t, s, "playbackEvent", object{"event": "playing", "episode": object{"eid": "one", "pid": "pod"}, "position": 12.0, "playedAt": at})
	s2, err := New(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s2.StartCloudSync(ctx)
	task := obj(obj(s2.cloudStore()["tasks"])["a:one"])
	recorded, _ := time.Parse(time.RFC3339Nano, at)
	due := str(task["dueAt"])
	if due != recorded.Add(5*time.Minute).Format(time.RFC3339Nano) {
		t.Fatal(task)
	}
	s2.StartCloudSync(ctx)
	if str(obj(obj(s2.cloudStore()["tasks"])["a:one"])["dueAt"]) != due {
		t.Fatal("paused deadline reset")
	}
	c := s2.cloudStore()
	completed := object{}
	for i := 0; i < 305; i++ {
		key := time.Unix(int64(i), 0).UTC().Format(time.RFC3339)
		completed[key] = object{"completedAt": key}
	}
	completed["a:same"] = object{"completedAt": "2099-01-01T00:00:00Z"}
	completed["b:same"] = object{"completedAt": "2099-01-01T00:00:01Z"}
	c["completed"] = completed
	trimCloudCompleted(c)
	if len(completed) != 300 || completed["a:same"] == nil || completed["b:same"] == nil || completed["1970-01-01T00:00:00Z"] != nil || len(obj(c["tasks"])) != 1 {
		t.Fatal("incorrect completion pruning")
	}
}

func TestCloudSuccessfulPartsShareTaskCooldown(t *testing.T) {
	s := cloudTestService(t)
	const pid = "abcdef0123456789abcdef02"
	calls := 0
	s.client.HTTPClient = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { calls++; return reply(200, "{}"), nil })}
	for _, id := range []string{"abcdef0123456789abcdef01", "abcdef0123456789abcdef03"} {
		for _, kind := range []string{"playing", "ended"} {
			call(t, s, "playbackEvent", object{"event": kind, "episode": object{"eid": id, "pid": pid}, "position": 12.0})
		}
	}
	tasks := obj(s.cloudStore()["tasks"])
	first := clone(obj(tasks["a:abcdef0123456789abcdef01"]))
	second := clone(obj(tasks["a:abcdef0123456789abcdef03"]))
	s.runCloudTask(context.Background(), "token", first, s.accountEpoch)
	s.runCloudTask(context.Background(), "token", second, s.accountEpoch)
	if calls != 2 || len(obj(s.cloudStore()["tasks"])) != 1 {
		t.Fatal("parts throttled or task burst", calls)
	}
	s2, err := New(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	s2.client = s.client
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s2.StartCloudSync(ctx); close(done) }()
	// Worker must wait on the persisted cooldown, rather than send on startup.
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done
	if calls != 2 {
		t.Fatal("restart burst", calls)
	}
	if due := cloudDue(second, cloudAccount(s2.cloudStore(), "a")); time.Until(due) < 4*time.Minute {
		t.Fatal("cooldown missing", due)
	}
}
