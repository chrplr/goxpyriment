// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

// Package units provides unit conversions for vision-science experiments.
//
// The central type is [Monitor], which encodes the physical dimensions of a
// display and the observer's viewing distance. From those three numbers every
// spatial quantity used in psychophysics can be converted between:
//
//   - pixels (px)    — the native unit of SDL and all stimulus constructors
//   - centimetres (cm) — physical size on the screen surface
//   - degrees of visual angle (°) — the standard unit in vision science
//
// # Quick start
//
//	mon := units.NewMonitorFromDiagonal(24, 1920, 1080, 60)
//
//	// Stimulus sizes
//	gaborSizePx  := mon.DegToPx(4.0)   // 4° Gabor patch
//	gaborSigmaPx := mon.DegToPx(0.5)   // 0.5° Gaussian envelope SD
//
//	// Spatial frequency
//	// 5 cycles/degree → cycles/pixel (for grating lambda parameter)
//	lambdaPx := mon.PPD() / 5.0
//
//	// Eccentricity
//	xPx := float32(mon.DegToPx(3.0))   // 3° to the right of centre
//
// # Axis conventions
//
// The horizontal axis (X) uses WidthCm / WidthPx for pixel-density
// conversions. Modern displays have square pixels, so the horizontal and
// vertical densities are equal; [Monitor.HasSquarePixels] can confirm this.
// Explicit Y-axis variants ([Monitor.DegToPxY] etc.) are provided for the
// rare case of non-square pixels.
package units

import (
	"errors"
	"fmt"
	"math"
)

// Monitor describes the physical and digital properties of a display and the
// observer's viewing distance.
//
// All distance and size fields are in centimetres. Resolution fields are in
// pixels. Construct with [NewMonitor] or [NewMonitorFromDiagonal] rather than
// using a struct literal, so that computed fields are set correctly.
type Monitor struct {
	WidthCm    float64 // physical screen width in centimetres
	HeightCm   float64 // physical screen height in centimetres
	WidthPx    int     // horizontal resolution in pixels
	HeightPx   int     // vertical resolution in pixels
	DistanceCm float64 // observer's viewing distance in centimetres
}

// NewMonitor creates a Monitor from explicit physical and pixel dimensions.
//
// Example — a 24″ 1920×1080 display at 60 cm:
//
//	mon := units.NewMonitor(53.1, 29.9, 1920, 1080, 60)
func NewMonitor(widthCm, heightCm float64, widthPx, heightPx int, distanceCm float64) Monitor {
	return Monitor{
		WidthCm:    widthCm,
		HeightCm:   heightCm,
		WidthPx:    widthPx,
		HeightPx:   heightPx,
		DistanceCm: distanceCm,
	}
}

// NewMonitorFromDiagonal derives physical dimensions from the diagonal screen
// size (in inches) and the pixel resolution, assuming a flat rectangular screen.
//
// Example:
//
//	mon := units.NewMonitorFromDiagonal(24, 1920, 1080, 60)
func NewMonitorFromDiagonal(diagonalInches float64, widthPx, heightPx int, distanceCm float64) Monitor {
	diagonalCm := diagonalInches * 2.54
	ratio := float64(widthPx) / float64(heightPx)
	// diagonal² = width² + height² = (ratio·height)² + height²
	heightCm := diagonalCm / math.Sqrt(ratio*ratio+1)
	widthCm := ratio * heightCm
	return Monitor{
		WidthCm:    widthCm,
		HeightCm:   heightCm,
		WidthPx:    widthPx,
		HeightPx:   heightPx,
		DistanceCm: distanceCm,
	}
}

// Validate returns an error if any field is non-positive.
// Useful when constructing a Monitor from user-provided configuration values.
func (m Monitor) Validate() error {
	switch {
	case m.WidthCm <= 0:
		return errors.New("units: WidthCm must be > 0")
	case m.HeightCm <= 0:
		return errors.New("units: HeightCm must be > 0")
	case m.WidthPx <= 0:
		return errors.New("units: WidthPx must be > 0")
	case m.HeightPx <= 0:
		return errors.New("units: HeightPx must be > 0")
	case m.DistanceCm <= 0:
		return errors.New("units: DistanceCm must be > 0")
	}
	return nil
}

// ── Horizontal conversions ────────────────────────────────────────────────────

