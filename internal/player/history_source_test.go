package player

import "testing"

func TestHistoryKeepsSubscribedSourceThroughProgressAndRestart(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	body := object{"eid": "shared-episode", "pid": "publisher", "title": "Shared episode", "podcastTitle": "Publisher", "image": "publisher.png", "duration": 1000.0, "position": 100.0, "subscriptionPid": "subscribed", "subscriptionTitle": "Subscribed program", "subscriptionImage": "subscribed.png", "isCoCreated": true}
	if r := call(t, s, "progress", body); !flag(r["ok"]) {
		t.Fatal(r)
	}
	// Old callers may send a progress-only update; it must retain the source.
	if r := call(t, s, "progress", object{"eid": "shared-episode", "position": 120.0}); !flag(r["ok"]) {
		t.Fatal(r)
	}
	restarted, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	state := obj(call(t, restarted, "state", nil)["state"])
	ep := obj(list(state["history"])[0])
	if str(ep["subscriptionPid"]) != "subscribed" || str(ep["subscriptionTitle"]) != "Subscribed program" || str(ep["subscriptionImage"]) != "subscribed.png" || !flag(ep["isCoCreated"]) {
		t.Fatal("source lost across progress and restart", ep)
	}
	if str(ep["pid"]) != "publisher" || str(ep["podcastTitle"]) != "Publisher" || num(ep["position"]) != 120 {
		t.Fatal("canonical data or progress changed", ep)
	}
}

func TestLegacyHistorySourceRecovery(t *testing.T) {
	for _, tc := range []struct {
		name                string
		ambiguous, explicit bool
	}{
		{name: "same episode queue"}, {name: "ambiguous queue", ambiguous: true}, {name: "explicit history wins", explicit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			s, err := New(dir)
			if err != nil {
				t.Fatal(err)
			}
			ep := object{"eid": "shared", "pid": "publisher", "podcastTitle": "Publisher", "image": "publisher.png"}
			if tc.explicit {
				ep["subscriptionPid"] = "history-source"
				ep["subscriptionTitle"] = "History source"
			}
			source := object{"eid": "shared", "subscriptionPid": "queue-source", "subscriptionTitle": "Queue source", "subscriptionImage": "queue.png", "isCoCreated": true}
			s.state["queue"] = []any{source, object{"eid": "unrelated", "subscriptionPid": "unrelated"}}
			if tc.ambiguous {
				s.state["queue"] = append(list(s.state["queue"]), object{"eid": "shared", "subscriptionPid": "other-source"})
			}
			s.state["history"] = []any{ep}
			got := obj(list(obj(call(t, s, "state", nil)["state"])["history"])[0])
			want := "queue-source"
			if tc.ambiguous {
				want = ""
			}
			if tc.explicit {
				want = "history-source"
			}
			if str(got["subscriptionPid"]) != want {
				t.Fatalf("source %q, want %q", got["subscriptionPid"], want)
			}
			if str(ep["subscriptionPid"]) == "queue-source" {
				t.Fatal("state response mutated stored history")
			}
			// Next progress save persists the repaired source for later launches.
			if r := call(t, s, "progress", object{"eid": "shared", "position": 15.0}); !flag(r["ok"]) {
				t.Fatal(r)
			}
			restarted, err := New(dir)
			if err != nil {
				t.Fatal(err)
			}
			saved := obj(list(restarted.state["history"])[0])
			if str(saved["subscriptionPid"]) != want {
				t.Fatal("repair was not persisted", saved)
			}
		})
	}
}
