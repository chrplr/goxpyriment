// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// FixCross is a centered fixation cross (horizontal and vertical lines).
// Position is in center-based coordinates; Size and LineWidth are in the same units.
//
// Embeds BaseVisual for position management and lifecycle no-ops.
type FixCross struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload
	Size       float32
	LineWidth  float32
	Color      sdl.Color
}

// NewFixCross creates a fixation cross with the given size, line width, and color (center at 0,0).
func NewFixCross(size float32, lineWidth float32, color sdl.Color) *FixCross {
	return &FixCross{
		Size:      size,
		LineWidth: lineWidth,
		Color:     color,
		// BaseVisual.Position defaults to (0, 0)
	}
}

func (f *FixCross) Draw(screen *apparatus.Screen) error {
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

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (f *FixCross) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(f, screen, clear, update)
}

// Preload, Unload, GetPosition, SetPosition are all provided by BaseVisual.
