package main

import (
	"github.com/egoist/mygo"
	"testing"
)

func TestTrayBlurHitArea(t *testing.T) {
	bounds := mygo.Rectangle{X: 100, Y: 0, Width: 24, Height: 24}
	for _, tc := range []struct {
		point mygo.Point
		want  bool
	}{
		{mygo.Point{X: 100, Y: 0}, true},
		{mygo.Point{X: 112, Y: 12}, true},
		{mygo.Point{X: 124, Y: 12}, false},
		{mygo.Point{X: 112, Y: 24}, false},
		{mygo.Point{X: 99, Y: 12}, false},
	} {
		if got := pointInTray(tc.point, bounds); got != tc.want {
			t.Fatalf("point %v: got %v", tc.point, got)
		}
	}
	if pointInTray(mygo.Point{}, mygo.Rectangle{}) {
		t.Fatal("empty bounds must not suppress blur")
	}
}
