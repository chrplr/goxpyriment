// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"math"

	"github.com/chrplr/goxpyriment/eyetracker"
)

// calLayoutMargin is the clearance, in drawing pixels, kept between a target's
// edge and the edge of the visible screen or the photodiode square.
const calLayoutMargin = 4

// CalibrationPoints returns n gaze-tracker targets (3, 5, 9 or 13, as
// eyetracker.StandardPoints) in normalized drawing-space coordinates, origin
// top-left, placed so that a disc of the given radius in drawing pixels is
// seen whole at each of them: inside what the display transform leaves on the
// screen (Screen.VisibleRect), and clear of the photodiode square
// (Screen.MarkerRect).
//
// The layout is the standard one, spanning 0.1-0.9 of the drawing space, when
// that fits. Otherwise it keeps the same pattern, symmetric about its own
// centre, shrunk or moved just enough — so the centre target is then not
// necessarily at the centre of the screen. A tracker told these positions (an
// EyeLink is told every one beforehand) calibrates on them as on any other.
//
// Without a screen it returns eyetracker.StandardPoints(n).
func (e *Experiment) CalibrationPoints(n int, radius float32) [][2]float64 {
	base := eyetracker.StandardPoints(n)
	if e == nil || e.Screen == nil || e.Screen.Width <= 0 || e.Screen.Height <= 0 {
		return base
	}
	v := e.Screen.VisibleRect()
	m := e.Screen.MarkerRect()
	return fitLayout(base, float64(e.Screen.Width), float64(e.Screen.Height),
		rect{float64(v.X), float64(v.Y), float64(v.W), float64(v.H)},
		rect{float64(m.X), float64(m.Y), float64(m.W), float64(m.H)},
		float64(radius)+calLayoutMargin)
}

// rect is a rectangle in drawing pixels, top-left origin.
type rect struct{ X, Y, W, H float64 }

func (r rect) empty() bool { return !(r.W > 0) || !(r.H > 0) }

// fitLayout places the pattern of base (normalized on a w×h drawing space)
// inside visible, each point at least clear from its edges and from the
// avoided rectangle (the photodiode square; empty for none), and returns it
// normalized again.
//
// The pattern is described by its centre and half-extents. Each axis keeps its
// standard half-extent if the visible area allows it, and the centre moves as
// little as possible to fit; otherwise the half-extent shrinks to what fits.
// If a point then still overlaps the avoided rectangle, the area is reduced to
// the side of that rectangle — left, right, above or below — that keeps the
// largest pattern, and the fit is redone.
func fitLayout(base [][2]float64, w, h float64, visible, avoid rect, clear float64) [][2]float64 {
	if len(base) == 0 {
		return base
	}
	// The pattern in units of its half-extents, u in [-1, 1].
	minX, maxX, minY, maxY := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, p := range base {
		minX, maxX = math.Min(minX, p[0]), math.Max(maxX, p[0])
		minY, maxY = math.Min(minY, p[1]), math.Max(maxY, p[1])
	}
	cx, cy := (minX+maxX)/2*w, (minY+maxY)/2*h
	ax, ay := (maxX-minX)/2*w, (maxY-minY)/2*h
	unit := make([][2]float64, len(base))
	for i, p := range base {
		unit[i] = [2]float64{ratio(p[0]*w-cx, ax), ratio(p[1]*h-cy, ay)}
	}

	place := func(area rect) (pts [][2]float64, px, py, hx, hy float64, ok bool) {
		in := rect{area.X + clear, area.Y + clear, area.W - 2*clear, area.H - 2*clear}
		if in.W < 0 || in.H < 0 {
			return nil, 0, 0, 0, 0, false
		}
		hx, px = fitAxis(cx, ax, in.X, in.W)
		hy, py = fitAxis(cy, ay, in.Y, in.H)
		pts = make([][2]float64, len(unit))
		for i, u := range unit {
			pts[i] = [2]float64{px + u[0]*hx, py + u[1]*hy}
		}
		return pts, px, py, hx, hy, true
	}
	hits := func(pts [][2]float64) bool {
		if avoid.empty() {
			return false
		}
		for _, p := range pts {
			if p[0] > avoid.X-clear && p[0] < avoid.X+avoid.W+clear &&
				p[1] > avoid.Y-clear && p[1] < avoid.Y+avoid.H+clear {
				return true
			}
		}
		return false
	}

	area := visible
	if area.empty() {
		area = rect{0, 0, w, h}
	}
	pts, _, _, _, _, ok := place(area)
	if !ok {
		return base
	}
	if hits(pts) {
		// The four ways of leaving the avoided rectangle out of the area.
		cands := []rect{
			{avoid.X + avoid.W, area.Y, area.X + area.W - (avoid.X + avoid.W), area.H}, // right of it
			{area.X, area.Y, avoid.X - area.X, area.H},                                 // left of it
			{area.X, avoid.Y + avoid.H, area.W, area.Y + area.H - (avoid.Y + avoid.H)}, // below it
			{area.X, area.Y, area.W, avoid.Y - area.Y},                                 // above it
		}
		best, bestSize := [][2]float64(nil), -1.0
		for _, c := range cands {
			if c.empty() {
				continue
			}
			p, _, _, hx, hy, ok := place(c)
			if !ok || hits(p) {
				continue
			}
			if size := hx * hy; size > bestSize {
				best, bestSize = p, size
			}
		}
		if best != nil {
			pts = best
		}
	}
	out := make([][2]float64, len(pts))
	for i, p := range pts {
		out[i] = [2]float64{p[0] / w, p[1] / h}
	}
	return out
}

// fitAxis returns the half-extent and centre of a pattern of half-extent a
// centred on c, fitted inside [lo, lo+span]: the same half-extent and the
// nearest centre if it fits, else the largest half-extent that fits, centred.
func fitAxis(c, a, lo, span float64) (half, centre float64) {
	if 2*a > span {
		return span / 2, lo + span/2
	}
	return a, math.Min(math.Max(c, lo+a), lo+span-a)
}

func ratio(d, a float64) float64 {
	if a == 0 {
		return 0
	}
	return d / a
}
