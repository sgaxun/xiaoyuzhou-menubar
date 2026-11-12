package main

import (
	"github.com/egoist/mygo"
	"os"
	"path/filepath"
	"runtime"
)

func defaultDataDir() (string, error) {
	if custom := os.Getenv("XIAOYUZHOU_MENUBAR_DATA_DIR"); custom != "" {
		return custom, nil
	}
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "XiaoyuzhouMenuBar"), err
	}
	config, err := os.UserConfigDir()
	return filepath.Join(config, "XiaoyuzhouMenuBar"), err
}

// Prefer below the tray, switching above a bottom taskbar and clamping to the monitor.
func trayWindowPosition(anchor, bounds, area mygo.Rectangle) (int, int) {
	x := max(area.X, min(anchor.X+(anchor.Width-bounds.Width)/2, area.X+area.Width-bounds.Width))
	y := anchor.Y + anchor.Height + 6
	if y+bounds.Height > area.Y+area.Height {
		y = anchor.Y - bounds.Height - 6
	}
	y = max(area.Y, min(y, max(area.Y, area.Y+area.Height-bounds.Height)))
	return x, y
}
