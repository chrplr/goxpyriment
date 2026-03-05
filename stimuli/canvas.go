// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
)

// Canvas represents a visual container stimulus that can be drawn upon.
type Canvas struct {
	Size     sdl.FPoint
	Position sdl.FPoint
	Color    sdl.Color
	Texture  *sdl.Texture
}

// NewCanvas creates a new Canvas.
func NewCanvas(width, height float32, color sdl.Color) *Canvas {
	return &Canvas{
		Size:     sdl.FPoint{X: width, Y: height},
		Position: sdl.FPoint{X: 0, Y: 0},
		Color:    color,
	}
}

// preload creates the internal texture for the canvas.
func (c *Canvas) preload(screen *io.Screen) error {
	if c.Texture != nil {
		return nil
	}
	
	tex, err := screen.Renderer.CreateTexture(sdl.PIXELFORMAT_RGBA32, sdl.TEXTUREACCESS_TARGET, int(c.Size.X), int(c.Size.Y))
	if err != nil {
		return err
	}
	c.Texture = tex
	
	// Initial clear
	return c.Clear(screen)
}

func (c *Canvas) Preload() error {
	return nil
}

// Clear clears the canvas with its background color.
func (c *Canvas) Clear(screen *io.Screen) error {
	if c.Texture == nil {
		if err := c.preload(screen); err != nil {
			return err
		}
	}

	prevTarget := screen.Renderer.RenderTarget()
	if err := screen.Renderer.SetRenderTarget(c.Texture); err != nil {
		return err
	}
	defer screen.Renderer.SetRenderTarget(prevTarget)

	if err := screen.Renderer.SetDrawColor(c.Color.R, c.Color.G, c.Color.B, c.Color.A); err != nil {
		return err
	}
	return screen.Renderer.Clear()
}

// Blit draws a stimulus onto the canvas.
func (c *Canvas) Blit(stim VisualStimulus, screen *io.Screen) error {
	if c.Texture == nil {
		if err := c.preload(screen); err != nil {
			return err
		}
	}

	prevTarget := screen.Renderer.RenderTarget()
	if err := screen.Renderer.SetRenderTarget(c.Texture); err != nil {
		return err
	}
	
	// Set the screen offset to the center of this canvas
	oldOffset := screen.CanvasOffset
	screen.CanvasOffset = &sdl.FPoint{X: c.Size.X / 2, Y: c.Size.Y / 2}
	
	// Ensure we restore both the render target AND the offset
	defer func() {
		screen.Renderer.SetRenderTarget(prevTarget)
		screen.CanvasOffset = oldOffset
	}()

	// Draw the stimulus onto the canvas texture
	return stim.Draw(screen)
}


func (c *Canvas) Draw(screen *io.Screen) error {
	if c.Texture == nil {
		if err := c.preload(screen); err != nil {
			return err
		}
	}
	
	destX, destY := screen.CenterToSDL(c.Position.X, c.Position.Y)
	destRect := &sdl.FRect{
		X: destX - c.Size.X/2,
		Y: destY - c.Size.Y/2,
		W: c.Size.X,
		H: c.Size.Y,
	}
	
	return screen.Renderer.RenderTexture(c.Texture, nil, destRect)
}

func (c *Canvas) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := c.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (c *Canvas) GetPosition() sdl.FPoint {
	return c.Position
}

func (c *Canvas) SetPosition(pos sdl.FPoint) {
	c.Position = pos
}

func (c *Canvas) Unload() error {
	if c.Texture != nil {
		c.Texture.Destroy()
		c.Texture = nil
	}
	return nil
}
