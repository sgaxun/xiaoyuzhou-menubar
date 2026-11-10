package player

import (
	"os"
	"path/filepath"
	"testing"
)

// Opt-in, read-only acceptance; account data is copied to an isolated store.
func TestLiveComments(t *testing.T) {
	source := os.Getenv("XIAOYUZHOU_COMMENTS_LIVE_DIR")
	if source == "" {
		t.Skip("opt-in comments acceptance")
	}
	dir := t.TempDir()
	raw, err := os.ReadFile(filepath.Join(source, "credentials.json"))
	if err != nil {
		t.Fatal("cannot read login")
	}
	if err = os.WriteFile(filepath.Join(dir, "credentials.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	eid := "6aeb10f797726cd4675cbe17"
	cursor := any(nil)
	seen := map[string]bool{}
	threads := map[string]bool{}
	for page := 0; page < 20; page++ {
		r := call(t, s, "comments", object{"eid": eid, "loadMoreKey": cursor})
		if !flag(r["ok"]) {
			t.Fatal(str(r["error"]))
		}
		for _, v := range list(r["data"]) {
			c := obj(v)
			seen[str(c["id"])] = true
			if num(c["threadReplyCount"]) > 0 || num(c["replyCount"]) > 0 {
				threads[str(c["id"])] = true
			}
		}
		cursor = r["loadMoreKey"]
		if cursor == nil {
			break
		}
		if page == 19 {
			t.Fatal("pagination did not finish")
		}
	}
	for id := range threads {
		cursor = nil
		for page := 0; page < 20; page++ {
			r := call(t, s, "comments", object{"eid": eid, "thread": id, "loadMoreKey": cursor})
			if !flag(r["ok"]) {
				t.Fatal(str(r["error"]))
			}
			for _, v := range list(r["data"]) {
				seen[str(obj(v)["id"])] = true
			}
			cursor = r["loadMoreKey"]
			if cursor == nil {
				break
			}
			if page == 19 {
				t.Fatal("reply pagination did not finish")
			}
		}
	}
	if len(seen) < 116 {
		t.Fatalf("incomplete: %d comments", len(seen))
	}
	t.Logf("read %d unique comments including replies", len(seen))
}
