package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"github.com/egoist/mygo"
	"xiaoyuzhou-menubar/internal/appupdate"
)

// Build configuration is public metadata only; update signing secrets never
// enter the application or the source repository.
//
//go:embed mygo.json
var buildConfig []byte

func currentVersion() string {
	var c struct {
		Version string `json:"version"`
	}
	_ = json.Unmarshal(buildConfig, &c)
	return c.Version
}

type updateDriver struct{}

func (updateDriver) Enabled() bool { return mygo.Updater.Enabled() }
func (updateDriver) Check(ctx context.Context) (appupdate.Candidate, error) {
	up, err := mygo.Updater.Check(ctx)
	if up == nil || err != nil {
		return nil, err
	}
	return nativeCandidate{up}, nil
}
func (updateDriver) Restart() { mygo.App.Relaunch() }

type nativeCandidate struct{ up *mygo.Update }

func (c nativeCandidate) Version() string { return c.up.Version }
func (c nativeCandidate) Notes() string   { return c.up.Notes }
func (c nativeCandidate) Install(ctx context.Context, progress func(int64, int64)) error {
	return c.up.Install(ctx, progress)
}
func (b *Backend) UpdateStatus() appupdate.Status { return b.updates.Status() }
func (b *Backend) CheckUpdates() appupdate.Status { return b.updates.Check() }
func (b *Backend) SetAutomaticUpdates(enabled bool) (appupdate.Status, error) {
	return b.updates.SetAutomatic(enabled)
}
func (b *Backend) DownloadUpdate() (appupdate.Status, error) { return b.updates.Download() }
func (b *Backend) RestartForUpdate() error                   { return b.updates.Restart() }
