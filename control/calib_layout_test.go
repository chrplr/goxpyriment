// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"math"
	"testing"

	"github.com/chrplr/goxpyriment/eyetracker"
)

// checkLayout verifies that every point is at least clear from the edges of
// visible and from avoid, and that the pattern is still the standard one up to
// a scale and a shift per axis.
func checkLayout(t *testing.T, name string, got, base [][2]float64, w, h float64, visible, avoid rect, clear float64) {
	t.Helper()
	if len(got) != len(base) {
		t.Fatalf("%s: %d points, want %d", name, len(got), len(base))
	}
	for i, p := range got {
		x, y := p[0]*w, p[1]*h
		if x < visible.X+clear-1e-9 || x > visible.X+visible.W-clear+1e-9 ||
			y < visible.Y+clear-1e-9 || y > visible.Y+visible.H-clear+1e-9 {
			t.Errorf("%s: point %d at (%.1f, %.1f) is within %.0f px of the visible edge %+v", name, i, x, y, clear, visible)
		}
		if !avoid.empty() && x > avoid.X-clear && x < avoid.X+avoid.W+clear &&
			y > avoid.Y-clear && y < avoid.Y+avoid.H+clear {
			t.Errorf("%s: point %d at (%.1f, %.1f) overlaps the avoided %+v", name, i, x, y, avoid)
		}
	}
	// Same pattern: an affine map per axis from base to got, fitted on two
	// points that differ on each axis (the centre and a corner).
	sx := (got[1][0] - got[0][0]) / (base[1][0] - base[0][0])
	sy := (got[1][1] - got[0][1]) / (base[1][1] - base[0][1])
	for i := range base {
		ex := got[0][0] + sx*(base[i][0]-base[0][0])
		ey := got[0][1] + sy*(base[i][1]-base[0][1])
		if math.Abs(ex-got[i][0]) > 1e-9 || math.Abs(ey-got[i][1]) > 1e-9 {
			t.Errorf("%s: point %d is %v, off the pattern (want %v, %v)", name, i, got[i], ex, ey)
		}
	}
}

func TestFitLayoutStandardWhenItFits(t *testing.T) {
	for _, n := range []int{3, 5, 9, 13} {
		base := eyetracker.StandardPoints(n)
		got := fitLayout(base, 1024, 768, rect{0, 0, 1024, 768}, rect{}, 20)
		for i := range base {
			if math.Abs(got[i][0]-base[i][0]) > 1e-12 || math.Abs(got[i][1]-base[i][1]) > 1e-12 {
				t.Errorf("n=%d point %d moved from %v to %v with nothing in the way", n, i, base[i], got[i])
			}
		}
	}
}

// The case seen in a windowed run: 1024x768, the 100 px photodiode square
// top-left, shift -80 and magnification 1.05 pushing the left column off the
// screen. The visible area is what VisibleRect returns for that transform.
func TestFitLayoutAvoidsEdgeAndPhotodiode(t *testing.T) {
	const w, h, clear = 1024.0, 768.0, 20.0
	visible := rect{X: 512 - (512-80)/1.05, Y: 384 - 384/1.05, W: 0, H: 0}
	visible.W = 1024 - visible.X
	visible.H = (384 + 384/1.05) - visible.Y
	// The square, converted to the drawing space as MarkerRect does: the
	// screen's (0,0)-(100,100) in drawing coordinates.
	avoid := rect{X: visible.X, Y: visible.Y, W: 100 / 1.05, H: 100 / 1.05}
	for _, n := range []int{9, 13} {
		base := eyetracker.StandardPoints(n)
		got := fitLayout(base, w, h, visible, avoid, clear)
		checkLayout(t, "transform+photodiode", got, base, w, h, visible, avoid, clear)
	}
	// Without a transform, only the square is in the way.
	full := rect{0, 0, w, h}
	sq := rect{0, 0, 100, 100}
	base := eyetracker.StandardPoints(9)
	got := fitLayout(base, w, h, full, sq, clear)
	checkLayout(t, "photodiode only", got, base, w, h, full, sq, clear)
	if got[0] == base[0] && got[1] == base[1] {
		t.Error("the top-left target was left on the photodiode square")
	}
}

func TestFitLayoutTooSmall(t *testing.T) {
	base := eyetracker.StandardPoints(9)
	got := fitLayout(base, 30, 30, rect{0, 0, 30, 30}, rect{}, 20)
	for i := range base {
		if got[i] != base[i] {
			t.Fatalf("a screen too small for the targets should fall back to the standard layout, got %v", got)
		}
	}
}
