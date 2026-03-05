// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/chrplr/goxpyriment/io"
	"math"
	"github.com/Zyko0/go-sdl3/sdl"
)

// Circle represents a circle stimulus.
type Circle struct {
	Radius   float32
	Position sdl.FPoint
	Color    sdl.Color
	Texture  *sdl.Texture
}

// NewCircle creates a new Circle stimulus.
func NewCircle(radius float32, color sdl.Color) *Circle {
	return &Circle{
		Radius: radius,
		Color:  color,
		Position: sdl.FPoint{X: 0, Y: 0},
	}
}

// Draw draws the circle using multiple lines or points.
// For better performance, we could use a texture.
func (c *Circle) Draw(screen *io.Screen) error {
	if err := screen.Renderer.SetDrawColor(c.Color.R, c.Color.G, c.Color.B, c.Color.A); err != nil {
		return err
	}
	
	cX, cY := screen.CenterToSDL(c.Position.X, c.Position.Y)
	
	// Draw a filled circle using horizontal lines
	for dy := -c.Radius; dy <= c.Radius; dy++ {
		dx := float32(math.Sqrt(float64(c.Radius*c.Radius - dy*dy)))
		x1, y := cX-dx, cY+dy
		x2 := cX+dx
		screen.Renderer.RenderLine(x1, y, x2, y)
	}
	
	return nil
}

func (c *Circle) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := c.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (c *Circle) Preload() error { return nil }
func (c *Circle) Unload() error  { return nil }

func (c *Circle) GetPosition() sdl.FPoint {
	return c.Position
}

func (c *Circle) SetPosition(pos sdl.FPoint) {
	c.Position = pos
}

// InsideCircle checks if the circle is inside another circle (area).
func (c *Circle) InsideCircle(areaRadius float32, areaPos sdl.FPoint) bool {
	dx := c.Position.X - areaPos.X
	dy := c.Position.Y - areaPos.Y
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	return dist+c.Radius <= areaRadius
}
