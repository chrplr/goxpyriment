// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/Zyko0/go-sdl3/img"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// Picture is an image stimulus loaded from a file path or from memory (e.g. embedded data).
//
// Embeds BaseVisual for position management. Overrides Unload to destroy the
// GPU texture; Preload is a no-op (lazy-loaded on first Draw via preload).
type Picture struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload (Unload overridden below)
	FilePath   string
	Memory     []byte
	Texture    *sdl.Texture
	Width      float32
	Height     float32
}

// NewPicture creates a picture stimulus from a file path, with center position (x, y).
func NewPicture(filePath string, x, y float32) *Picture {
	return &Picture{
		BaseVisual: BaseVisual{Position: sdl.FPoint{X: x, Y: y}},
		FilePath:   filePath,
	}
}

// NewPictureFromMemory creates a new Picture stimulus from embedded data.
func NewPictureFromMemory(data []byte, x, y float32) *Picture {
	return &Picture{
		BaseVisual: BaseVisual{Position: sdl.FPoint{X: x, Y: y}},
		Memory:     data,
	}
}

// preload prepares the texture from the file or memory.
func (p *Picture) preload(screen *apparatus.Screen) error {
	var surface *sdl.Surface
	var err error

	if p.Memory != nil {
		ioStream, err := sdl.IOFromBytes(p.Memory)
		if err != nil {
			return err
		}
		defer ioStream.Close()
		surface, err = img.LoadIO(ioStream, false)
		if err != nil {
			return err
		}
	} else {
		surface, err = img.Load(p.FilePath)
		if err != nil {
			return err
		}
	}
	defer surface.Destroy()

	if p.Width == 0 {
		p.Width = float32(surface.W)
	}
	if p.Height == 0 {
		p.Height = float32(surface.H)
	}

	texture, err := screen.Renderer.CreateTextureFromSurface(surface)
	if err != nil {
		return err
	}
	p.Texture = texture
	return nil
}

// Preload is provided by BaseVisual (no-op; texture is lazy-loaded on first Draw).

func (p *Picture) Draw(screen *apparatus.Screen) error {
	if p.Texture == nil {
		if err := p.preload(screen); err != nil {
			return err
		}
	}

	destX, destY := screen.CenterToSDL(p.Position.X, p.Position.Y)
	// Centering the image at the target position
	destRect := &sdl.FRect{
		X: destX - p.Width/2,
		Y: destY - p.Height/2,
		W: p.Width,
		H: p.Height,
	}

	return screen.Renderer.RenderTexture(p.Texture, nil, destRect)
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (p *Picture) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(p, screen, clear, update)
}

// Unload overrides BaseVisual.Unload to destroy the GPU texture.
func (p *Picture) Unload() error {
	if p.Texture != nil {
		p.Texture.Destroy()
		p.Texture = nil
	}
	return nil
}

// GetPosition, SetPosition are provided by BaseVisual.
