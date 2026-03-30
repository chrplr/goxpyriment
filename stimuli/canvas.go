// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// Canvas is an offscreen render target of the given size and background color; stimuli can be drawn to it, then the canvas is drawn to the screen.
//
// Embeds BaseVisual for position management. Overrides Unload to destroy the
// GPU texture; Preload is a no-op (lazy-loaded on first Draw/Blit via preload).
type Canvas struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload (Unload overridden below)
	Size       sdl.FPoint
	Color      sdl.Color
	Texture    *sdl.Texture
}

// NewCanvas creates a canvas with the given width, height, and background color (center at 0,0).
func NewCanvas(width, height float32, color sdl.Color) *Canvas {
	return &Canvas{
		Size: sdl.FPoint{X: width, Y: height},
		// BaseVisual.Position defaults to (0, 0)
		Color: color,
	}
}

// preload creates the internal texture for the canvas.
func (c *Canvas) preload(screen *apparatus.Screen) error {
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

// Preload is provided by BaseVisual (no-op; texture is lazy-loaded on first Draw/Blit).

// Clear clears the canvas with its background color.
func (c *Canvas) Clear(screen *apparatus.Screen) error {
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
func (c *Canvas) Blit(stim VisualStimulus, screen *apparatus.Screen) error {
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

func (c *Canvas) Draw(screen *apparatus.Screen) error {
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

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (c *Canvas) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(c, screen, clear, update)
}

// GetPosition, SetPosition are provided by BaseVisual.

// Unload overrides BaseVisual.Unload to destroy the GPU texture.
func (c *Canvas) Unload() error {
	if c.Texture != nil {
		c.Texture.Destroy()
		c.Texture = nil
	}
	return nil
}
