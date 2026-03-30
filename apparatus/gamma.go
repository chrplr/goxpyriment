// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package apparatus

import (
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
)

// GammaCorrector holds a 256-entry inverse-gamma look-up table (LUT) for each
// RGB channel. It maps "desired linear luminance" (0–255) to the physical
// digital value needed to achieve that luminance on a display with the given
// gamma curve.
//
// Background: standard monitors apply a power-law transfer function
//
//	L(V) = k · (V/255)^γ   (γ ≈ 2.2 for sRGB displays)
//
// This means equal steps in RGB values do *not* produce equal steps in
// physical luminance. GammaCorrector pre-applies the inverse curve so that
// if you pass linear luminance values (0–255) to CorrectColor, the monitor
// will reproduce them with physically uniform spacing.
//
// # Usage
//
//	gc := apparatus.NewGammaCorrectorUniform(2.2)
//	corrected := gc.CorrectColor(sdl.Color{R: 128, G: 128, B: 128, A: 255})
//	// corrected.R ≈ 186 — the physical value needed for 50% luminance on a
//	// γ=2.2 monitor.
//
// # SDL3 note
//
// SDL3 provides COLORSPACE_SRGB_LINEAR (via CreateRendererWithProperties with
// property "SDL_PROP_RENDERER_CREATE_OUTPUT_COLORSPACE_NUMBER") for automatic
// GPU-side sRGB linearization, but this requires OS/driver support and only
// handles the fixed sRGB curve.  The LUT approach here is portable and
// supports custom per-channel calibration from photometer measurements.
type GammaCorrector struct {
	R [256]uint8
	G [256]uint8
	B [256]uint8
}

// buildLUT computes a 256-entry inverse-gamma LUT for a single channel.
func buildLUT(gamma float64) [256]uint8 {
	var lut [256]uint8
	invGamma := 1.0 / gamma
	for v := 0; v < 256; v++ {
		lut[v] = uint8(math.Round(255.0 * math.Pow(float64(v)/255.0, invGamma)))
	}
	return lut
}

// NewGammaCorrectorUniform builds a GammaCorrector assuming the same gamma
// for all three RGB channels. A gamma of 2.2 is typical for sRGB monitors;
// older CRTs often measured between 1.8 and 2.5.
func NewGammaCorrectorUniform(gamma float64) *GammaCorrector {
	lut := buildLUT(gamma)
	return &GammaCorrector{R: lut, G: lut, B: lut}
}

// NewGammaCorrector builds a GammaCorrector with independent per-channel gamma
// values. Use this when photometer measurements reveal channel imbalance
// (e.g. a monitor whose red phosphor has a different gamma than blue/green).
func NewGammaCorrector(gammaR, gammaG, gammaB float64) *GammaCorrector {
	return &GammaCorrector{
		R: buildLUT(gammaR),
		G: buildLUT(gammaG),
		B: buildLUT(gammaB),
	}
}

// CorrectColor applies the inverse-gamma LUT to c and returns the corrected
// color. The alpha channel is passed through unchanged.
//
// Pass linear luminance values (0–255) in the input color; the returned color
// holds the physical digital values to send to the renderer so that the
// monitor reproduces the intended luminance.
func (gc *GammaCorrector) CorrectColor(c sdl.Color) sdl.Color {
	return sdl.Color{
		R: gc.R[c.R],
		G: gc.G[c.G],
		B: gc.B[c.B],
		A: c.A,
	}
}
