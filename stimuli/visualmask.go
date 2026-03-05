// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"goxpyriment/io"
	"image"
	"image/color"
	"image/draw"
	"math/rand"
	"github.com/Zyko0/go-sdl3/sdl"
)

// VisualMask represents a visual mask stimulus.
type VisualMask struct {
	Size             sdl.FPoint
	Position         sdl.FPoint
	DotSize          sdl.FPoint
	BackgroundColor  sdl.Color
	DotColor         sdl.Color
	DotPercentage    int
	Texture          *sdl.Texture
}

// NewVisualMask creates a new VisualMask stimulus.
func NewVisualMask(width, height float32, dotWidth, dotHeight float32, bgColor, dotColor sdl.Color, dotPercentage int) *VisualMask {
	return &VisualMask{
		Size:            sdl.FPoint{X: width, Y: height},
		Position:        sdl.FPoint{X: 0, Y: 0},
		DotSize:         sdl.FPoint{X: dotWidth, Y: dotHeight},
		BackgroundColor: bgColor,
		DotColor:        dotColor,
		DotPercentage:   dotPercentage,
	}
}

// preload generates the mask and creates the SDL texture.
func (vm *VisualMask) preload(screen *io.Screen) error {
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

func (vm *VisualMask) Preload() error {
	return nil
}

func (vm *VisualMask) Draw(screen *io.Screen) error {
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

func (vm *VisualMask) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := vm.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (vm *VisualMask) Unload() error {
	if vm.Texture != nil {
		vm.Texture.Destroy()
		vm.Texture = nil
	}
	return nil
}

func (vm *VisualMask) GetPosition() sdl.FPoint {
	return vm.Position
}

func (vm *VisualMask) SetPosition(pos sdl.FPoint) {
	vm.Position = pos
}
