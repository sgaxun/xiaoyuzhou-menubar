package main

import (
	"bytes"
	"image/png"
	"os"
	"runtime"
	"testing"
)

func TestSubtitleFixedViewport(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system macOS font")
	}
	text := "这一段字幕需要在固定宽度的菜单栏里滚动显示，不能再只显示开头，也不能改变菜单栏的宽度。"
	first, a, err := subtitleFrame(text, 0)
	if err != nil {
		t.Fatal(err)
	}
	last, b, err := subtitleFrame(text, 1)
	if err != nil {
		t.Fatal(err)
	}
	short, _, err := subtitleFrame("短句", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{first, last, short} {
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != 476 || img.Bounds().Dy() != 36 {
			t.Fatal("viewport changed")
		}
	}
	if a != 0 || b <= a || bytes.Equal(first, last) {
		t.Fatal("long subtitle did not scroll")
	}
	if path := os.Getenv("XYZ_SUBTITLE_PREVIEW"); path != "" {
		if err = os.WriteFile(path, first, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
