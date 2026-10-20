// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// Shape is a filled polygon defined by Points (relative to the shape center,
// +Y up) and a single color.
//
// The polygon is triangulated as a fan from Points[0], so it must be convex,
// or at least star-shaped from that point. The edges are antialiased (see
// feather.go) unless Aliased is set, which gives hard pixel edges with no
// intermediate grey levels.
//
// Embeds BaseVisual for position management and lifecycle no-ops.
type Shape struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload
	Points     []sdl.FPoint
	Color      sdl.Color
	Aliased    bool // true = no edge antialiasing

	verts   []sdl.Vertex // reused across Draw calls
	indices []int32
}

// NewShape creates a new polygon shape. Points are relative to the shape's center.
func NewShape(points []sdl.FPoint, color sdl.Color) *Shape {
	return &Shape{
		Points: points,
		Color:  color,
		// BaseVisual.Position defaults to (0, 0)
	}
}

// Draw draws the polygon as one RenderGeometry call. The vertex buffers are
// kept between calls, so redrawing allocates nothing.
func (s *Shape) Draw(screen *apparatus.Screen) error {
	cX, cY := screen.CenterToSDL(s.Position.X, s.Position.Y)
	s.verts, s.indices = appendConvexFill(s.verts[:0], s.indices[:0], cX, cY, s.Points, toFColor(s.Color), s.Aliased)
	if err := drawGeometry(screen, s.verts, s.indices); err != nil {
		return fmt.Errorf("stimuli.Shape.Draw: %w", err)
	}
	return nil
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (s *Shape) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(s, screen, clear, update)
}

// Preload, Unload, GetPosition, SetPosition are all provided by BaseVisual.
