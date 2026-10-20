// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

import (
	"math"

	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// Circle is a filled circle with the given radius and color; Position is center-based.
//
// The edge is antialiased (see feather.go) unless Aliased is set, which gives
// hard pixel edges with no intermediate grey levels.
//
// Embeds BaseVisual for position management and lifecycle no-ops.
type Circle struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload
	Radius     float32
	Color      sdl.Color
	Aliased    bool // true = no edge antialiasing

	verts   []sdl.Vertex // reused across Draw calls
	indices []int32
}

// NewCircle creates a new Circle stimulus.
func NewCircle(radius float32, color sdl.Color) *Circle {
	return &Circle{
		Radius: radius,
		Color:  color,
		// BaseVisual.Position defaults to (0, 0)
	}
}

// Draw draws the circle as one RenderGeometry triangle fan. The vertex
// buffers are kept between calls, so redrawing allocates nothing.
func (c *Circle) Draw(screen *apparatus.Screen) error {
	cX, cY := screen.CenterToSDL(c.Position.X, c.Position.Y)
	c.verts, c.indices = appendDisc(c.verts[:0], c.indices[:0], cX, cY, c.Radius, toFColor(c.Color), c.Aliased)
	if err := drawGeometry(screen, c.verts, c.indices); err != nil {
		return fmt.Errorf("stimuli.Circle.Draw: %w", err)
	}
	return nil
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (c *Circle) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(c, screen, clear, update)
}

// Preload, Unload, GetPosition, SetPosition are all provided by BaseVisual.

// InsideCircle checks if the circle is inside another circle (area).
func (c *Circle) InsideCircle(areaRadius float32, areaPos sdl.FPoint) bool {
	dx := c.Position.X - areaPos.X
	dy := c.Position.Y - areaPos.Y
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	return dist+c.Radius <= areaRadius
}
