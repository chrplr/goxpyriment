package stimuli

import (
	"goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
)

// TextLine represents a line of text.
type TextLine struct {
	Text     string
	Position sdl.FPoint
	Color    sdl.Color
	Font     *ttf.Font
	Texture  *sdl.Texture
	Width    float32
	Height   float32
}

func NewTextLine(text string, x, y float32, color sdl.Color) *TextLine {
	return &TextLine{
		Text:     text,
		Position: sdl.FPoint{X: x, Y: y},
		Color:    color,
	}
}

// preload prepares the texture from the font.
func (t *TextLine) preload(screen *io.Screen, font *ttf.Font) error {
	if font == nil {
		return nil // Fallback to DebugText if no font provided
	}
	
	surface, err := font.RenderTextBlended(t.Text, t.Color)
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

func (t *TextLine) Preload() error {
	return nil
}

func (t *TextLine) Draw(screen *io.Screen) error {
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
	// Note: DebugText doesn't support centering easily, just using Position
	return screen.Renderer.DebugText(t.Position.X, t.Position.Y, t.Text)
}

func (t *TextLine) Present(screen *io.Screen, clear, update bool) error {
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

func (t *TextLine) Unload() error {
	if t.Texture != nil {
		t.Texture.Destroy()
		t.Texture = nil
	}
	return nil
}

func (t *TextLine) GetPosition() sdl.FPoint {
	return t.Position
}

func (t *TextLine) SetPosition(pos sdl.FPoint) {
	t.Position = pos
}

