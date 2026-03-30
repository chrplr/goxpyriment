// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
	"github.com/chrplr/goxpyriment/apparatus"
)

// TextBox represents a multi-line text box with wrapping and alignment.
//
// Embeds BaseVisual for position management. Overrides Unload to destroy the
// GPU texture; Preload is a no-op (lazy-loaded on first Draw via preload).
type TextBox struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload (Unload overridden below)
	Text       string
	BoxWidth   int32
	Color      sdl.Color
	Alignment  ttf.HorizontalAlignment
	Font       *ttf.Font
	Texture    *sdl.Texture
	Width      float32
	Height     float32
}

// NewTextBox creates a multi-line text box with the given maximum width (in pixels), center position, and color.
func NewTextBox(text string, boxWidth int32, position sdl.FPoint, color sdl.Color) *TextBox {
	return &TextBox{
		BaseVisual: BaseVisual{Position: position},
		Text:       text,
		BoxWidth:   boxWidth,
		Color:      color,
		Alignment:  ttf.HORIZONTAL_ALIGN_CENTER,
	}
}

// preload prepares the texture from the font.
func (t *TextBox) preload(screen *apparatus.Screen, font *ttf.Font) error {
	if font == nil {
		return nil
	}

	if t.Text == "" {
		t.Width = 0
		t.Height = float32(font.Height())
		t.Texture = nil
		t.Font = font
		return nil
	}

	// Set alignment
	font.SetWrapAlignment(t.Alignment)

	surface, err := font.RenderTextBlendedWrapped(t.Text, t.Color, t.BoxWidth)
	if err != nil {
		return err
	}
	defer surface.Destroy()

	t.Width = float32(surface.W)
	t.Height = float32(surface.H)

	texture, err := screen.Renderer.CreateTextureFromSurface(surface)
	if err != nil {
		return err
	}
	t.Texture = texture
	t.Font = font
	return nil
}

// Preload is provided by BaseVisual (no-op; texture is lazy-loaded on first Draw).

func (t *TextBox) Draw(screen *apparatus.Screen) error {
	f := t.Font
	if f == nil {
		f = screen.DefaultFont
	}

	if f != nil {
		if t.Texture == nil || t.Font != f {
			if err := t.preload(screen, f); err != nil {
				return err
			}
		}

		if t.Texture != nil {
			destX, destY := screen.CenterToSDL(t.Position.X, t.Position.Y)
			destRect := &sdl.FRect{
				X: destX - t.Width/2,
				Y: destY - t.Height/2,
				W: t.Width,
				H: t.Height,
			}
			return screen.Renderer.RenderTexture(t.Texture, nil, destRect)
		}
	}

	// Fallback to DebugText
	if err := screen.Renderer.SetDrawColor(t.Color.R, t.Color.G, t.Color.B, t.Color.A); err != nil {
		return err
	}
	return screen.Renderer.DebugText(t.Position.X, t.Position.Y, t.Text)
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (t *TextBox) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(t, screen, clear, update)
}

// Unload overrides BaseVisual.Unload to destroy the GPU texture.
func (t *TextBox) Unload() error {
	if t.Texture != nil {
		t.Texture.Destroy()
		t.Texture = nil
	}
	return nil
}

// GetPosition, SetPosition are provided by BaseVisual.
