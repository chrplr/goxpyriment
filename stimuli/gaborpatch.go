// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"image"
	"image/color"
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// GaborPatch is a Gabor patch (sinusoidal grating windowed by a Gaussian) with orientation, spatial frequency, phase, and size parameters.
//
// Embeds BaseVisual for position management. Overrides Unload to destroy the
// GPU texture; Preload is a no-op (lazy-loaded on first Draw via preload).
type GaborPatch struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload (Unload overridden below)
	Sigma      float64
	Theta      float64 // orientation in degrees
	Lambda     float64 // spatial wavelength in pixels (cycles per pixel = 1/Lambda)
	Phase      float64
	Psi        float64
	Gamma      float64
	// Contrast is the Michelson contrast of the grating [0, 1].
	// Zero defaults to full contrast (1.0) for backwards compatibility.
	Contrast        float64
	BackgroundColor sdl.Color
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
		// BaseVisual.Position defaults to (0, 0)
		Size: size,
	}
}

// preload generates the Gabor patch texture.
func (gp *GaborPatch) preload(screen *apparatus.Screen) error {
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

			// Apply Michelson contrast (defaults to 1.0 when unset).
			c := gp.Contrast
			if c == 0 {
				c = 1.0
			}
			// Map [-c, c] → [0, 255]; midpoint 127.5 is neutral gray.
			val := c * envelope * grating
			cVal := uint8((val + 1) * 127.5)
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
	if err := texture.SetBlendMode(sdl.BLENDMODE_BLEND); err != nil {
		texture.Destroy()
		return err
	}
	gp.Texture = texture
	return nil
}

// Preload is provided by BaseVisual (no-op; texture is lazy-loaded on first Draw).

func (gp *GaborPatch) Draw(screen *apparatus.Screen) error {
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

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (gp *GaborPatch) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(gp, screen, clear, update)
}

// GetPosition, SetPosition are provided by BaseVisual.

// Unload overrides BaseVisual.Unload to destroy the GPU texture.
func (gp *GaborPatch) Unload() error {
	if gp.Texture != nil {
		gp.Texture.Destroy()
		gp.Texture = nil
	}
	return nil
}
