package player

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"xiaoyuzhou-menubar/internal/podcast"
)

const updatePID = "0123456789abcdef01234567"

func updateTestService(t *testing.T, episodes *[]any, fail *bool, calls *int) *Service {
	t.Helper()
	c := podcast.NewClient()
	c.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		*calls++
		if r.URL.Path != "/podcast/"+updatePID || r.Header.Get("x-jike-access-token") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected authenticated/non-public request: %s", r.URL.Path)
		}
		if *fail {
			return reply(503, "unavailable"), nil
		}
		data, _ := json.Marshal(object{"props": object{"pageProps": object{"podcast": object{"pid": updatePID, "title": "Program", "episodes": *episodes}}}})
		return reply(200, fmt.Sprintf(`<script id="__NEXT_DATA__" type="application/json">%s</script>`, data)), nil
	})}
	s, err := NewWithClient(t.TempDir(), c)
	if err != nil {
		t.Fatal(err)
	}
	// A malformed credential file makes any accidental token lookup fail.
	if err := writeJSON(s.dir+"/credentials.json", []any{"not credentials"}); err != nil {
		t.Fatal(err)
	}
	s.cache["subs"] = object{"value": []any{object{"pid": updatePID, "title": "Program", "latestEpisodePubDate": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}}}
	if err := s.saveCache(); err != nil {
		t.Fatal(err)
	}
	return s
}
func updateEpisode(id string, at time.Time) any {
	return object{"eid": id, "title": id, "pubDate": at.UTC().Format(time.RFC3339)}
}
func expireUpdate(t *testing.T, s *Service) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.state)
	obj(obj(obj(next["episodeUpdates"])["programs"])[updatePID])["attemptedAt"] = ""
	if err := s.saveState(next); err != nil {
		t.Fatal(err)
	}
}
func TestEpisodeUpdatesPersistenceAndWatermark(t *testing.T) {
	at := time.Now().Add(-30 * time.Minute)
	eps := []any{updateEpisode("a", at), updateEpisode("b", at)}
	fail := false
	calls := 0
	s := updateTestService(t, &eps, &fail, &calls)
	s.cache["podcast:"+updatePID] = object{"value": object{"episodes": []any{updateEpisode("cached", at.Add(-time.Minute))}}}
	r := call(t, s, "checkEpisodeUpdates", nil)
	if len(list(r["episodes"])) != 2 || calls != 1 || str(r["checkedAt"]) == "" {
		t.Fatal(r, calls)
	}
	// Cooldown survives restart and reading the feed never requests anything.
	reopened, err := NewWithClient(s.dir, s.client)
	if err != nil {
		t.Fatal(err)
	}
	s = reopened
	call(t, s, "episodeUpdates", nil)
	call(t, s, "checkEpisodeUpdates", nil)
	if calls != 1 {
		t.Fatal("cooldown not persisted")
	}
	call(t, s, "dismissEpisodeUpdates", object{"eids": []any{"a"}})
	expireUpdate(t, s)
	eps = append(eps, updateEpisode("same-date", at), updateEpisode("old", at.Add(-time.Hour)))
	r = call(t, s, "checkEpisodeUpdates", nil)
	ids := []string{}
	for _, v := range list(r["episodes"]) {
		ids = append(ids, str(obj(v)["eid"]))
	}
	if strings.Join(ids, ",") != "b,same-date" {
		t.Fatal(ids)
	}
	fail = true
	expireUpdate(t, s)
	r = call(t, s, "checkEpisodeUpdates", nil)
	if len(list(r["failures"])) != 1 || len(list(r["episodes"])) != 2 || calls != 3 {
		t.Fatal(r, calls)
	}
	r = call(t, s, "checkEpisodeUpdates", nil)
	if len(list(r["failures"])) != 1 {
		t.Fatal("cooldown cleared failure", r)
	}
	if calls != 3 {
		t.Fatal("failure retried")
	}
	call(t, s, "clearCache", nil)
	r = call(t, s, "episodeUpdates", nil)
	if len(list(r["episodes"])) != 0 || len(list(r["failures"])) != 0 {
		t.Fatal("clear leaked feed", r)
	}
}
func TestEpisodeUpdatesRecentFallbackAndAccountInvalidation(t *testing.T) {
	eps := []any{updateEpisode("recent", time.Now().Add(-time.Hour)), updateEpisode("ancient", time.Now().Add(-8*24*time.Hour))}
	fail := false
	calls := 0
	s := updateTestService(t, &eps, &fail, &calls)
	obj(list(obj(s.cache["subs"])["value"])[0])["latestEpisodePubDate"] = time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339)
	r := call(t, s, "checkEpisodeUpdates", nil)
	if len(list(r["episodes"])) != 1 {
		t.Fatal(r)
	}
	s.mu.Lock()
	err := s.clearAccountCache()
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	r = call(t, s, "episodeUpdates", nil)
	if len(list(r["episodes"])) != 0 || num(r["remaining"]) != 0 {
		t.Fatal(r)
	}
	// With no cached baseline at all, only the recent seven days are eligible.
	s.cache["subs"] = object{"value": []any{object{"pid": updatePID}}}
	r = call(t, s, "checkEpisodeUpdates", nil)
	if len(list(r["episodes"])) != 1 || str(obj(list(r["episodes"])[0])["eid"]) != "recent" {
		t.Fatal("missing baseline imported old episodes", r)
	}
}

