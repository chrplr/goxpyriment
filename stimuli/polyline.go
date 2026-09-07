// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

import (
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// joinSegments is the number of triangles in the disc placed at every vertex to
// round the joins and the end caps. Twelve is smooth enough at the stroke widths
// used for stimuli (a few pixels) and keeps the geometry small.
const joinSegments = 12

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
// The stroke is rendered as one RenderGeometry call: a quad per segment plus a
// disc per vertex, which gives round joins and round caps. Round joins matter
// for sharp angles — a mitre would shoot a long spike out of a 20° vertex.
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

	verts   []sdl.Vertex // reused across Draw calls
	indices []int32
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
	n := len(p.Points)
	if n < 2 || p.LineWidth <= 0 {
		return nil // nothing to stroke
	}

	// Centre of the figure in SDL top-left pixel space. Note the "- pt.Y" when
	// placing each point: Points use the +Y-up convention, SDL is Y-down.
	cX, cY := screen.CenterToSDL(p.Position.X, p.Position.Y)

	nSeg := n - 1
	if p.Closed {
		nSeg = n
	}
	hw := p.LineWidth / 2
	fc := sdl.FColor{
		R: float32(p.Color.R) / 255,
		G: float32(p.Color.G) / 255,
		B: float32(p.Color.B) / 255,
		A: float32(p.Color.A) / 255,
	}

	// Grow the retained buffers if needed, then reslice to the exact size.
	nVerts := nSeg*4 + n*(1+joinSegments)
	nIdx := nSeg*6 + n*joinSegments*3
	if cap(p.verts) < nVerts {
		p.verts = make([]sdl.Vertex, nVerts)
	}
	if cap(p.indices) < nIdx {
		p.indices = make([]int32, nIdx)
	}
	verts := p.verts[:0]
	indices := p.indices[:0]

	addVertex := func(x, y float32) int32 {
		verts = append(verts, sdl.Vertex{Position: sdl.FPoint{X: x, Y: y}, Color: fc})
		return int32(len(verts) - 1)
	}

	// One quad (two triangles) per segment, offset half the stroke width along
	// the segment normal on either side.
	for i := 0; i < nSeg; i++ {
		a := p.Points[i]
		b := p.Points[(i+1)%n]
		x1, y1 := cX+a.X, cY-a.Y
		x2, y2 := cX+b.X, cY-b.Y
		dx, dy := x2-x1, y2-y1
		length := float32(math.Hypot(float64(dx), float64(dy)))
		if length == 0 {
			continue // coincident points: the vertex disc covers it
		}
		nx, ny := -dy/length*hw, dx/length*hw

		i0 := addVertex(x1+nx, y1+ny)
		i1 := addVertex(x2+nx, y2+ny)
		i2 := addVertex(x2-nx, y2-ny)
		i3 := addVertex(x1-nx, y1-ny)
		indices = append(indices, i0, i1, i2, i0, i2, i3)
	}

	// A disc at every vertex: round joins on the interior points, round caps on
	// the two ends of an open poly-line.
	for _, pt := range p.Points {
		x, y := cX+pt.X, cY-pt.Y
		centre := addVertex(x, y)
		first := int32(len(verts))
		for k := 0; k < joinSegments; k++ {
			a := 2 * math.Pi * float64(k) / joinSegments
			addVertex(x+hw*float32(math.Cos(a)), y+hw*float32(math.Sin(a)))
		}
		for k := 0; k < joinSegments; k++ {
			indices = append(indices,
				centre,
				first+int32(k),
				first+int32((k+1)%joinSegments))
		}
	}

	// Keep the grown buffers for the next Draw.
	p.verts = verts[:cap(verts)]
	p.indices = indices[:cap(indices)]

	return screen.Renderer.RenderGeometry(nil, verts, indices)
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (p *PolyLine) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(p, screen, clear, update)
}

// Preload, Unload, GetPosition, SetPosition are all provided by BaseVisual.
