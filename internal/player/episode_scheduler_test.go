package player

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func schedulerStamp(s *Service) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return str(obj(s.state["episodeUpdates"])["lastAutomaticCheck"])
}

func schedulerSetStamp(t *testing.T, s *Service, at time.Time) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.state)
	feed := obj(next["episodeUpdates"])
	feed["lastAutomaticCheck"] = at.UTC().Format(time.RFC3339Nano)
	next["episodeUpdates"] = feed
	if err := s.saveState(next); err != nil {
		t.Fatal(err)
	}
}

func TestEpisodeSchedulerSkips(t *testing.T) {
	for _, scenario := range []string{"disabled", "no cache", "empty cache", "not due"} {
		t.Run(scenario, func(t *testing.T) {
			eps := []any{updateEpisode("new", time.Now())}
			fail, calls := false, 0
			s := updateTestService(t, &eps, &fail, &calls)
			now := time.Now().UTC()
			switch scenario {
			case "disabled":
				call(t, s, "setEpisodeUpdateAutomatic", object{"enabled": false})
			case "no cache":
				delete(s.cache, "subs")
			case "empty cache":
				s.cache["subs"] = object{"value": []any{}}
			case "not due":
				schedulerSetStamp(t, s, now.Add(-59*time.Minute))
			}
			before := schedulerStamp(s)
			if err := s.runAutomaticEpisodeUpdates(context.Background(), now); err != nil {
				t.Fatal(err)
			}
			if calls != 0 {
				t.Fatalf("skip made %d network requests", calls)
			}
			if got := schedulerStamp(s); got != before {
				t.Fatalf("skip changed automatic timestamp: %q -> %q", before, got)
			}
			feed := call(t, s, "episodeUpdates", nil)
			if checking, ok := feed["checking"].(bool); !ok || checking {
				t.Fatalf("idle checking flag: %#v", feed["checking"])
			}
		})
	}
}

func TestEpisodeSchedulerDuePersistsAndThrottles(t *testing.T) {
	for _, failing := range []bool{false, true} {
		name := "success"
		if failing {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			now := time.Now().UTC()
			eps := []any{updateEpisode("scheduled", now.Add(-time.Minute))}
			fail, calls := failing, 0
			s := updateTestService(t, &eps, &fail, &calls)
			if automatic, ok := call(t, s, "episodeUpdates", nil)["automatic"].(bool); !ok || !automatic {
				t.Fatal("automatic updates must default to enabled")
			}
			schedulerSetStamp(t, s, now.Add(-time.Hour-time.Second))
			if err := s.runAutomaticEpisodeUpdates(context.Background(), now); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("due pass made %d requests, want one", calls)
			}
			stamp := schedulerStamp(s)
			if at := updateDate(stamp); at.IsZero() || at.Before(now.Add(-time.Second)) || at.After(time.Now().Add(time.Second)) {
				t.Fatalf("attempt timestamp not saved: %q", stamp)
			}
			reopened, err := NewWithClient(s.dir, s.client)
			if err != nil {
				t.Fatal(err)
			}
			if schedulerStamp(reopened) != stamp {
				t.Fatal("automatic attempt not persisted across restart")
			}
			feed := call(t, reopened, "episodeUpdates", nil)
			if failing {
				if len(list(feed["failures"])) != 1 {
					t.Fatalf("missing failure: %#v", feed)
				}
			} else if episodes := list(feed["episodes"]); len(episodes) != 1 || str(obj(episodes[0])["eid"]) != "scheduled" {
				t.Fatalf("missing persisted episode: %#v", feed)
			}
			// Remove the per-program cooldown to prove the hourly scheduler gate itself survives restart.
			expireUpdate(t, reopened)
			if err := reopened.runAutomaticEpisodeUpdates(context.Background(), now.Add(30*time.Minute)); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || schedulerStamp(reopened) != stamp {
				t.Fatal("automatic attempt retried within the hour")
			}
			call(t, reopened, "checkEpisodeUpdates", nil)
			if calls != 2 {
				t.Fatal("manual check was blocked by automatic cooldown")
			}
			if schedulerStamp(reopened) != stamp {
				t.Fatal("manual check changed automatic timestamp")
			}
		})
	}
}

func TestEpisodeSchedulerTogglePersists(t *testing.T) {
	eps := []any{}
	fail, calls := false, 0
	s := updateTestService(t, &eps, &fail, &calls)
	for _, enabled := range []bool{false, true} {
		feed := call(t, s, "setEpisodeUpdateAutomatic", object{"enabled": enabled})
		if got, ok := feed["automatic"].(bool); !ok || got != enabled {
			t.Fatalf("toggle response automatic = %#v, want %v", feed["automatic"], enabled)
		}
		reopened, err := NewWithClient(s.dir, s.client)
		if err != nil {
			t.Fatal(err)
		}
		s = reopened
		if got, ok := call(t, s, "episodeUpdates", nil)["automatic"].(bool); !ok || got != enabled {
			t.Fatalf("restarted automatic = %#v, want %v", got, enabled)
		}
	}
	if calls != 0 {
		t.Fatal("toggling fetched episodes")
	}
}

func TestEpisodeSchedulerDisableCancelsWithoutOverlap(t *testing.T) {
	eps := []any{}
	fail, calls := false, 0
	s := updateTestService(t, &eps, &fail, &calls)
	started := make(chan struct{})
	cancelled := make(chan struct{})
	var requests atomic.Int32
	s.client.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if requests.Add(1) == 1 {
			close(started)
		}
		<-r.Context().Done()
		if requests.Load() == 1 {
			close(cancelled)
		}
		return nil, r.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.runAutomaticEpisodeUpdates(ctx, time.Now()) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("automatic request did not start")
	}
	feed := call(t, s, "episodeUpdates", nil)
	if checking, ok := feed["checking"].(bool); !ok || !checking {
		t.Fatalf("active checking flag: %#v", feed["checking"])
	}
	overlap := make(chan struct{})
	go func() {
		s.runAutomaticEpisodeUpdates(ctx, time.Now().Add(2*time.Hour))
		s.Request("checkEpisodeUpdates", nil)
		close(overlap)
	}()
	select {
	case <-overlap:
	case <-time.After(time.Second):
		t.Fatal("overlapping checks blocked behind network request")
	}
	if requests.Load() != 1 {
		t.Fatal("overlapping check issued another request")
	}
	toggled := make(chan object, 1)
	go func() { v, _ := s.Request("setEpisodeUpdateAutomatic", object{"enabled": false}); toggled <- obj(v) }()
	select {
	case feed = <-toggled:
		if automatic, ok := feed["automatic"].(bool); !ok || automatic {
			t.Fatal("disable did not report automatic=false")
		}
	case <-time.After(time.Second):
		t.Fatal("disable blocked behind network request")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("disable did not cancel active request")
	}
	select {
	case <-done: // Cancellation may be returned or consumed by the check loop.
	case <-time.After(time.Second):
		t.Fatal("cancelled automatic pass did not finish")
	}
	if checking, ok := call(t, s, "episodeUpdates", nil)["checking"].(bool); !ok || checking {
		t.Fatal("checking stayed true after cancellation")
	}
}
