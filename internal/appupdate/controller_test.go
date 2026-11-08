package appupdate

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type fakeCandidate struct {
	failure  bool
	installs atomic.Int32
}

func (*fakeCandidate) Version() string { return "0.4.0" }
func (*fakeCandidate) Notes() string   { return "新版说明" }
func (c *fakeCandidate) Install(ctx context.Context, p func(int64, int64)) error {
	c.installs.Add(1)
	p(4, 10)
	p(10, 10)
	if c.failure {
		return errors.New("bad signature")
	}
	return nil
}

type fakeDriver struct {
	enabled  bool
	up       Candidate
	err      error
	checks   atomic.Int32
	restarts atomic.Int32
	gate     chan struct{}
}

func (d *fakeDriver) Enabled() bool { return d.enabled }
func (d *fakeDriver) Check(ctx context.Context) (Candidate, error) {
	d.checks.Add(1)
	if d.gate != nil {
		select {
		case <-d.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return d.up, d.err
}
func (d *fakeDriver) Restart() { d.restarts.Add(1) }
func waitPhase(t *testing.T, c *Controller, phase string) Status {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		status := c.Status()
		if status.Phase == phase {
			return status
		}
		select {
		case <-deadline:
			t.Fatalf("phase=%s, want=%s", status.Phase, phase)
		case <-time.After(time.Millisecond):
		}
	}
}
func TestCheckNeverDownloadsOrRestarts(t *testing.T) {
	up := &fakeCandidate{}
	driver := &fakeDriver{enabled: true, up: up}
	c := New("0.3.0", driver, "")
	defer c.Close()
	c.Check()
	status := waitPhase(t, c, "available")
	if status.AvailableVersion != "0.4.0" || status.Notes != "新版说明" || up.installs.Load() != 0 || driver.restarts.Load() != 0 {
		t.Fatal(status)
	}
	if c.Restart() == nil {
		t.Fatal("restart without downloaded update")
	}
}
func TestExplicitDownloadThenExplicitRestart(t *testing.T) {
	up := &fakeCandidate{}
	driver := &fakeDriver{enabled: true, up: up}
	c := New("0.3.0", driver, "")
	defer c.Close()
	c.Check()
	waitPhase(t, c, "available")
	if _, err := c.Download(); err != nil {
		t.Fatal(err)
	}
	status := waitPhase(t, c, "ready")
	if driver.restarts.Load() != 0 || status.Downloaded != 10 {
		t.Fatal("automatic restart or missing progress")
	}
	if err := c.Restart(); err != nil {
		t.Fatal(err)
	}
	_ = c.Restart()
	if driver.restarts.Load() != 1 {
		t.Fatal("duplicate restart")
	}
}
func TestFailedSignatureNeverAllowsRestart(t *testing.T) {
	up := &fakeCandidate{failure: true}
	driver := &fakeDriver{enabled: true, up: up}
	c := New("0.3.0", driver, "")
	defer c.Close()
	c.Check()
	waitPhase(t, c, "available")
	_, _ = c.Download()
	waitPhase(t, c, "error")
	if c.Restart() == nil || driver.restarts.Load() != 0 {
		t.Fatal("failed update permitted restart")
	}
	if _, err := c.Download(); err != nil {
		t.Fatal("download retry unavailable")
	}
	waitPhase(t, c, "error")
}
func TestAutomaticPreferenceSurvivesRestart(t *testing.T) {
	file := filepath.Join(t.TempDir(), "updates.json")
	c := New("0.3.0", &fakeDriver{}, file)
	defer c.Close()
	if !c.Status().Automatic {
		t.Fatal("expected default automatic check")
	}
	if _, err := c.SetAutomatic(false); err != nil {
		t.Fatal(err)
	}
	next := New("0.3.0", &fakeDriver{}, file)
	defer next.Close()
	if next.Status().Automatic {
		t.Fatal("preference not persisted")
	}
}
func TestDuplicateChecksDisabledAndNetworkErrors(t *testing.T) {
	driver := &fakeDriver{enabled: true, gate: make(chan struct{})}
	c := New("0.3.0", driver, "")
	defer c.Close()
	for i := 0; i < 10; i++ {
		c.Check()
	}
	close(driver.gate)
	waitPhase(t, c, "current")
	if driver.checks.Load() != 1 {
		t.Fatal("overlapping checks")
	}
	disabled := New("0.3.0", &fakeDriver{}, "")
	defer disabled.Close()
	if disabled.Check().Phase != "disabled" {
		t.Fatal("disabled updater checked network")
	}
	offline := New("0.3.0", &fakeDriver{enabled: true, err: errors.New("offline")}, "")
	defer offline.Close()
	offline.Check()
	waitPhase(t, offline, "error")
}
