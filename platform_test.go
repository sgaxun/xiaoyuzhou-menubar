package main

import (
	"github.com/egoist/mygo"
	"path/filepath"
	"testing"
)

func TestTrayWindowPlacement(t *testing.T) {
	window := mygo.Rectangle{Width: 460, Height: 620}
	for _, tc := range []struct {
		name       string
		tray, area mygo.Rectangle
		x, y       int
	}{
		{"top menu", mygo.Rectangle{X: 900, Y: 0, Width: 24, Height: 24}, mygo.Rectangle{Y: 24, Width: 1440, Height: 876}, 682, 30},
		{"bottom taskbar", mygo.Rectangle{X: 1400, Y: 860, Width: 24, Height: 40}, mygo.Rectangle{Width: 1440, Height: 860}, 980, 234},
		{"left monitor", mygo.Rectangle{X: -50, Y: 1040, Width: 24, Height: 40}, mygo.Rectangle{X: -1920, Width: 1920, Height: 1040}, -460, 414},
		{"small display", mygo.Rectangle{X: 600, Y: 460, Width: 24, Height: 20}, mygo.Rectangle{Width: 640, Height: 460}, 180, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x, y := trayWindowPosition(tc.tray, window, tc.area)
			if x != tc.x || y != tc.y {
				t.Fatalf("got %d,%d want %d,%d", x, y, tc.x, tc.y)
			}
		})
	}
}
func TestCustomDataDirectory(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom")
	t.Setenv("XIAOYUZHOU_MENUBAR_DATA_DIR", want)
	got, err := defaultDataDir()
	if err != nil || got != want {
		t.Fatalf("%q %v", got, err)
	}
}
