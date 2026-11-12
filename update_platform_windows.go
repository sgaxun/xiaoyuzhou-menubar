package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/netproxy"
	"golang.org/x/sys/windows"
	"xiaoyuzhou-menubar/internal/appupdate"
)

const privilegedUpdateFlag = "--xiaoyuzhou-privileged-update"

// Read-only discovery bypasses MyGo's write probe. Installation still goes
// through MyGo, in the installed executable elevated only by explicit request.
func platformUpdatesEnabled() bool {
	return !mygo.IsDev() && windowsUpdateFeed() != ""
}

func windowsUpdateFeed() string {
	var c struct {
		Updates struct{ GitHub, PublicKey, TagPrefix, URL string } `json:"updates"`
	}
	if json.Unmarshal(buildConfig, &c) != nil || c.Updates.GitHub == "" || c.Updates.PublicKey == "" || c.Updates.URL != "" {
		return ""
	}
	// This app publishes one stable release stream; fail closed if config
	// switches to tagged channels, which need different discovery semantics.
	if c.Updates.TagPrefix != "" && c.Updates.TagPrefix != "v" {
		return ""
	}
	return "https://github.com/" + c.Updates.GitHub + "/releases/latest/download/update-windows-" + runtime.GOARCH + ".json"
}

func platformCheckUpdate(ctx context.Context) (appupdate.Candidate, error) {
	if mygo.Updater.Enabled() {
		return nativeCheckUpdate(ctx)
	}
	if !platformUpdatesEnabled() {
		return nil, mygo.ErrUpdatesDisabled
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, windowsUpdateFeed(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "xiaoyuzhou-updater")
	client := &http.Client{Transport: netproxy.Transport(), CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || len(via) >= 10 {
			return errors.New("invalid update redirect")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update feed: %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 1<<20 {
		return nil, errors.New("update manifest too large")
	}
	var m struct {
		Version, Notes, URL, Signature string
		Size                           int64
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m.URL == "" || m.Signature == "" || m.Size <= 0 || m.Size > 1<<30 {
		return nil, errors.New("incomplete update manifest")
	}
	newer, err := newerStableVersion(m.Version, currentVersion())
	if err != nil || !newer {
		return nil, err
	}
	return elevatedCandidate{version: m.Version, notes: m.Notes}, nil
}

type elevatedCandidate struct{ version, notes string }

func (c elevatedCandidate) Version() string { return c.version }
func (c elevatedCandidate) Notes() string   { return c.notes }
func powershellLiteral(s string) string     { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func (c elevatedCandidate) Install(ctx context.Context, _ func(int64, int64)) error {
	if _, err := stableVersion(c.version); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// Use the system copy, never PATH, and avoid a writable result-file IPC.
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		return err
	}
	script := "$ErrorActionPreference='Stop'; try { $p=Start-Process -FilePath " + powershellLiteral(exe) +
		" -ArgumentList " + powershellLiteral(privilegedUpdateFlag+" "+c.version) +
		" -Verb RunAs -Wait -PassThru; exit $p.ExitCode } catch { exit 1 }"
	cmd := exec.CommandContext(ctx, filepath.Join(systemDir, "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("管理员更新未完成（可能已取消授权）: %w", err)
	}
	return nil
}

// Call first in main, before data access, singleton locks, or GUI startup.
// The helper never launches a GUI or runs a user-supplied executable/script.
func handlePrivilegedUpdate() {
	if len(os.Args) < 2 || os.Args[1] != privilegedUpdateFlag {
		return
	}
	if len(os.Args) != 3 {
		os.Exit(1)
	}
	newer, err := newerStableVersion(os.Args[2], currentVersion())
	if err != nil || !newer {
		os.Exit(1)
	}
	mygo.App.SetVersion(currentVersion())
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Minute)
	defer cancel()
	up, err := mygo.Updater.Check(ctx)
	// Re-fetch from the build's own feed and reject a release changed since
	// discovery. Install validates the archive with the build's embedded key.
	if err != nil || up == nil || up.Version != os.Args[2] {
		os.Exit(1)
	}
	if err := up.Install(ctx, nil); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
