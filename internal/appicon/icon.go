// Package appicon renders the shared planet shape for the tray and app bundle.
package appicon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

func planet(x, y float64) bool {
	const angle = -38 * math.Pi / 180
	c, s := math.Cos(angle), math.Sin(angle)
	u, v := c*x+s*y, -s*x+c*y
	const a, b = 17.0, 4.8
	gradient := 2 * math.Hypot(u/(a*a), v/(b*b))
	distance := math.Inf(1)
	if gradient > 0 {
		distance = math.Abs((u*u/(a*a) + v*v/(b*b) - 1) / gradient)
	}
	body := x*x+y*y <= 10.5*10.5
	if body && v > 0 && distance < 1.9 {
		body = false
	}
	return body || distance <= 0.85
}
func encode(img image.Image) []byte {
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	return out.Bytes()
}
func TemplatePNG() []byte {
	const size, samples = 36, 8
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			count := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					if planet(float64(x)+(float64(sx)+.5)/samples-size/2, float64(y)+(float64(sy)+.5)/samples-size/2) {
						count++
					}
				}
			}
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: uint8(count * 255 / (samples * samples))})
		}
	}
	return encode(img)
}
func AppPNG() []byte {
	const size, samples = 1024, 4
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r, g, b, coverage := 0., 0., 0., 0.
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					px, py := float64(x)+(float64(sx)+.5)/samples, float64(y)+(float64(sy)+.5)/samples
					// Rounded square: retain the existing purple app background.
					dx, dy := math.Max(math.Abs(px-512)-224, 0), math.Max(math.Abs(py-512)-224, 0)
					if dx*dx+dy*dy > 256*256 {
						continue
					}
					t := math.Max(0, math.Min(1, (px+py-64)/1920))
					cr, cg, cb := 91+(44-91)*t, 107+(30-107)*t, 245+(138-245)*t
					if planet((px-512)/22, (py-512)/22) {
						cr, cg, cb = 255, 255, 255
					}
					r += cr
					g += cg
					b += cb
					coverage++
				}
			}
			if coverage > 0 {
				img.SetNRGBA(x, y, color.NRGBA{R: uint8(r / coverage), G: uint8(g / coverage), B: uint8(b / coverage), A: uint8(coverage * 255 / (samples * samples))})
			}
		}
	}
	return encode(img)
}
