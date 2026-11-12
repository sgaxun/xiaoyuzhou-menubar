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
	"runtime"
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

func loadSubtitleFont() (font.Face, error) {
	subtitleFont.Do(func() {
		paths := subtitleFontPaths()
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
		return nil, fmt.Errorf("字幕字体加载失败: %w", subtitleFont.err)
	}
	return subtitleFont.face, nil
}

// Windows resolves fonts from its actual installation directory, including TTC
// Chinese fonts and TTF fallbacks. Keep the macOS candidate order unchanged.
func subtitleFontPaths() []string {
	if runtime.GOOS == "windows" {
		root := os.Getenv("WINDIR")
		if root == "" {
			root = `C:\Windows`
		}
		fonts := filepath.Join(root, "Fonts")
		return []string{filepath.Join(fonts, "msyh.ttc"), filepath.Join(fonts, "msyh.ttf"), filepath.Join(fonts, "msjh.ttc"), filepath.Join(fonts, "simhei.ttf"), filepath.Join(fonts, "simsun.ttc"), filepath.Join(fonts, "segoeui.ttf")}
	}
	paths, _ := filepath.Glob("/System/Library/AssetsV2/com_apple_MobileAsset_Font*/*/AssetData/PingFang.ttc")
	return append(paths, "/System/Library/Fonts/PingFang.ttc", "/System/Library/Fonts/STHeiti Light.ttc")
}

const subtitleWidth, subtitleHeight, subtitleLeft = 476, 36, 48

// One current-text entry bounds retention across episodes. Oversized lines keep
// their measurement but are drawn directly rather than retaining a huge bitmap.
const subtitleMaxRasterWidth = 32768

type subtitleRenderer struct {
	base    image.Image
	basePNG []byte
	text    string
	extent  int
	raster  *image.NRGBA
	face    font.Face
}

func (r *subtitleRenderer) baseIcon() ([]byte, error) {
	if r.base == nil {
		raw := trayIcon()
		icon, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		r.base, r.basePNG = icon, raw
	}
	return r.basePNG, nil
}

func normalizeSubtitle(text string) string { return strings.Join(strings.Fields(text), " ") }

// prepare measures/rasterizes only on text changes, before any frame work.
func (r *subtitleRenderer) prepare(text string, progress float64) (int, error) {
	if r.face == nil {
		face, err := loadSubtitleFont()
		if err != nil {
			return 0, err
		}
		r.face = face
	}
	if r.text != text {
		r.text = text
		r.extent = font.MeasureString(r.face, text).Ceil()
		r.raster = nil
		// Include glyph overhang, retaining the original clipping at the viewport.
		bounds, _ := font.BoundString(r.face, text)
		right := max(r.extent, bounds.Max.X.Ceil())
		left := min(0, bounds.Min.X.Floor())
		if right-left <= subtitleMaxRasterWidth {
			r.raster = image.NewNRGBA(image.Rect(left, 0, max(left+1, right), subtitleHeight))
			r.drawText(r.raster, 0)
		}
	}
	if math.IsNaN(progress) || math.IsInf(progress, 0) {
		progress = 0
	}
	progress = math.Max(0, math.Min(1, progress))
	return int(math.Round(float64(max(0, r.extent-(subtitleWidth-subtitleLeft))) * progress)), nil
}

func (r *subtitleRenderer) drawText(dst draw.Image, x int) {
	metrics := r.face.Metrics()
	baseline := (fixed.I(subtitleHeight)-metrics.Height)/2 + metrics.Ascent
	d := font.Drawer{Dst: dst, Src: image.NewUniform(color.White), Face: r.face, Dot: fixed.Point26_6{X: fixed.I(x), Y: baseline}}
	d.DrawString(r.text)
}

func (r *subtitleRenderer) render(offset int) ([]byte, error) {
	if _, err := r.baseIcon(); err != nil {
		return nil, err
	}
	img := image.NewNRGBA(image.Rect(0, 0, subtitleWidth, subtitleHeight))
	draw.Draw(img, image.Rect(0, 0, 36, 36), r.base, image.Point{}, draw.Src)
	viewport := image.Rect(subtitleLeft, 0, subtitleWidth, subtitleHeight)
	if r.raster != nil {
		draw.Draw(img, viewport, r.raster, image.Pt(offset, 0), draw.Over)
	} else {
		r.drawText(img.SubImage(viewport).(*image.NRGBA), subtitleLeft-offset)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// update commits the displayed key only after a successful native update. A
// failed SetIcon therefore remains retryable, including blank-icon resets.
func (r *subtitleRenderer) update(text string, progress float64, displayed *string, displayedOffset *int, setIcon func([]byte) error) error {
	text = normalizeSubtitle(text)
	if text == "" {
		if *displayed == "" {
			return nil
		}
		raw, err := r.baseIcon()
		if err != nil {
			return err
		}
		if err = setIcon(raw); err != nil {
			return err
		}
		*displayed, *displayedOffset = "", 0
		return nil
	}
	offset, err := r.prepare(text, progress)
	if err != nil {
		return err
	}
	if *displayed == text && *displayedOffset == offset {
		return nil
	}
	raw, err := r.render(offset)
	if err != nil {
		return err
	}
	if err = setIcon(raw); err != nil {
		return err
	}
	*displayed, *displayedOffset = text, offset
	return nil
}