func TestEpisodeUpdatesNetworkUnlockAndInvalidation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan object, 1)
	c := podcast.NewClient()
	c.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-release
		return reply(200, `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"podcast":{"pid":"0123456789abcdef01234567","episodes":[]}}}}</script>`), nil
	})}
	s, err := NewWithClient(t.TempDir(), c)
	if err != nil {
		t.Fatal(err)
	}
	s.cache["subs"] = object{"value": []any{object{"pid": updatePID}}}
	go func() { v, _ := s.Request("checkEpisodeUpdates", nil); done <- obj(v) }()
	<-started
	cleared := make(chan struct{})
	go func() { s.Request("clearCache", nil); close(cleared) }()
	select {
	case <-cleared:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("network held service lock")
	}
	close(release)
	r := <-done
	if str(r["checkedAt"]) != "" || len(obj(s.state["episodeUpdates"])) != 0 {
		t.Fatal("in-flight result crossed cache/account reset", r)
	}
}
func TestEpisodeUpdatesSpacingAndSort(t *testing.T) {
	starts := []time.Time{}
	c := podcast.NewClient()
	c.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		starts = append(starts, time.Now())
		data, _ := json.Marshal(object{"props": object{"pageProps": object{"podcast": object{"pid": updatePID, "episodes": []any{updateEpisode(r.URL.Path+"older", time.Now().Add(-2*time.Hour)), updateEpisode(r.URL.Path+"newer", time.Now().Add(-time.Hour))}}}}})
		return reply(200, fmt.Sprintf(`<script id="__NEXT_DATA__" type="application/json">%s</script>`, data)), nil
	})}
	s, err := NewWithClient(t.TempDir(), c)
	if err != nil {
		t.Fatal(err)
	}
	s.cache["subs"] = object{"value": []any{object{"pid": updatePID}, object{"pid": "1123456789abcdef01234567"}}}
	v, err := s.checkEpisodeUpdates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(starts) != 2 || starts[1].Sub(starts[0]) < time.Second {
		t.Fatal("requests not spaced", starts)
	}
	eps := list(obj(v)["episodes"])
	if len(eps) != 4 {
		t.Fatal(v)
	}
	for i := 1; i < len(eps); i++ {
		if updateDate(obj(eps[i])["pubDate"]).After(updateDate(obj(eps[i-1])["pubDate"])) {
			t.Fatal("not descending", eps)
		}
	}
}

func TestEpisodeUpdatesExcludeQueueAndDoNotResurface(t *testing.T) {
	at := time.Now().Add(-time.Minute)
	eps := []any{updateEpisode("queued", at), updateEpisode("pending", at)}
	fail, calls := false, 0
	s := updateTestService(t, &eps, &fail, &calls)
	call(t, s, "saveQueue", object{"queue": []any{object{"eid": "queued"}}, "index": 0})
	feed := call(t, s, "checkEpisodeUpdates", nil)
	if len(list(feed["episodes"])) != 1 || str(obj(list(feed["episodes"])[0])["eid"]) != "pending" {
		t.Fatal(feed)
	}
	call(t, s, "saveQueue", object{"queue": eps, "index": 0})
	if len(list(call(t, s, "episodeUpdates", nil)["episodes"])) != 0 {
		t.Fatal("queued pending episode remained")
	}
	call(t, s, "saveQueue", object{"queue": []any{}, "index": -1})
	expireUpdate(t, s)
	feed = call(t, s, "checkEpisodeUpdates", nil)
	if len(list(feed["episodes"])) != 0 {
		t.Fatal("already queued episode resurfaced", feed)
	}
}

