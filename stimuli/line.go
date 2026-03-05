// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
)

// Line represents a line stimulus.
type Line struct {
	Start     sdl.FPoint
	End       sdl.FPoint
	Color     sdl.Color
	LineWidth float32
}

// NewLine creates a new Line stimulus relative to center.
func NewLine(start, end sdl.FPoint, color sdl.Color, lineWidth float32) *Line {
	return &Line{
		Start:     start,
		End:       end,
		Color:     color,
		LineWidth: lineWidth,
	}
}

func (l *Line) Draw(screen *io.Screen) error {
	if err := screen.Renderer.SetDrawColor(l.Color.R, l.Color.G, l.Color.B, l.Color.A); err != nil {
		return err
	}
	
	x1, y1 := screen.CenterToSDL(l.Start.X, l.Start.Y)
	x2, y2 := screen.CenterToSDL(l.End.X, l.End.Y)
	
	// For simplicity, just use RenderLine. 
	// For thicker lines, we'd need to draw a rotated rectangle.
	return screen.Renderer.RenderLine(x1, y1, x2, y2)
}

func (l *Line) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := l.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (l *Line) GetPosition() sdl.FPoint {
	// Position of a line is defined as its center
	return sdl.FPoint{X: (l.Start.X + l.End.X) / 2, Y: (l.Start.Y + l.End.Y) / 2}
}

func (l *Line) SetPosition(pos sdl.FPoint) {
	current := l.GetPosition()
	dx := pos.X - current.X
	dy := pos.Y - current.Y
	l.Start.X += dx
	l.Start.Y += dy
	l.End.X += dx
	l.End.Y += dy
}

func (l *Line) Preload() error { return nil }
func (l *Line) Unload() error  { return nil }
