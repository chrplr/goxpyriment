// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/chrplr/goxpyriment/io"
	"image"
	"image/color"
	"math"
	"github.com/Zyko0/go-sdl3/sdl"
)

// GaborPatch is a Gabor patch (sinusoidal grating windowed by a Gaussian) with orientation, spatial frequency, phase, and size parameters.
type GaborPatch struct {
	Sigma           float64
	Theta           float64 // orientation in degrees
	Lambda          float64 // spatial frequency
	Phase           float64
	Psi             float64
	Gamma           float64
	BackgroundColor sdl.Color
	Position        sdl.FPoint
	Size            float32
	Texture         *sdl.Texture
}

// NewGaborPatch creates a Gabor patch with the given sigma, theta (degrees), lambda, phase, psi, gamma, background color, and size in pixels.
func NewGaborPatch(sigma, theta, lambda, phase, psi, gamma float64, bgColor sdl.Color, size float32) *GaborPatch {
	return &GaborPatch{
		Sigma:           sigma,
		Theta:           theta,
		Lambda:          lambda,
		Phase:           phase,
		Psi:             psi,
		Gamma:           gamma,
		BackgroundColor: bgColor,
		Position:        sdl.FPoint{X: 0, Y: 0},
		Size:            size,
	}
}

// preload generates the Gabor patch texture.
func (gp *GaborPatch) preload(screen *io.Screen) error {
	w, h := int(gp.Size), int(gp.Size)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	
	thetaRad := gp.Theta * math.Pi / 180.0
	
	halfW := float64(w) / 2
	halfH := float64(h) / 2
	
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Center coordinates
			xf := float64(x) - halfW
			yf := float64(y) - halfH
			
			// Rotation
			x_prime := xf*math.Cos(thetaRad) + yf*math.Sin(thetaRad)
			y_prime := -xf*math.Sin(thetaRad) + yf*math.Cos(thetaRad)
			
			// Gaussian envelope
			envelope := math.Exp(-(x_prime*x_prime + gp.Gamma*gp.Gamma*y_prime*y_prime) / (2 * gp.Sigma * gp.Sigma))
			
			// Sinusoidal grating
			grating := math.Cos(2*math.Pi*(x_prime/gp.Lambda) + gp.Psi + gp.Phase*2*math.Pi)
			
			// Combine and scale to 0-255
			val := envelope * grating
			
			// Map val [-1, 1] to [0, 255]
			cVal := uint8((val + 1) * 127.5)
			
			// Background blending
			// In expyriment, it's often centered around the background color
			// Let's just use the calculated gray value for now
			img.Set(x, y, color.RGBA{R: cVal, G: cVal, B: cVal, A: uint8(envelope * 255)})
		}
	}
	
	surface, err := sdl.CreateSurfaceFrom(w, h, sdl.PIXELFORMAT_RGBA32, img.Pix, w*4)
	if err != nil {
		return err
	}
	defer surface.Destroy()
	
	texture, err := screen.Renderer.CreateTextureFromSurface(surface)
	if err != nil {
		return err
	}
	gp.Texture = texture
	return nil
}

func (gp *GaborPatch) Preload() error {
	return nil
}

func (gp *GaborPatch) Draw(screen *io.Screen) error {
	if gp.Texture == nil {
		if err := gp.preload(screen); err != nil {
			return err
		}
	}
	
	destX, destY := screen.CenterToSDL(gp.Position.X, gp.Position.Y)
	destRect := &sdl.FRect{
		X: destX - gp.Size/2,
		Y: destY - gp.Size/2,
		W: gp.Size,
		H: gp.Size,
	}
	
	return screen.Renderer.RenderTexture(gp.Texture, nil, destRect)
}

func (gp *GaborPatch) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := gp.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (gp *GaborPatch) GetPosition() sdl.FPoint {
	return gp.Position
}

func (gp *GaborPatch) SetPosition(pos sdl.FPoint) {
	gp.Position = pos
}

func (gp *GaborPatch) Unload() error {
	if gp.Texture != nil {
		gp.Texture.Destroy()
		gp.Texture = nil
	}
	return nil
}
