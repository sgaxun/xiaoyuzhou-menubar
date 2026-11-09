package player

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Opt-in regression: verify content identity, not just a successful HTTP response.
func TestLiveKaifengTranscript(t *testing.T) {
	source := os.Getenv("XIAOYUZHOU_LIVE_DATA_DIR")
	if source == "" {
		t.Skip("opt-in live transcript acceptance")
	}
	raw, err := os.ReadFile(filepath.Join(source, "credentials.json"))
	if err != nil {
		t.Fatal("login unavailable")
	}
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "credentials.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	service, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	result := call(t, service, "transcript?eid=6af01e065b7c4709bfe2c966", nil)
	if !flag(result["ok"]) || result["status"] != "available" {
		t.Fatal("official transcript unavailable")
	}
	segments := list(result["segments"])
	var text strings.Builder
	last := float64(0)
	for _, v := range segments {
		segment := obj(v)
		text.WriteString(str(segment["text"]))
		last = num(segment["startMs"])
	}
	if len(segments) < 100 || last < 70*60*1000 || !strings.Contains(text.String(), "开封") || !strings.Contains(text.String(), "汴梁") {
		t.Fatalf("transcript does not match Kaifeng episode: segments=%d lastMs=%.0f", len(segments), last)
	}
	t.Logf("Kaifeng content verified: %d segments, last timestamp %.2f minutes", len(segments), last/60000)
}
