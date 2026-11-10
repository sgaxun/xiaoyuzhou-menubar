package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Render the icon and text into one fixed-size template image. AppKit sizes
// titles by their contents, so a title cannot provide a stable scrolling slot.
var subtitleFont struct {
	sync.Once
	face font.Face
	err  error
}

func subtitleFrame(text string, progress float64) ([]byte, int, error) {
	subtitleFont.Do(func() {
		paths, _ := filepath.Glob("/System/Library/AssetsV2/com_apple_MobileAsset_Font*/*/AssetData/PingFang.ttc")
		paths = append(paths, "/System/Library/Fonts/PingFang.ttc", "/System/Library/Fonts/STHeiti Light.ttc")
		for _, path := range paths {
			raw, err := os.ReadFile(path)
			if err != nil {
				subtitleFont.err = err
				continue
			}
			collection, err := opentype.ParseCollection(raw)
			if err != nil {
				subtitleFont.err = err
				continue
			}
			f, err := collection.Font(0)
			if err != nil {
				subtitleFont.err = err
				continue
			}
			subtitleFont.face, err = opentype.NewFace(f, &opentype.FaceOptions{Size: 26, DPI: 72, Hinting: font.HintingFull})
			subtitleFont.err = err
			if err == nil {
				return
			}
		}
	})
	if subtitleFont.err != nil {
		return nil, 0, fmt.Errorf("字幕字体加载失败: %w", subtitleFont.err)
	}
	text = strings.Join(strings.Fields(text), " ")
	const width, height, left = 476, 36, 48
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	icon, err := png.Decode(bytes.NewReader(trayIcon()))
	if err != nil {
		return nil, 0, err
	}
	draw.Draw(img, image.Rect(0, 0, 36, 36), icon, image.Point{}, draw.Src)
	face := subtitleFont.face
	extent := font.MeasureString(face, text).Ceil()
	if math.IsNaN(progress) || math.IsInf(progress, 0) {
		progress = 0
	}
	progress = math.Max(0, math.Min(1, progress))
	offset := int(math.Round(float64(max(0, extent-(width-left))) * progress))
	clip := img.SubImage(image.Rect(left, 0, width, height)).(*image.NRGBA)
	metrics := face.Metrics()
	baseline := (fixed.I(height)-metrics.Height)/2 + metrics.Ascent
	d := font.Drawer{Dst: clip, Src: image.NewUniform(color.White), Face: face, Dot: fixed.Point26_6{X: fixed.I(left - offset), Y: baseline}}
	d.DrawString(text)
	var out bytes.Buffer
	if err = png.Encode(&out, img); err != nil {
		return nil, 0, err
	}
	return out.Bytes(), offset, nil
}