// DegToPx converts a horizontal size in degrees of visual angle to pixels.
//
//	size_px = 2 · distance · tan(size_deg / 2) · (widthPx / widthCm)
func (m Monitor) DegToPx(deg float64) float64 {
	return m.DegToCm(deg) * float64(m.WidthPx) / m.WidthCm
}

// PxToDeg converts a horizontal size in pixels to degrees of visual angle.
func (m Monitor) PxToDeg(px float64) float64 {
	return m.CmToDeg(px * m.WidthCm / float64(m.WidthPx))
}

// CmToPx converts a horizontal size in centimetres to pixels.
func (m Monitor) CmToPx(cm float64) float64 {
	return cm * float64(m.WidthPx) / m.WidthCm
}

// PxToCm converts a horizontal size in pixels to centimetres.
func (m Monitor) PxToCm(px float64) float64 {
	return px * m.WidthCm / float64(m.WidthPx)
}

// ── Vertical conversions ──────────────────────────────────────────────────────

// DegToPxY converts a vertical size in degrees of visual angle to pixels.
// Equivalent to [Monitor.DegToPx] on displays with square pixels.
func (m Monitor) DegToPxY(deg float64) float64 {
	return m.DegToCm(deg) * float64(m.HeightPx) / m.HeightCm
}

// PxToDegY converts a vertical size in pixels to degrees of visual angle.
func (m Monitor) PxToDegY(px float64) float64 {
	return m.CmToDeg(px * m.HeightCm / float64(m.HeightPx))
}

// CmToPxY converts a vertical size in centimetres to pixels.
func (m Monitor) CmToPxY(cm float64) float64 {
	return cm * float64(m.HeightPx) / m.HeightCm
}

// PxToCmY converts a vertical size in pixels to centimetres.
func (m Monitor) PxToCmY(px float64) float64 {
	return px * m.HeightCm / float64(m.HeightPx)
}

// ── Distance-only conversions (no pixel density required) ────────────────────

// DegToCm converts an angle in degrees to a physical size in centimetres at
// the configured viewing distance.
//
//	size_cm = 2 · distance · tan(deg / 2)
func (m Monitor) DegToCm(deg float64) float64 {
	return 2 * m.DistanceCm * math.Tan(deg*math.Pi/360)
}

// CmToDeg converts a physical size in centimetres to degrees of visual angle
// at the configured viewing distance.
//
//	deg = 2 · atan(cm / (2 · distance)) · (180 / π)
func (m Monitor) CmToDeg(cm float64) float64 {
	return 2 * math.Atan(cm/(2*m.DistanceCm)) * 180 / math.Pi
}

// ── Summary statistics ────────────────────────────────────────────────────────

// PPcmX returns horizontal pixel density in pixels per centimetre.
func (m Monitor) PPcmX() float64 { return float64(m.WidthPx) / m.WidthCm }

// PPcmY returns vertical pixel density in pixels per centimetre.
func (m Monitor) PPcmY() float64 { return float64(m.HeightPx) / m.HeightCm }

// PPI returns horizontal pixel density in pixels per inch.
func (m Monitor) PPI() float64 { return m.PPcmX() * 2.54 }

// PPD returns pixels per degree of visual angle (horizontal) at the configured
// viewing distance. This is the canonical resolution unit in vision science.
//
//	PPD ≈ 2 · distance · tan(0.5°) · (widthPx / widthCm)
func (m Monitor) PPD() float64 { return m.DegToPx(1) }

// HasSquarePixels reports whether horizontal and vertical pixel densities agree
// to within 0.1 %. Most modern displays have square pixels; if this returns
// false use the Y-axis variants for vertical measurements.
func (m Monitor) HasSquarePixels() bool {
	dx, dy := m.PPcmX(), m.PPcmY()
	return math.Abs(dx-dy)/((dx+dy)*0.5) < 0.001
}

// String returns a human-readable summary of the monitor configuration.
func (m Monitor) String() string {
	return fmt.Sprintf(
		"%.1f×%.1f cm  |  %d×%d px  |  %.0f cm distance  |  %.0f PPI  |  %.1f px/°",
		m.WidthCm, m.HeightCm, m.WidthPx, m.HeightPx, m.DistanceCm, m.PPI(), m.PPD(),
	)
}
