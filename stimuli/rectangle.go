// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
)

// Rectangle represents a rectangle stimulus.
type Rectangle struct {
	Position sdl.FPoint
	Rect     sdl.FRect
	Color    sdl.Color
}

func NewRectangle(x, y, w, h float32, color sdl.Color) *Rectangle {
	return &Rectangle{
		Position: sdl.FPoint{X: x, Y: y},
		Rect:     sdl.FRect{W: w, H: h},
		Color:    color,
	}
}

func (r *Rectangle) Draw(screen *io.Screen) error {
	if err := screen.Renderer.SetDrawColor(r.Color.R, r.Color.G, r.Color.B, r.Color.A); err != nil {
		return err
	}
	
	destX, destY := screen.CenterToSDL(r.Position.X, r.Position.Y)
	// Centering the rectangle at the target position
	destRect := &sdl.FRect{
		X: destX - r.Rect.W/2,
		Y: destY - r.Rect.H/2,
		W: r.Rect.W,
		H: r.Rect.H,
	}
	
	return screen.Renderer.RenderFillRect(destRect)
}

func (r *Rectangle) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := r.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (r *Rectangle) Preload() error { return nil }
func (r *Rectangle) Unload() error  { return nil }

func (r *Rectangle) GetPosition() sdl.FPoint {
	return r.Position
}

func (r *Rectangle) SetPosition(pos sdl.FPoint) {
	r.Position = pos
}
