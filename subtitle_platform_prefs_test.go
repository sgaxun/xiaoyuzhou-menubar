package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"xiaoyuzhou-menubar/internal/player"
)

func TestSubtitlePreferencePersistenceAndConcurrentPatches(t *testing.T) {
	dir := t.TempDir()
	service, err := player.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	b := &Backend{service: service}
	value, err := b.SubtitlePreference()
	if err != nil || value["enabled"] != nil {
		t.Fatal("legacy preference must be absent", value, err)
	}
	// Both partial patches use the same service lock, so neither drops the other.
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); failures <- b.SaveSubtitlePreference(true) }()
	go func() {
		defer wg.Done()
		_, err := service.Request("savePrefs", map[string]any{"rate": 1.5, "volume": 0.3})
		failures <- err
	}()
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	reopened, err := player.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	b.service = reopened
	value, err = b.SubtitlePreference()
	if err != nil || value["enabled"] != true {
		t.Fatal("enabled not restored", value, err)
	}
	raw, err := reopened.Request("state", nil)
	if err != nil {
		t.Fatal(err)
	}
	state := raw.(map[string]any)["state"].(map[string]any)
	if state["rate"] != 1.5 || state["volume"] != 0.3 {
		t.Fatal("subtitle write lost player prefs", state)
	}
	if err := b.SaveSubtitlePreference(false); err != nil {
		t.Fatal(err)
	}
	// Unsupported types must not turn the switch on, or remove its false value.
	_, err = reopened.Request("savePrefs", map[string]any{"menubarTranscript": "true", "panelWidth": 640})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err = player.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	b.service = reopened
	value, err = b.SubtitlePreference()
	if err != nil || value["enabled"] != false {
		t.Fatal("disabled not restored", value, err)
	}
}

func TestSubtitlePreferenceReportsFailedSave(t *testing.T) {
	dir := t.TempDir()
	service, err := player.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	// An existing directory makes atomic replacement of state.json fail.
	if err = os.Mkdir(filepath.Join(dir, "state.json"), 0700); err != nil {
		t.Fatal(err)
	}
	b := &Backend{service: service}
	if err = b.SaveSubtitlePreference(true); err == nil {
		t.Fatal("failed write reported success")
	}
	value, err := b.SubtitlePreference()
	if err != nil || value["enabled"] != nil {
		t.Fatal("failed save committed in memory", value, err)
	}
}
