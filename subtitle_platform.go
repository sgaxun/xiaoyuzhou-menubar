package main

import "github.com/egoist/mygo"

// Screen work areas are DIPs and may have negative origins. Keep the strip
// wholly inside even unusually small work areas, with a bottom inset if it fits.
func subtitleStripBounds(area mygo.Rectangle) mygo.Rectangle {
	width, height := min(subtitleWidth, area.Width), min(subtitleHeight, area.Height)
	return mygo.Rectangle{X: area.X + (area.Width-width)/2, Y: max(area.Y, area.Y+area.Height-height-12), Width: width, Height: height}
}
