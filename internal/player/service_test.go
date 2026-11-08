package player

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func call(t *testing.T, s *Service, action string, body object) object {
	t.Helper()
	v, err := s.Request(action, body)
	if err != nil {
		t.Fatal(err)
	}
	return obj(v)
}
func TestStateCompatibilityAndPersistence(t *testing.T) {
	dir := t.TempDir()
	legacy := defaultState()
	legacy["queue"] = []any{object{"eid": "legacy-episode", "title": "旧队列"}}
	legacy["rate"] = 2
	if err := writeJSON(filepath.Join(dir, "state.json"), legacy); err != nil {
		t.Fatal(err)
	}
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	state := call(t, s, "state", nil)
	if num(obj(state["state"])["rate"]) != 2 || len(list(obj(state["state"])["queue"])) != 1 {
		t.Fatal(state)
	}
	for _, position := range []float64{300, 20} {
		r := call(t, s, "progress", object{"eid": "test-episode", "position": position, "duration": 1000.0})
		if num(r["position"]) != 300 {
			t.Fatal(r)
		}
	}
	if r := call(t, s, "savePrefs", object{"rate": 1.5}); !flag(r["ok"]) {
		t.Fatal(r)
	}
	s2, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if num(s2.state["rate"]) != 1.5 {
		t.Fatal(s2.state)
	}
	info, _ := os.Stat(filepath.Join(dir, "state.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
}
func TestLogoutIsolationAndFailure(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.writeTokens("test-access", "test-refresh"); err != nil {
		t.Fatal(err)
	}
	s.cache["subs"] = object{"value": []any{object{"pid": "old-account"}}, "at": 1}
	if r := call(t, s, "subsCache", nil); !flag(r["hasCache"]) {
		t.Fatal(r)
	}
	// A failed atomic rename must not report successful logout.
	if err = os.Remove(filepath.Join(dir, "credentials.json")); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(dir, "credentials.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if r := call(t, s, "logout", nil); flag(r["ok"]) {
		t.Fatal(r)
	}
	if err = os.Remove(filepath.Join(dir, "credentials.json")); err != nil {
		t.Fatal(err)
	}
	if err = s.writeTokens("test-access", "test-refresh"); err != nil {
		t.Fatal(err)
	}
	if r := call(t, s, "logout", nil); !flag(r["ok"]) {
		t.Fatal(r)
	}
	r := call(t, s, "state", nil)
	if flag(r["loggedIn"]) {
		t.Fatal(r)
	}
	if r = call(t, s, "subsCache", nil); flag(r["hasCache"]) {
		t.Fatal(r)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if string(raw) != "null" {
		t.Fatal("credentials not deleted")
	}
}
func TestRequestBoundaries(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"debugLog", "invalid", "../state"} {
		r, err := s.Request(action, nil)
		if err == nil && flag(obj(r)["ok"]) {
			t.Fatalf("unexpected success: %s", action)
		}
	}
	if r := call(t, s, "subscriptions", nil); !flag(r["needsLogin"]) {
		t.Fatal(r)
	}
	for _, u := range []string{"http://127.0.0.1/", "https://localhost/", "https://127.0.0.1/"} {
		if validateMedia(u) == nil {
			t.Fatal(u)
		}
	}
	if validateMedia("https://media.xyzcdn.net/audio.m4a") != nil {
		t.Fatal("valid CDN rejected")
	}
	b, _ := json.Marshal(s.state)
	if !json.Valid(b) {
		t.Fatal("invalid state")
	}
}
