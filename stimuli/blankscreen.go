// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// BlankScreen is a full-screen filled rectangle of one color (e.g. for
// inter-trial intervals or clearing between stimuli).
//
// BlankScreen does NOT embed BaseVisual because it has no meaningful position
// (it always covers the entire screen). GetPosition always returns (0,0) and
// SetPosition is a no-op.
//
// Its Present method intentionally ignores the `clear` flag, since drawing the
// blank screen IS effectively a clear. This differs from PresentDrawable, so
// BlankScreen implements Present directly rather than delegating.
type BlankScreen struct {
	Color sdl.Color
}

// NewBlankScreen creates a blank screen that draws the given color over the full screen.
func NewBlankScreen(color sdl.Color) *BlankScreen {
	return &BlankScreen{Color: color}
}

func (b *BlankScreen) Draw(screen *apparatus.Screen) error {
	if err := screen.Renderer.SetDrawColor(b.Color.R, b.Color.G, b.Color.B, b.Color.A); err != nil {
		return err
	}
	// We draw a large rectangle covering the screen area
	w, h, _ := screen.Renderer.RenderOutputSize()
	rect := &sdl.FRect{X: 0, Y: 0, W: float32(w), H: float32(h)}
	return screen.Renderer.RenderFillRect(rect)
}

// Present draws the blank screen. The `clear` flag is intentionally ignored
// because drawing a full-screen fill IS the clear. This is the reason
// BlankScreen does not delegate to PresentDrawable.
func (b *BlankScreen) Present(screen *apparatus.Screen, clear, update bool) error {
	if err := b.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

// Preload is a no-op (BlankScreen has no GPU resources to prepare).
func (b *BlankScreen) Preload() error { return nil }

// Unload is a no-op (BlankScreen has no GPU resources to release).
func (b *BlankScreen) Unload() error { return nil }

// GetPosition always returns (0,0) — BlankScreen has no meaningful position.
func (b *BlankScreen) GetPosition() sdl.FPoint { return sdl.FPoint{X: 0, Y: 0} }

// SetPosition is a no-op — BlankScreen always covers the entire screen.
func (b *BlankScreen) SetPosition(pos sdl.FPoint) {}
