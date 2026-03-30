// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// Circle is a filled circle with the given radius and color; Position is center-based.
//
// Embeds BaseVisual for position management and lifecycle no-ops.
type Circle struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload
	Radius     float32
	Color      sdl.Color
	Texture    *sdl.Texture
}

// NewCircle creates a new Circle stimulus.
func NewCircle(radius float32, color sdl.Color) *Circle {
	return &Circle{
		Radius: radius,
		Color:  color,
		// BaseVisual.Position defaults to (0, 0)
	}
}

// Draw draws the circle using multiple lines or points.
// For better performance, we could use a texture.
func (c *Circle) Draw(screen *apparatus.Screen) error {
	if err := screen.Renderer.SetDrawColor(c.Color.R, c.Color.G, c.Color.B, c.Color.A); err != nil {
		return err
	}

	cX, cY := screen.CenterToSDL(c.Position.X, c.Position.Y)

	// Draw a filled circle using horizontal lines
	for dy := -c.Radius; dy <= c.Radius; dy++ {
		dx := float32(math.Sqrt(float64(c.Radius*c.Radius - dy*dy)))
		x1, y := cX-dx, cY+dy
		x2 := cX + dx
		screen.Renderer.RenderLine(x1, y, x2, y)
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
