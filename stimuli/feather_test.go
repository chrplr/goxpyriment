// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

import (
	"math"
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
)

var opaqueWhite = sdl.FColor{R: 1, G: 1, B: 1, A: 1}

// checkMesh fails on an index out of range or a count that is not whole triangles.
func checkMesh(t *testing.T, verts []sdl.Vertex, indices []int32) {
	t.Helper()
	if len(indices) == 0 || len(indices)%3 != 0 {
		t.Fatalf("got %d indices, want a positive multiple of 3", len(indices))
	}
	for _, i := range indices {
		if i < 0 || int(i) >= len(verts) {
			t.Fatalf("index %d out of range [0, %d)", i, len(verts))
		}
	}
}

func dist(p sdl.FPoint, x, y float32) float64 {
	return math.Hypot(float64(p.X-x), float64(p.Y-y))
}

func TestArcSegments(t *testing.T) {
	for _, r := range []float64{0.2, 1, 3, 10, 100, 1000} {
		n := arcSegments(r, 2*math.Pi)
		if n < 16 || n > 512 {
			t.Errorf("r=%g: %d segments, want 16..512", r, n)
		}
		if n < 512 {
			if sag := r * (1 - math.Cos(math.Pi/float64(n))); sag > maxSagitta+1e-9 {
				t.Errorf("r=%g: sagitta %.3f px with %d segments", r, sag, n)
			}
		}
	}
	if n := arcSegments(10, 0); n != 0 {
		t.Errorf("zero sweep: %d segments, want 0", n)
	}
}

func TestDiscFeather(t *testing.T) {
	const cx, cy, r = 50, 40, 10
	verts, indices := appendDisc(nil, nil, cx, cy, r, opaqueWhite, false)
	checkMesh(t, verts, indices)
	for _, v := range verts {
		d := dist(v.Position, cx, cy)
		switch v.Color.A {
		case 1:
			if d > r-featherHalf+1e-3 {
				t.Errorf("opaque vertex at %.3f px, want <= %.1f", d, r-featherHalf)
			}
		case 0:
			if math.Abs(d-(r+featherHalf)) > 1e-3 {
				t.Errorf("transparent vertex at %.3f px, want %.1f", d, r+featherHalf)
			}
		default:
			t.Errorf("unexpected alpha %g", v.Color.A)
		}
	}

	verts, indices = appendDisc(nil, nil, cx, cy, r, opaqueWhite, true)
	checkMesh(t, verts, indices)
	for _, v := range verts {
		if v.Color.A != 1 {
			t.Fatalf("aliased disc has a vertex with alpha %g", v.Color.A)
		}
		if d := dist(v.Position, cx, cy); d > r+1e-3 {
			t.Errorf("aliased vertex at %.3f px, want <= %d", d, r)
		}
	}
}

func TestTinyDiscFades(t *testing.T) {
	verts, _ := appendDisc(nil, nil, 0, 0, 0.25, opaqueWhite, false)
	if a := verts[0].Color.A; a <= 0 || a >= 1 {
		t.Errorf("centre alpha of a 0.5 px dot = %g, want strictly between 0 and 1", a)
	}
}

func TestConvexFillFeather(t *testing.T) {
	// A 20° isosceles triangle, both windings, +Y-up points about (100, 100).
	tri := []sdl.FPoint{{X: 0, Y: 100}, {X: -17.6, Y: 0}, {X: 17.6, Y: 0}}
	rev := []sdl.FPoint{tri[2], tri[1], tri[0]}
	for name, pts := range map[string][]sdl.FPoint{"cw": tri, "ccw": rev} {
		verts, indices := appendConvexFill(nil, nil, 100, 100, pts, opaqueWhite, false)
		checkMesh(t, verts, indices)
		if len(verts) != 2*len(pts) {
			t.Fatalf("%s: %d vertices, want %d", name, len(verts), 2*len(pts))
		}
		cx, cy := float32(100), float32(100-100.0/3) // centroid in SDL space
		for i, p := range pts {
			in, out := verts[2*i], verts[2*i+1]
			if in.Color.A != 1 || out.Color.A != 0 {
				t.Errorf("%s corner %d: alphas %g/%g, want 1/0", name, i, in.Color.A, out.Color.A)
			}
			corner := sdl.FPoint{X: 100 + p.X, Y: 100 - p.Y}
			if d := dist(out.Position, corner.X, corner.Y); d > 10*featherHalf+1e-3 {
				t.Errorf("%s corner %d: fringe %.2f px from the corner", name, i, d)
			}
			// The fringe vertex lies outside the polygon, the core one inside.
			if dist(out.Position, cx, cy) <= dist(corner, cx, cy) || dist(in.Position, cx, cy) >= dist(corner, cx, cy) {
				t.Errorf("%s corner %d: inner/outer vertices on the wrong side", name, i)
			}
		}
	}
}

