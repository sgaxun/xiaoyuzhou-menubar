//go:build !windows

package main

import (
	"context"
	"github.com/egoist/mygo"
	"xiaoyuzhou-menubar/internal/appupdate"
)

func platformUpdatesEnabled() bool { return mygo.Updater.Enabled() }
func platformCheckUpdate(ctx context.Context) (appupdate.Candidate, error) {
	return nativeCheckUpdate(ctx)
}
func handlePrivilegedUpdate() {}
