// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

// feather.go — antialiased geometry for the filled and stroked shapes.
//
// SDL's 2D renderer rasterises triangles without coverage blending and has no
// multisampling, so a plain RenderGeometry fill has staircased edges. These
// helpers antialias by edge feathering: every outline gets a second ring of
// vertices, one pixel wide, whose alpha falls linearly from the shape colour
// on the inside (edge - 0.5 px) to zero on the outside (edge + 0.5 px). The
// GPU interpolates the alpha across that ring, which approximates the pixel
// coverage of the edge.
//
// An axis-aligned edge lying on a pixel boundary stays crisp: the pixel centre
// half a pixel inside gets full alpha, the one half a pixel outside gets zero.
//
// The append* helpers only append to the slices they are given, so callers
// that keep the slices between frames allocate nothing once they have grown.
// Points are in SDL space (Y down) unless stated otherwise.

import (
	"fmt"
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

const (
	// featherHalf is half the width of the alpha ramp along every edge.
	featherHalf = 0.5

	// maxSagitta bounds the gap, in pixels, between an arc and the chords
	// that approximate it.
	maxSagitta = 0.1

	// maxMiterScale bounds 1/|averaged normal|² at polygon corners, so the
	// fringe of a very sharp corner cannot shoot out arbitrarily far
	// (10× the half-feather at most, as in Dear ImGui).
	maxMiterScale = 100
)

// toFColor converts an 8-bit colour to the float colour RenderGeometry takes.
func toFColor(c sdl.Color) sdl.FColor {
	return sdl.FColor{
		R: float32(c.R) / 255,
		G: float32(c.G) / 255,
		B: float32(c.B) / 255,
		A: float32(c.A) / 255,
	}
}

// transparent returns fc with zero alpha: the colour of the outer fringe ring.
// Keeping the RGB of the shape avoids a dark halo while the alpha ramps down.
func transparent(fc sdl.FColor) sdl.FColor {
	fc.A = 0
	return fc
}

// arcSegments returns how many chords approximate an arc of radius r sweeping
// |sweep| radians with a sagitta of at most maxSagitta pixels. The count is at
// least 4 per quarter turn and at most 512 per full turn.
func arcSegments(r, sweep float64) int {
	sweep = math.Abs(sweep)
	if sweep == 0 {
		return 0
	}
	step := math.Pi
	if r > maxSagitta {
		step = 2 * math.Acos(1-maxSagitta/r)
	}
	n := int(math.Ceil(sweep / step))
	lo := int(math.Ceil(sweep / (math.Pi / 8)))
	hi := int(math.Ceil(sweep / (2 * math.Pi) * 512))
	return max(lo, min(n, hi))
}

// appendDisc appends a filled disc of radius r centred on (cx, cy).
//
// Aliased, it is a plain triangle fan. Antialiased, the opaque fan stops at
// r - 0.5 and a fringe ring runs out to r + 0.5. A disc thinner than one pixel
// keeps only the fringe, with its centre alpha scaled by the diameter, so tiny
// dots fade rather than vanish.
func appendDisc(verts []sdl.Vertex, indices []int32, cx, cy, r float32, fc sdl.FColor, aliased bool) ([]sdl.Vertex, []int32) {
	if r <= 0 {
		return verts, indices
	}
	return appendWedge(verts, indices, cx, cy, r, 0, 2*math.Pi, fc, aliased)
}

// appendWedge appends a pie slice of radius r centred on (cx, cy), from angle
// a0 sweeping by sweep radians (either sign; angles in SDL space). Only the
// arc is feathered: the two straight sides are left for the geometry the
// wedge abuts (PolyLine joins and caps). A full turn gives a disc.
func appendWedge(verts []sdl.Vertex, indices []int32, cx, cy, r float32, a0, sweep float64, fc sdl.FColor, aliased bool) ([]sdl.Vertex, []int32) {
	inner, outer := r, r
	core := fc
	if !aliased {
		inner = r - featherHalf
		outer = r + featherHalf
		if inner < 0 {
			inner = 0
			core.A *= min(1, 2*r)
		}
	}
	n := arcSegments(float64(outer), sweep)
	if n == 0 {
		return verts, indices
	}

	centre := int32(len(verts))
	verts = append(verts, sdl.Vertex{Position: sdl.FPoint{X: cx, Y: cy}, Color: core})
	first := int32(len(verts))
	stride := int32(1)
	if !aliased {
		stride = 2 // inner and outer vertex for every arc point
	}
	edge := transparent(fc)
	for k := 0; k <= n; k++ {
		a := a0 + sweep*float64(k)/float64(n)
		c, s := float32(math.Cos(a)), float32(math.Sin(a))
		verts = append(verts, sdl.Vertex{Position: sdl.FPoint{X: cx + inner*c, Y: cy + inner*s}, Color: core})
		if !aliased {
			verts = append(verts, sdl.Vertex{Position: sdl.FPoint{X: cx + outer*c, Y: cy + outer*s}, Color: edge})
		}
	}
	for k := int32(0); k < int32(n); k++ {
		in0 := first + k*stride
		in1 := in0 + stride
		indices = append(indices, centre, in0, in1)
		if !aliased {
			indices = append(indices, in0, in0+1, in1+1, in0, in1+1, in1)
		}
	}
	return verts, indices
}

// appendConvexFill appends a filled polygon. pts are relative to (cx, cy) in
// the +Y-up stimulus convention. The polygon is triangulated as a fan from
// pts[0], so it must be convex or at least star-shaped from that point.
//
// Antialiased, each corner is split along the averaged normal of its two
// edges (the Dear ImGui AddConvexPolyFilled scheme): the inner vertex, half a
// pixel inside, carries the opaque fan; the outer vertex, half a pixel
// outside, is transparent; one quad per edge joins them. Either winding works.
func appendConvexFill(verts []sdl.Vertex, indices []int32, cx, cy float32, pts []sdl.FPoint, fc sdl.FColor, aliased bool) ([]sdl.Vertex, []int32) {
	n := len(pts)
	if n < 3 {
		return verts, indices
	}
	at := func(i int) (float32, float32) {
		p := pts[i%n]
		return cx + p.X, cy - p.Y
	}

	first := int32(len(verts))
	if aliased {
		for i := 0; i < n; i++ {
			x, y := at(i)
			verts = append(verts, sdl.Vertex{Position: sdl.FPoint{X: x, Y: y}, Color: fc})
		}
		for i := int32(1); i < int32(n)-1; i++ {
			indices = append(indices, first, first+i, first+i+1)
		}
		return verts, indices
	}

	// Signed area in SDL space: positive means the outward normal of edge
	// (dx, dy) is (dy, -dx).
	var area float32
	for i := 0; i < n; i++ {
		x0, y0 := at(i)
		x1, y1 := at(i + 1)
		area += x0*y1 - x1*y0
	}
	sign := float32(1)
	if area < 0 {
		sign = -1
	}
	edgeNormal := func(i int) (float32, float32) {
		x0, y0 := at(i)
		x1, y1 := at(i + 1)
		dx, dy := x1-x0, y1-y0
		l := float32(math.Hypot(float64(dx), float64(dy)))
		if l == 0 {
			return 0, 0
		}
		return sign * dy / l, -sign * dx / l
	}

	edge := transparent(fc)
	pnx, pny := edgeNormal(n - 1)
	for i := 0; i < n; i++ {
		nx, ny := edgeNormal(i)
		mx, my := (pnx+nx)/2, (pny+ny)/2
		if d2 := mx*mx + my*my; d2 > 1e-6 {
			s := min(1/d2, maxMiterScale)
			mx, my = mx*s, my*s
		}
		mx, my = mx*featherHalf, my*featherHalf
		x, y := at(i)
		verts = append(verts,
			sdl.Vertex{Position: sdl.FPoint{X: x - mx, Y: y - my}, Color: fc},
			sdl.Vertex{Position: sdl.FPoint{X: x + mx, Y: y + my}, Color: edge})
		pnx, pny = nx, ny
	}
	for i := int32(1); i < int32(n)-1; i++ {
		indices = append(indices, first, first+2*i, first+2*(i+1))
	}
	for i := int32(0); i < int32(n); i++ {
		in0, out0 := first+2*i, first+2*i+1
		j := (i + 1) % int32(n)
		in1, out1 := first+2*j, first+2*j+1
		indices = append(indices, in0, out0, out1, in0, out1, in1)
	}
	return verts, indices
}

// appendStrokeQuad appends the band of half-width hw around the segment
// (x1, y1)–(x2, y2), whose unit normal is (nx, ny). The ends are cut square at
// the endpoints; PolyLine closes them with wedges.
//
// Antialiased, the band is three quads across: a transparent-to-opaque
// fringe, the opaque core of half-width hw - 0.5, and the opposite fringe.
// A stroke thinner than one pixel keeps only the two fringes, with its peak
// alpha scaled by the width.
func appendStrokeQuad(verts []sdl.Vertex, indices []int32, x1, y1, x2, y2, nx, ny, hw float32, fc sdl.FColor, aliased bool) ([]sdl.Vertex, []int32) {
	first := int32(len(verts))
	if aliased {
		ox, oy := nx*hw, ny*hw
		verts = append(verts,
			sdl.Vertex{Position: sdl.FPoint{X: x1 + ox, Y: y1 + oy}, Color: fc},
			sdl.Vertex{Position: sdl.FPoint{X: x2 + ox, Y: y2 + oy}, Color: fc},
			sdl.Vertex{Position: sdl.FPoint{X: x2 - ox, Y: y2 - oy}, Color: fc},
			sdl.Vertex{Position: sdl.FPoint{X: x1 - ox, Y: y1 - oy}, Color: fc})
		return verts, append(indices, first, first+1, first+2, first, first+2, first+3)
	}

	core := fc
	inner := hw - featherHalf
	if inner < 0 {
		inner = 0
		core.A *= min(1, 2*hw)
	}
	outer := hw + featherHalf
	edge := transparent(fc)
	// Four offsets across the band, each at both ends: +outer, +inner, -inner, -outer.
	for _, off := range [4]struct {
		d float32
		c sdl.FColor
	}{{outer, edge}, {inner, core}, {-inner, core}, {-outer, edge}} {
		ox, oy := nx*off.d, ny*off.d
		verts = append(verts,
			sdl.Vertex{Position: sdl.FPoint{X: x1 + ox, Y: y1 + oy}, Color: off.c},
			sdl.Vertex{Position: sdl.FPoint{X: x2 + ox, Y: y2 + oy}, Color: off.c})
	}
	for k := int32(0); k < 3; k++ {
		a1, a2 := first+2*k, first+2*k+1 // this offset, start and end
		b1, b2 := a1+2, a2+2             // next offset inwards
		indices = append(indices, a1, a2, b2, a1, b2, b1)
	}
	return verts, indices
}

// drawGeometry renders untextured triangles with alpha blending. Untextured
// RenderGeometry uses the renderer's draw blend mode, which is BLENDMODE_NONE
// unless something changed it; under NONE the transparent fringe would be
// written as an opaque border. The previous mode is restored afterwards.
func drawGeometry(screen *apparatus.Screen, verts []sdl.Vertex, indices []int32) error {
	if len(indices) == 0 {
		return nil
	}
	r := screen.Renderer
	prev, err := r.DrawBlendMode()
	if err != nil {
		return fmt.Errorf("reading blend mode: %w", err)
	}
	if prev != sdl.BLENDMODE_BLEND {
		if err := r.SetDrawBlendMode(sdl.BLENDMODE_BLEND); err != nil {
			return fmt.Errorf("setting blend mode: %w", err)
		}
		defer func() { _ = r.SetDrawBlendMode(prev) }()
	}
	return r.RenderGeometry(nil, verts, indices)
}
