// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/chrplr/goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
)

// TextBox represents a multi-line text box with wrapping and alignment.
type TextBox struct {
	Text      string
	BoxWidth  int32
	Position  sdl.FPoint
	Color     sdl.Color
	Alignment ttf.HorizontalAlignment
	Font      *ttf.Font
	Texture   *sdl.Texture
	Width     float32
	Height    float32
}

// NewTextBox creates a new TextBox stimulus.
func NewTextBox(text string, boxWidth int32, position sdl.FPoint, color sdl.Color) *TextBox {
	return &TextBox{
		Text:      text,
		BoxWidth:  boxWidth,
		Position:  position,
		Color:     color,
		Alignment: ttf.HORIZONTAL_ALIGN_CENTER,
	}
}

// preload prepares the texture from the font.
func (t *TextBox) preload(screen *io.Screen, font *ttf.Font) error {
	if font == nil {
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

func (t *TextBox) Preload() error {
	return nil
}

func (t *TextBox) Draw(screen *io.Screen) error {
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

func (t *TextBox) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := t.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (t *TextBox) Unload() error {
	if t.Texture != nil {
		t.Texture.Destroy()
		t.Texture = nil
	}
	return nil
}

func (t *TextBox) GetPosition() sdl.FPoint {
	return t.Position
}

func (t *TextBox) SetPosition(pos sdl.FPoint) {
	t.Position = pos
}
