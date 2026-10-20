// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

import (
	"fmt"
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// PolyLine is a stroked poly-line: a sequence of points connected by segments of
// LineWidth pixels, optionally closed into an outline.
//
// It is the thick-stroke counterpart of Line (which always renders one pixel
// wide and ignores its LineWidth field) and the unfilled counterpart of Shape.
// Use it for outlined polygons — triangles, open angles, arbitrary contours.
//
// Points are relative to the stimulus centre in the center-based convention with
// **+Y pointing UP**, exactly like Shape.Points. SetPosition moves the whole
// figure; the points themselves are not rewritten.
//
// The stroke is rendered as one RenderGeometry call: a quad per segment, a
// wedge on the outer side of every join and a half-disc at each open end,
// which gives round joins and round caps. Round joins matter for sharp
// angles — a mitre would shoot a long spike out of a 20° vertex. The wedge
// covers exactly what a full disc at the vertex would add to the two quads,
// without overlapping their antialiased fringes (overlapping fringes would
// draw a heavier edge at every join).
//
// The edges are antialiased (see feather.go) unless Aliased is set. With a
// translucent Color the quads still overlap on the inner side of each join,
// where the stroke shows darker.
//
// The vertex and index buffers are retained between Draw calls and only grow, so
// redrawing a PolyLine every frame allocates nothing after the first call. That
// makes it safe inside a GC-disabled, VSYNC-locked presentation loop.
type PolyLine struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload

	Points    []sdl.FPoint // vertices relative to the centre, +Y up
	Closed    bool         // connect the last point back to the first
	LineWidth float32      // stroke width in pixels
	Color     sdl.Color
	Aliased   bool // true = no edge antialiasing

	verts   []sdl.Vertex // reused across Draw calls
	indices []int32
	pts     []sdl.FPoint // SDL-space points without consecutive duplicates
}

// NewPolyLine creates a stroked poly-line through points (relative to the
// stimulus centre). Set closed to connect the last point back to the first,
// which turns the poly-line into a polygon outline.
func NewPolyLine(points []sdl.FPoint, closed bool, lineWidth float32, color sdl.Color) *PolyLine {
	return &PolyLine{
		Points:    points,
		Closed:    closed,
		LineWidth: lineWidth,
		Color:     color,
		// BaseVisual.Position defaults to (0, 0)
	}
}

func (p *PolyLine) Draw(screen *apparatus.Screen) error {
	if len(p.Points) < 2 || p.LineWidth <= 0 {
		return nil // nothing to stroke
	}

	// Centre of the figure in SDL top-left pixel space. Note the "- pt.Y" when
	// placing each point: Points use the +Y-up convention, SDL is Y-down.
	cX, cY := screen.CenterToSDL(p.Position.X, p.Position.Y)
	pts := p.pts[:0]
	for _, pt := range p.Points {
		q := sdl.FPoint{X: cX + pt.X, Y: cY - pt.Y}
		if len(pts) == 0 || q != pts[len(pts)-1] {
			pts = append(pts, q) // coincident points would give no direction
		}
	}
	if p.Closed && len(pts) > 1 && pts[0] == pts[len(pts)-1] {
		pts = pts[:len(pts)-1]
	}
	p.pts = pts

	p.verts, p.indices = appendStroke(p.verts[:0], p.indices[:0], pts, p.Closed, p.LineWidth/2, toFColor(p.Color), p.Aliased)
	if err := drawGeometry(screen, p.verts, p.indices); err != nil {
		return fmt.Errorf("stimuli.PolyLine.Draw: %w", err)
	}
	return nil
}

// appendStroke appends the stroke of half-width hw through pts (SDL space,
// no two consecutive points equal). A single point gives a dot.
func appendStroke(verts []sdl.Vertex, indices []int32, pts []sdl.FPoint, closed bool, hw float32, fc sdl.FColor, aliased bool) ([]sdl.Vertex, []int32) {
	n := len(pts)
	if n == 1 {
		return appendDisc(verts, indices, pts[0].X, pts[0].Y, hw, fc, aliased)
	}
	nSeg := n - 1
	if closed {
		nSeg = n
	}
	// dir returns the unit direction of segment i, from pts[i] to pts[i+1].
	dir := func(i int) (float32, float32) {
		a, b := pts[i%n], pts[(i+1)%n]
		dx, dy := b.X-a.X, b.Y-a.Y
		l := float32(math.Hypot(float64(dx), float64(dy)))
		return dx / l, dy / l
	}

	for i := 0; i < nSeg; i++ {
		a, b := pts[i], pts[(i+1)%n]
		dx, dy := dir(i)
		verts, indices = appendStrokeQuad(verts, indices, a.X, a.Y, b.X, b.Y, -dy, dx, hw, fc, aliased)
	}

	// The normal of a segment with direction (dx, dy) is (-dy, dx), i.e. the
	// direction rotated by +90°, so rotating the normal by -π passes through
	// the direction: that is the cap after the last point, and +π the cap
	// before the first.
	for i := 0; i < n; i++ {
		var in, out int // segments entering and leaving pts[i]
		switch {
		case closed:
			in, out = (i+n-1)%n, i
		case i == 0:
			dx, dy := dir(0)
			verts, indices = appendWedge(verts, indices, pts[0].X, pts[0].Y, hw, math.Atan2(float64(dx), float64(-dy)), math.Pi, fc, aliased)
			continue
		case i == n-1:
			dx, dy := dir(n - 2)
			verts, indices = appendWedge(verts, indices, pts[i].X, pts[i].Y, hw, math.Atan2(float64(dx), float64(-dy)), -math.Pi, fc, aliased)
			continue
		default:
			in, out = i-1, i
		}
		d1x, d1y := dir(in)
		d2x, d2y := dir(out)
		cross := float64(d1x*d2y - d1y*d2x)
		dot := float64(d1x*d2x + d1y*d2y)
		n1x, n1y := float64(-d1y), float64(d1x)
		var a0, sweep float64
		switch {
		case cross == 0 && dot > 0:
			continue // straight on: the quads already meet
		case cross == 0:
			// Full reversal: half a disc around the end of the incoming segment.
			a0, sweep = math.Atan2(n1y, n1x), -math.Pi
		default:
			// The outgoing segment turns towards +n1 when cross > 0, so the
			// gap between the quads is on the -n side, and vice versa.
			s := 1.0
			if cross > 0 {
				s = -1
			}
			a0 = math.Atan2(s*n1y, s*n1x)
			sweep = math.Atan2(cross, dot) // signed turn angle, in (-π, π)
		}
		verts, indices = appendWedge(verts, indices, pts[i].X, pts[i].Y, hw, a0, sweep, fc, aliased)
	}
	return verts, indices
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (p *PolyLine) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(p, screen, clear, update)
}

// Preload, Unload, GetPosition, SetPosition are all provided by BaseVisual.
