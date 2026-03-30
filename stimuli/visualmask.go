// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"image"
	"image/color"
	"image/draw"
	"math/rand"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// VisualMask is a pattern mask of random dots over a background (e.g. for masking stimuli); DotPercentage controls fill (0–100).
//
// Embeds BaseVisual for position management. Overrides Unload to destroy the
// GPU texture; Preload is a no-op (lazy-loaded on first Draw via preload).
type VisualMask struct {
	BaseVisual      // Position, GetPosition, SetPosition, Preload, Unload (Unload overridden below)
	Size            sdl.FPoint
	DotSize         sdl.FPoint
	BackgroundColor sdl.Color
	DotColor        sdl.Color
	DotPercentage   int
	Texture         *sdl.Texture
}

// NewVisualMask creates a mask of the given size, dot size, background and dot colors, and dot percentage (0–100).
func NewVisualMask(width, height float32, dotWidth, dotHeight float32, bgColor, dotColor sdl.Color, dotPercentage int) *VisualMask {
	return &VisualMask{
		Size: sdl.FPoint{X: width, Y: height},
		// BaseVisual.Position defaults to (0, 0)
		DotSize:         sdl.FPoint{X: dotWidth, Y: dotHeight},
		BackgroundColor: bgColor,
		DotColor:        dotColor,
		DotPercentage:   dotPercentage,
	}
}

// preload generates the mask and creates the SDL texture.
func (vm *VisualMask) preload(screen *apparatus.Screen) error {
	w, h := int(vm.Size.X), int(vm.Size.Y)
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	bg := color.RGBA{vm.BackgroundColor.R, vm.BackgroundColor.G, vm.BackgroundColor.B, vm.BackgroundColor.A}
	dot := color.RGBA{vm.DotColor.R, vm.DotColor.G, vm.DotColor.B, vm.DotColor.A}

	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	dw, dh := int(vm.DotSize.X), int(vm.DotSize.Y)
	numDots := (w * h * vm.DotPercentage) / (100 * dw * dh)

	for i := 0; i < numDots; i++ {
		x := rand.Intn(w - dw + 1)
		y := rand.Intn(h - dh + 1)
		draw.Draw(img, image.Rect(x, y, x+dw, y+dh), &image.Uniform{dot}, image.Point{}, draw.Src)
	}

	// Convert image.RGBA to sdl.Surface
	surface, err := sdl.CreateSurfaceFrom(w, h, sdl.PIXELFORMAT_RGBA32, img.Pix, w*4)
	if err != nil {
		return err
	}
	defer surface.Destroy()

	texture, err := screen.Renderer.CreateTextureFromSurface(surface)
	if err != nil {
		return err
	}
	vm.Texture = texture
	return nil
}

// Preload is provided by BaseVisual (no-op; texture is lazy-loaded on first Draw).

func (vm *VisualMask) Draw(screen *apparatus.Screen) error {
	if vm.Texture == nil {
		if err := vm.preload(screen); err != nil {
			return err
		}
	}

	destX, destY := screen.CenterToSDL(vm.Position.X, vm.Position.Y)
	destRect := &sdl.FRect{
		X: destX - vm.Size.X/2,
		Y: destY - vm.Size.Y/2,
		W: vm.Size.X,
		H: vm.Size.Y,
	}

	return screen.Renderer.RenderTexture(vm.Texture, nil, destRect)
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (vm *VisualMask) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(vm, screen, clear, update)
}

// Unload overrides BaseVisual.Unload to destroy the GPU texture.
func (vm *VisualMask) Unload() error {
	if vm.Texture != nil {
		vm.Texture.Destroy()
		vm.Texture = nil
	}
	return nil
}

// GetPosition, SetPosition are provided by BaseVisual.
