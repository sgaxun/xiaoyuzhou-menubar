package player

import (
	"os"
	"path/filepath"
	"testing"
)

// Explicit opt-in: copy local credentials to an isolated directory and perform
// read-only content requests. Never print tokens, user details, or subscriptions.
func TestLiveReadOnlyBackend(t *testing.T) {
	source := os.Getenv("XIAOYUZHOU_LIVE_DATA_DIR")
	if source == "" {
		t.Skip("opt-in live acceptance")
	}
	dir := t.TempDir()
	raw, err := os.ReadFile(filepath.Join(source, "credentials.json"))
	if err != nil {
		t.Fatal("cannot read local login")
	}
	if err = os.WriteFile(filepath.Join(dir, "credentials.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"state", "subscriptions?refresh=1", "discover?refresh=1", "search?q=科技", "episode?eid=6aed7a0e88191f4f6e874397", "shownotes?eid=6aed7a0e88191f4f6e874397", "audioUrl?eid=6aed7a0e88191f4f6e874397"} {
		r := call(t, s, action, nil)
		if !flag(r["ok"]) {
			t.Fatalf("%s: %s", action, str(r["error"]))
		}
		switch {
		case action == "subscriptions?refresh=1":
			if len(list(r["list"])) == 0 {
				t.Fatal("no subscriptions returned")
			}
			t.Logf("subscriptions: %d", len(list(r["list"])))
		case action == "discover?refresh=1":
			if len(list(r["picks"])) == 0 {
				t.Fatal("no editor picks")
			}
			t.Logf("editor picks groups: %d", len(list(r["picks"])))
		case action == "shownotes?eid=6aed7a0e88191f4f6e874397":
			if len(str(r["html"])) == 0 {
				t.Fatal("empty shownotes")
			}
		}
	}
}