func TestEpisodeUpdatesSubscribedSourceAndLegacyCache(t *testing.T) {
	at := time.Now().Add(-time.Minute)
	otherPID := "1123456789abcdef01234567"
	eps := []any{object{"eid": "collaboration", "title": "Shared episode", "pubDate": at.UTC().Format(time.RFC3339), "podcast": object{"pid": otherPID, "title": "Partner", "image": object{"picUrl": "partner.png"}}}}
	fail, calls := false, 0
	s := updateTestService(t, &eps, &fail, &calls)
	sub := obj(list(obj(s.cache["subs"])["value"])[0])
	sub["image"] = "subscribed.png"
	feed := call(t, s, "checkEpisodeUpdates", nil)
	ep := obj(list(feed["episodes"])[0])
	if str(ep["subscriptionTitle"]) != "Program" || str(ep["subscriptionImage"]) != "subscribed.png" || !flag(ep["isCoCreated"]) {
		t.Fatal(ep)
	}
	if str(ep["pid"]) != otherPID || str(ep["podcastTitle"]) != "Partner" || str(ep["image"]) != "partner.png" {
		t.Fatal("canonical metadata changed", ep)
	}
	// Older pending episodes must display the correct source without networking
	// or changing the user's stored records/seen markers.
	legacy := object{"eid": "legacy", "pid": updatePID, "podcastTitle": "Partner", "image": "partner.png"}
	f := obj(s.state["episodeUpdates"])
	f["episodes"] = []any{legacy}
	feed = call(t, s, "episodeUpdates", nil)
	ep = obj(list(feed["episodes"])[0])
	if str(ep["subscriptionTitle"]) != "Program" || !flag(ep["isCoCreated"]) || calls != 1 {
		t.Fatal(ep, calls)
	}
	if _, changed := legacy["subscriptionTitle"]; changed {
		t.Fatal("read mutated stored episode")
	}
	normal := subscriptionEpisode(object{"pid": updatePID, "podcastTitle": "Program"}, sub)
	if flag(normal["isCoCreated"]) {
		t.Fatal("ordinary episode marked as collaboration")
	}
}

func TestQueueSubscriptionSourceSurvivesSaveAndRestart(t *testing.T) {
	at := time.Now().Add(-time.Minute)
	eps := []any{object{"eid": "shared-episode", "title": "Shared", "pubDate": at.UTC().Format(time.RFC3339), "podcast": object{"pid": "1123456789abcdef01234567", "title": "Partner", "image": object{"picUrl": "partner.png"}}}}
	fail, calls := false, 0
	s := updateTestService(t, &eps, &fail, &calls)
	sub := obj(list(obj(s.cache["subs"])["value"])[0])
	sub["image"] = "subscribed.png"
	if err := s.saveCache(); err != nil {
		t.Fatal(err)
	}
	call(t, s, "checkEpisodeUpdates", nil)
	if err := s.writeTokens("local-test-token", ""); err != nil {
		t.Fatal(err)
	}
	// A legacy saved queue retained the canonical title/image but lost its source.
	legacy := object{"eid": "shared-episode", "pid": "1123456789abcdef01234567", "title": "Shared", "podcastTitle": "Partner", "image": "partner.png"}
	call(t, s, "saveQueue", object{"queue": []any{legacy}, "index": 0})
	view := obj(call(t, s, "state", nil)["state"])
	restored := obj(list(view["queue"])[0])
	if str(restored["subscriptionTitle"]) != "Program" || str(restored["subscriptionImage"]) != "subscribed.png" || !flag(restored["isCoCreated"]) {
		t.Fatal(restored)
	}
	if str(restored["podcastTitle"]) != "Partner" || str(restored["image"]) != "partner.png" {
		t.Fatal("canonical metadata overwritten", restored)
	}
	call(t, s, "saveQueue", object{"queue": view["queue"], "index": 0})
	reopened, err := NewWithClient(s.dir, s.client)
	if err != nil {
		t.Fatal(err)
	}
	again := obj(list(obj(call(t, reopened, "state", nil)["state"])["queue"])[0])
	if str(again["subscriptionPid"]) != updatePID || str(again["subscriptionTitle"]) != "Program" || calls != 1 {
		t.Fatal(again, calls)
	}
	// Multiple possible local sources must not invent an attribution.
	s.cache["subs"] = object{"value": []any{sub, object{"pid": "2123456789abcdef01234567", "title": "Other"}}}
	obj(obj(s.state["episodeUpdates"])["programs"])["2123456789abcdef01234567"] = object{"seen": object{"shared-episode": true}}
	if str(s.queueSubscriptionSource(legacy)["subscriptionTitle"]) != "" {
		t.Fatal("ambiguous source was guessed")
	}
}
