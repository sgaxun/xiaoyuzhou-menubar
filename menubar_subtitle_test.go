package main

import (
	"bytes"
	"errors"
	"image/png"
	"math"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestSubtitleFixedViewport(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system macOS font")
	}
	var renderer subtitleRenderer
	frame := func(text string, progress float64) ([]byte, int, error) {
		offset, err := renderer.prepare(text, progress)
		if err != nil {
			return nil, 0, err
		}
		raw, err := renderer.render(offset)
		return raw, offset, err
	}
	text := "这一段字幕需要在固定宽度的菜单栏里滚动显示，不能再只显示开头，也不能改变菜单栏的宽度。"
	first, a, err := frame(text, 0)
	if err != nil {
		t.Fatal(err)
	}
	last, b, err := frame(text, 1)
	if err != nil {
		t.Fatal(err)
	}
	short, _, err := frame("短句", 0)
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

func TestSubtitleCacheAndUpdates(t *testing.T) {
	var r subtitleRenderer
	var displayed string
	var offset, calls int
	set := func(raw []byte) error {
		calls++
		if len(raw) == 0 {
			t.Fatal("empty icon")
		}
		return nil
	}
	text := strings.Repeat("字幕滚动测试", 30)
	update := func(text string, progress float64) {
		t.Helper()
		if err := r.update(text, progress, &displayed, &offset, set); err != nil {
			t.Fatal(err)
		}
	}
	update(text, .5)
	raster, base := r.raster, r.base
	if raster == nil || base == nil {
		t.Fatal("missing raster/base cache")
	}
	// Removing the base makes any accidental render observable: render reloads it.
	r.base = nil
	update(text, .5)
	update("  "+text+"  ", .5)
	update(text, .5+0.01/float64(r.extent))
	if calls != 1 || r.base != nil || r.raster != raster {
		t.Fatal("unchanged pixel performed frame work")
	}
	r.base = base
	update(text, 1)
	end := offset
	update(text, 0)
	if calls != 3 || end <= 0 || offset != 0 || r.raster != raster || r.base != base {
		t.Fatal("seek/cache behavior")
	}
	update("短句", 0)
	if r.raster == raster {
		t.Fatal("text raster not replaced")
	}
	shortRaster := r.raster
	update("短句", 1)
	if calls != 4 || r.raster != shortRaster {
		t.Fatal("short text should not rerender")
	}
	update(" \n\t", 0)
	update("", 0)
	if calls != 5 || displayed != "" || offset != 0 {
		t.Fatal("blank reset not idempotent")
	}
	update("短句", 0)
	if calls != 6 {
		t.Fatal("subtitle not restored after blank")
	}
	failure := errors.New("native failure")
	if err := r.update(text, 1, &displayed, &offset, func([]byte) error { return failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if displayed != "短句" {
		t.Fatal("failed update committed")
	}
	update(text, 1)
	if calls != 7 || displayed != text {
		t.Fatal("failed update not retried")
	}
	if err := r.update("", 0, &displayed, &offset, func([]byte) error { return failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	update("", 0)
	if calls != 8 || displayed != "" {
		t.Fatal("failed reset not retried")
	}
}

func TestSubtitleProgressAndBoundedRaster(t *testing.T) {
	var r subtitleRenderer
	text := strings.Repeat("长字幕", 20)
	for _, progress := range []float64{-1, math.NaN(), math.Inf(1)} {
		offset, err := r.prepare(text, progress)
		if err != nil || offset != 0 {
			t.Fatalf("invalid progress: %d %v", offset, err)
		}
	}
	end, _ := r.prepare(text, 1)
	beyond, _ := r.prepare(text, 2)
	if end <= 0 || beyond != end {
		t.Fatal("progress not clamped")
	}
	if _, err := r.prepare(strings.Repeat("字幕", 10000), 1); err != nil {
		t.Fatal(err)
	}
	if r.raster != nil {
		t.Fatal("oversized raster retained")
	}
	raw, err := r.render(0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil || img.Bounds().Dx() != 476 || img.Bounds().Dy() != 36 {
		t.Fatal("oversized line changed viewport", err)
	}
	if _, err := r.prepare("短句", 0); err != nil || r.raster == nil {
		t.Fatal("cache did not recover", err)
	}
}
