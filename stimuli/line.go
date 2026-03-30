// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// Line is a line segment from Start to End in center-based coordinates, with
// the given color and line width.
//
// Unlike most visual stimuli, Line does NOT embed BaseVisual because its
// position is defined by two endpoints rather than a single center point.
// GetPosition returns the midpoint of the segment; SetPosition translates
// both endpoints so that the midpoint moves to the new location.
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

func (l *Line) Draw(screen *apparatus.Screen) error {
	if err := screen.Renderer.SetDrawColor(l.Color.R, l.Color.G, l.Color.B, l.Color.A); err != nil {
		return err
	}

	x1, y1 := screen.CenterToSDL(l.Start.X, l.Start.Y)
	x2, y2 := screen.CenterToSDL(l.End.X, l.End.Y)

	// For simplicity, just use RenderLine.
	// For thicker lines, we'd need to draw a rotated rectangle.
	return screen.Renderer.RenderLine(x1, y1, x2, y2)
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (l *Line) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(l, screen, clear, update)
}

// GetPosition returns the midpoint of the line segment.
// This convention lets the line participate in the VisualStimulus interface
// even though it is defined by two endpoints rather than a single center.
func (l *Line) GetPosition() sdl.FPoint {
	return sdl.FPoint{X: (l.Start.X + l.End.X) / 2, Y: (l.Start.Y + l.End.Y) / 2}
}

// SetPosition translates both endpoints so the midpoint moves to pos.
func (l *Line) SetPosition(pos sdl.FPoint) {
	current := l.GetPosition()
	dx := pos.X - current.X
	dy := pos.Y - current.Y
	l.Start.X += dx
	l.Start.Y += dy
	l.End.X += dx
	l.End.Y += dy
}

// Preload is a no-op (Line has no GPU resources to prepare).
func (l *Line) Preload() error { return nil }

// Unload is a no-op (Line has no GPU resources to release).
func (l *Line) Unload() error { return nil }
