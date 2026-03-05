// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
)

// BlankScreen represents an empty visual stimulus.
type BlankScreen struct {
	Color sdl.Color
}

// NewBlankScreen creates a new BlankScreen stimulus.
func NewBlankScreen(color sdl.Color) *BlankScreen {
	return &BlankScreen{Color: color}
}

func (b *BlankScreen) Draw(screen *io.Screen) error {
	if err := screen.Renderer.SetDrawColor(b.Color.R, b.Color.G, b.Color.B, b.Color.A); err != nil {
		return err
	}
	// We draw a large rectangle covering the screen area
	w, h, _ := screen.Renderer.RenderOutputSize()
	rect := &sdl.FRect{X: 0, Y: 0, W: float32(w), H: float32(h)}
	return screen.Renderer.RenderFillRect(rect)
}

func (b *BlankScreen) Present(screen *io.Screen, clear, update bool) error {
	// BlankScreen ignore clear since it is a clear itself
	if err := b.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (b *BlankScreen) Preload() error { return nil }
func (b *BlankScreen) Unload() error  { return nil }

func (b *BlankScreen) GetPosition() sdl.FPoint { return sdl.FPoint{X: 0, Y: 0} }
func (b *BlankScreen) SetPosition(pos sdl.FPoint) {}
