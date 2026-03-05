// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/chrplr/goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
)

// FixCross represents a fixation cross.
type FixCross struct {
	Size      float32
	LineWidth float32
	Color     sdl.Color
	Position  sdl.FPoint
}

func NewFixCross(size float32, lineWidth float32, color sdl.Color) *FixCross {
	return &FixCross{
		Size:      size,
		LineWidth: lineWidth,
		Color:     color,
		Position:  sdl.FPoint{X: 0, Y: 0},
	}
}

func (f *FixCross) Draw(screen *io.Screen) error {
	if err := screen.Renderer.SetDrawColor(f.Color.R, f.Color.G, f.Color.B, f.Color.A); err != nil {
		return err
	}
	
	cX, cY := screen.CenterToSDL(f.Position.X, f.Position.Y)
	halfSize := f.Size / 2
	
	// Horizontal line
	hRect := &sdl.FRect{
		X: cX - halfSize,
		Y: cY - f.LineWidth/2,
		W: f.Size,
		H: f.LineWidth,
	}
	if err := screen.Renderer.RenderFillRect(hRect); err != nil {
		return err
	}
	
	// Vertical line
	vRect := &sdl.FRect{
		X: cX - f.LineWidth/2,
		Y: cY - halfSize,
		W: f.LineWidth,
		H: f.Size,
	}
	return screen.Renderer.RenderFillRect(vRect)
}

func (f *FixCross) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := f.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (f *FixCross) Preload() error { return nil }
func (f *FixCross) Unload() error  { return nil }

func (f *FixCross) GetPosition() sdl.FPoint {
	return f.Position
}

func (f *FixCross) SetPosition(pos sdl.FPoint) {
	f.Position = pos
}