func TestConvexFillAliasedUnchanged(t *testing.T) {
	pts := []sdl.FPoint{{X: -10, Y: -10}, {X: 10, Y: -10}, {X: 10, Y: 10}, {X: -10, Y: 10}}
	verts, indices := appendConvexFill(nil, nil, 0, 0, pts, opaqueWhite, true)
	if len(verts) != 4 || len(indices) != 6 {
		t.Fatalf("aliased square: %d vertices, %d indices; want the 4/6 triangle fan", len(verts), len(indices))
	}
}

func TestStrokeFeather(t *testing.T) {
	// A zig-zag with a sharp (~17°) vertex at (100, 0), open and closed.
	pts := []sdl.FPoint{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 0, Y: 30}}
	const hw = 3
	for _, closed := range []bool{false, true} {
		nSeg := len(pts) - 1
		if closed {
			nSeg = len(pts)
		}
		for _, aliased := range []bool{false, true} {
			verts, indices := appendStroke(nil, nil, pts, closed, hw, opaqueWhite, aliased)
			checkMesh(t, verts, indices)
			// No vertex beyond the outline, and every transparent one on the
			// outline of some segment. (On the inner side of a join a fringe
			// vertex may lie inside the neighbouring stroke: it is drawn over
			// opaque core of the same colour and changes nothing.)
			reach := float64(hw)
			if !aliased {
				reach += featherHalf
			}
			for _, v := range verts {
				onOutline, d := false, math.Inf(1)
				for i := 0; i < nSeg; i++ {
					di := segDist(v.Position, pts[i], pts[(i+1)%len(pts)])
					d = math.Min(d, di)
					onOutline = onOutline || math.Abs(di-reach) < 1e-3
				}
				if d > reach+1e-3 {
					t.Errorf("closed=%v aliased=%v: vertex %.2f px from the line, want <= %.1f", closed, aliased, d, reach)
				}
				if v.Color.A == 0 && !onOutline {
					t.Errorf("closed=%v: transparent vertex not on the outline of any segment", closed)
				}
			}
			// The outer side of the sharp join must be filled: neither quad
			// reaches it, only the join wedge does.
			d1x, d1y := 1.0, 0.0
			l := math.Hypot(100, 30)
			d2x, d2y := -100/l, 30/l
			ox, oy := d1x-d2x, d1y-d2y
			ol := math.Hypot(ox, oy)
			probe := sdl.FPoint{X: 100 + float32((hw-1)*ox/ol), Y: float32((hw - 1) * oy / ol)}
			if !coveredOpaque(verts, indices, probe) {
				t.Errorf("closed=%v aliased=%v: outer side of the sharp join is not filled", closed, aliased)
			}
		}
	}
}

// coveredOpaque reports whether p lies in a triangle whose three vertices are opaque.
func coveredOpaque(verts []sdl.Vertex, indices []int32, p sdl.FPoint) bool {
	cross := func(a, b sdl.FPoint) float32 {
		return (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)
	}
	for k := 0; k+2 < len(indices); k += 3 {
		a, b, c := verts[indices[k]], verts[indices[k+1]], verts[indices[k+2]]
		if a.Color.A != 1 || b.Color.A != 1 || c.Color.A != 1 {
			continue
		}
		s1, s2, s3 := cross(a.Position, b.Position), cross(b.Position, c.Position), cross(c.Position, a.Position)
		if (s1 >= 0 && s2 >= 0 && s3 >= 0) || (s1 <= 0 && s2 <= 0 && s3 <= 0) {
			return true
		}
	}
	return false
}

func TestStrokeSinglePointIsDot(t *testing.T) {
	verts, indices := appendStroke(nil, nil, []sdl.FPoint{{X: 5, Y: 5}}, false, 2, opaqueWhite, false)
	checkMesh(t, verts, indices)
}

// segDist is the distance from p to the segment a–b.
func segDist(p, a, b sdl.FPoint) float64 {
	dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
	t := (float64(p.X-a.X)*dx + float64(p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(float64(p.X)-float64(a.X)-t*dx, float64(p.Y)-float64(a.Y)-t*dy)
}

func TestRedrawDoesNotAllocate(t *testing.T) {
	pts := []sdl.FPoint{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 0, Y: 30}}
	var verts []sdl.Vertex
	var indices []int32
	frame := func() {
		verts, indices = appendDisc(verts[:0], indices[:0], 10, 10, 5, opaqueWhite, false)
		verts, indices = appendConvexFill(verts, indices, 0, 0, pts, opaqueWhite, false)
		verts, indices = appendStroke(verts, indices, pts, true, 3, opaqueWhite, false)
	}
	frame()
	if n := testing.AllocsPerRun(20, frame); n != 0 {
		t.Errorf("redrawing allocated %v times per frame, want 0", n)
	}
}
