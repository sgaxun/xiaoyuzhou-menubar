package player

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xiaoyuzhou-menubar/internal/qrauth"
)

func TestCorruptCacheRecoversWithoutDiscardingState(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cache.json"), []byte(`{"truncated":`), 0600); err != nil {
		t.Fatal(err)
	}
	state := defaultState()
	state["rate"] = 2
	if err := writeJSON(filepath.Join(dir, "state.json"), state); err != nil {
		t.Fatal(err)
	}
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if num(s.state["rate"]) != 2 || len(s.cache) != 0 {
		t.Fatal("state/cache recovery failed")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "cache.invalid-*.json"))
	if len(files) != 1 {
		t.Fatal("corrupt cache not preserved")
	}
}
func TestServiceQRCancelInterruptsPolling(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	s.qr = qrauth.New(qrauth.Options{HTTPClient: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/create") {
			return reply(200, `{"id":"test-remote"}`), nil
		}
		close(started)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}, Persist: func(qrauth.Tokens) (map[string]any, error) { t.Error("cancelled session persisted"); return nil, nil }})
	v, err := s.Request("qrStart", nil)
	if err != nil {
		t.Fatal(err)
	}
	id := str(v.(qrauth.Result)["sessionId"])
	done := make(chan struct{})
	go func() { defer close(done); _, _ = s.Request("qrStatus", object{"sessionId": id}) }()
	<-started
	cancelDone := make(chan struct{})
	go func() { defer close(cancelDone); _, _ = s.Request("qrCancel", object{"sessionId": id}) }()
	select {
	case <-cancelDone:
	case <-time.After(time.Second):
		t.Fatal("QR cancellation blocked by service state lock")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancel did not abort polling")
	}
}
