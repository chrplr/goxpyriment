// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

// moving_grating.go — animated sinusoidal gratings (rectangular and Gabor).
//
// Both PresentMovingGrating and PresentMovingGabor run a VSYNC-locked
// animation loop that drifts a sinusoidal luminance grating across the screen.
// They share the same event-handling and return conventions as
// PresentMovingDotCloud (see moving_dotcloud.go).
//
// Design:
//   - A gratingState is initialised once before the loop. It pre-computes, for
//     each pixel, the "spatial argument" (2π × spatialFreq × projected coord).
//     For a Gabor it also pre-computes the Gaussian envelope and bakes it into
//     a static alpha look-up table.
//   - Every frame only needs to evaluate cos(spatialArg[i] + phase(t)) per
//     pixel — no per-frame memory allocation. The pixel buffer ([]byte, RGBA)
//     is allocated once and reused; sdl.CreateSurfaceFrom wraps it without
//     copying, then CreateTextureFromSurface uploads to GPU.
//   - phase(t) = 2π × temporalFreq × elapsed_seconds, giving smooth continuous
//     drift independent of frame rate.
//
// Luminance formula (for rectangular grating, envelope = 1):
//   L(x,y,t) = bgLuminance + contrast × bgLuminance × cos(spatialArg[i] + phase(t))
//
// For the Gabor:
//   L(x,y,t) = bgLuminance + contrast × bgLuminance × envelope(x,y) × cos(...)
//   alpha(x,y) = envelope(x,y)   (edges fade out via alpha blending)

import (
	"math"
	"runtime/debug"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// ── Pre-computed grating state ────────────────────────────────────────────────

// gratingState holds the per-pixel spatial data that is constant across frames.
type gratingState struct {
	w, h       int
	pixels     []byte    // RGBA pixel buffer, w*h*4 bytes; reused every frame
	spatialArg []float64 // 2π × spatialFreq × x_projected[i], one entry per pixel
	envelope   []float64 // Gaussian envelope [0,1] per pixel; nil for rectangular
	alphaLUT   []byte    // pre-baked uint8 alpha per pixel; nil for rectangular
	meanGray   float64   // bgLuminance × 255
	amplitude  float64   // contrast × meanGray (peak grating swing in 0-255 units)
}

// buildGratingState pre-computes spatialArg and (when sigma > 0) the Gaussian
// envelope. orientation is in degrees measured from the horizontal axis;
// sigma = 0 selects a rectangular (uniform) aperture.
func buildGratingState(w, h int, orientation, spatialFreqCPP, bgLuminance, contrast, sigma float64) gratingState {
	g := gratingState{
		w:          w,
		h:          h,
		pixels:     make([]byte, w*h*4),
		spatialArg: make([]float64, w*h),
		meanGray:   bgLuminance * 255.0,
		amplitude:  contrast * bgLuminance * 255.0,
	}
	if sigma > 0 {
		g.envelope = make([]float64, w*h)
		g.alphaLUT = make([]byte, w*h)
	}

	thetaRad := orientation * math.Pi / 180.0
	cosTheta := math.Cos(thetaRad)
	sinTheta := math.Sin(thetaRad)
	halfW := float64(w) / 2.0
	halfH := float64(h) / 2.0
	twoPiSF := 2.0 * math.Pi * spatialFreqCPP
	twoSigmaSq := 2.0 * sigma * sigma

	for row := 0; row < h; row++ {
		yf := float64(row) - halfH
		for col := 0; col < w; col++ {
			xf := float64(col) - halfW
			i := row*w + col

			// Coordinate along the grating-perpendicular axis (the direction of
			// luminance variation). Orientation 0° → vertical bars, drift right.
			xp := xf*cosTheta + yf*sinTheta
			g.spatialArg[i] = twoPiSF * xp

			if sigma > 0 {
				env := math.Exp(-(xf*xf + yf*yf) / twoSigmaSq)
				g.envelope[i] = env
				g.alphaLUT[i] = byte(env * 255.0)
			}
		}
	}
	return g
}

// updatePixels writes the RGBA pixel buffer for the given instantaneous phase
// (in radians). No allocations are performed.
func (g *gratingState) updatePixels(phase float64) {
	mg := g.meanGray
	amp := g.amplitude

	if g.envelope == nil {
		// Rectangular grating: uniform aperture, fully opaque.
		for i := 0; i < g.w*g.h; i++ {
			lum := mg + amp*math.Cos(g.spatialArg[i]+phase)
			if lum < 0 {
				lum = 0
			} else if lum > 255 {
				lum = 255
			}
			c := byte(lum)
			base := i * 4
			g.pixels[base+0] = c
			g.pixels[base+1] = c
			g.pixels[base+2] = c
			g.pixels[base+3] = 255
		}
	} else {
		// Gabor: envelope modulates both luminance swing and alpha.
		for i := 0; i < g.w*g.h; i++ {
			env := g.envelope[i]
			lum := mg + amp*env*math.Cos(g.spatialArg[i]+phase)
			if lum < 0 {
				lum = 0
			} else if lum > 255 {
				lum = 255
			}
			c := byte(lum)
			base := i * 4
			g.pixels[base+0] = c
			g.pixels[base+1] = c
			g.pixels[base+2] = c
			g.pixels[base+3] = g.alphaLUT[i]
		}
	}
}

// ── Shared animation loop ─────────────────────────────────────────────────────

// presentGratingLoop is the VSYNC-locked inner loop shared by both grating
// variants. g.pixels is updated each frame, uploaded to a short-lived SDL
// texture, rendered, and destroyed — the pixel buffer itself is reused.
func presentGratingLoop(
	screen *apparatus.Screen,
	g *gratingState,
	center sdl.FPoint,
	temporalFreq float64,
	maxDurationMs int64,
	interruptKeys []sdl.Keycode,
	catchMouse bool,
) (MotionResult, error) {

	// Destination rectangle in SDL-space (top-left origin, fixed each frame).
	cx, cy := screen.CenterToSDL(center.X, center.Y)
	destRect := &sdl.FRect{
		X: cx - float32(g.w)/2,
		Y: cy - float32(g.h)/2,
		W: float32(g.w),
		H: float32(g.h),
	}

	// Drain stale input events before starting.
	var ev sdl.Event
	for sdl.PollEvent(&ev) {
	}

	// Disable GC during the animation loop to prevent jitter.
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	twoPiTF := 2.0 * math.Pi * temporalFreq
	start := time.Now()

	for {
		phase := twoPiTF * time.Since(start).Seconds()

		// ── Compute pixel data for this frame ────────────────────────────────
		g.updatePixels(phase)

		// ── Upload to GPU via a transient surface + texture ──────────────────
		// sdl.CreateSurfaceFrom wraps g.pixels without copying.
		// CreateTextureFromSurface uploads to GPU (synchronous) then the
		// surface can be discarded. The texture is destroyed after rendering.
		surface, err := sdl.CreateSurfaceFrom(g.w, g.h, sdl.PIXELFORMAT_RGBA32, g.pixels, g.w*4)
		if err != nil {
			return MotionResult{}, err
		}
		texture, err := screen.Renderer.CreateTextureFromSurface(surface)
		surface.Destroy()
		if err != nil {
			return MotionResult{}, err
		}

		// ── Draw ─────────────────────────────────────────────────────────────
		if err := screen.Clear(); err != nil {
			texture.Destroy()
			return MotionResult{}, err
		}
		if err := screen.Renderer.RenderTexture(texture, nil, destRect); err != nil {
			texture.Destroy()
			return MotionResult{}, err
		}
		texture.Destroy()

		// ── Present (VSYNC-locked) ────────────────────────────────────────────
		if err := screen.Update(); err != nil {
			return MotionResult{}, err
		}

		rtMs := time.Since(start).Milliseconds()

		// ── Poll events ───────────────────────────────────────────────────────
		for sdl.PollEvent(&ev) {
			switch ev.Type {
			case sdl.EVENT_KEY_DOWN:
				k := ev.KeyboardEvent().Key
				if k == sdl.K_ESCAPE {
					return MotionResult{RTms: rtMs}, sdl.EndLoop
				}
				if interruptKeys != nil {
					for _, ik := range interruptKeys {
						if k == ik {
							return MotionResult{Key: k, RTms: rtMs}, nil
						}
					}
				}
			case sdl.EVENT_MOUSE_BUTTON_DOWN:
				if catchMouse {
					btn := ev.MouseButtonEvent().Button
					return MotionResult{Button: uint8(btn), RTms: rtMs}, nil
				}
			case sdl.EVENT_QUIT:
				return MotionResult{RTms: rtMs}, sdl.EndLoop
			}
		}

		// ── Timeout ───────────────────────────────────────────────────────────
		if maxDurationMs > 0 && rtMs >= maxDurationMs {
			return MotionResult{RTms: rtMs}, nil
		}
	}
}

// ── Public API ────────────────────────────────────────────────────────────────

// PresentMovingGrating displays a drifting sinusoidal grating in a rectangular
// aperture and optionally waits for a response.
//
// Parameters:
//
//   - screen        — SDL screen (window + renderer).
//   - width, height — Aperture size in pixels.
//   - center        — Centre position in screen-centre coordinates (0,0 = screen centre).
//   - orientation   — Grating bar angle in degrees from horizontal
//     (0° = vertical bars drifting right; 90° = horizontal bars drifting down).
//   - spatialFreq   — Spatial frequency in cycles per pixel
//     (e.g. 0.05 → one cycle every 20 pixels).
//   - temporalFreq  — Drift speed in cycles per second (Hz).
//     Positive values drift in the direction of increasing orientation angle.
//   - contrast      — Michelson contrast in [0, 1].
//   - bgLuminance   — Mean luminance in [0, 1] (0.5 = mid-gray).
//   - maxDurationMs — Maximum display time in ms; 0 = run until a response.
//   - interruptKeys — Keycodes that end the display; nil = ignore all keys.
//   - catchMouse    — If true, any mouse button press ends the display.
//
// Returns a MotionResult (key / mouse button / RT) and any error.
// ESC or window-close returns sdl.EndLoop as the error.
func PresentMovingGrating(
	screen *apparatus.Screen,
	width, height float32,
	center sdl.FPoint,
	orientation float64,
	spatialFreq float64,
	temporalFreq float64,
	contrast float64,
	bgLuminance float64,
	maxDurationMs int64,
	interruptKeys []sdl.Keycode,
	catchMouse bool,
) (MotionResult, error) {
	g := buildGratingState(int(width), int(height), orientation, spatialFreq, bgLuminance, contrast, 0)
	return presentGratingLoop(screen, &g, center, temporalFreq, maxDurationMs, interruptKeys, catchMouse)
}

// PresentMovingGabor displays a drifting Gabor patch: a sinusoidal grating
// windowed by an isotropic Gaussian envelope. The patch is alpha-blended onto
// the existing screen content so that the edges fade smoothly into the
// background set by the experiment.
//
// Parameters:
//
//   - screen        — SDL screen.
//   - size          — Bounding-box side length in pixels.
//     Should be at least 6 × sigma to capture the full Gaussian.
//   - sigma         — Gaussian standard deviation in pixels.
//   - center        — Centre position in screen-centre coordinates.
//   - orientation   — Grating bar angle in degrees from horizontal.
//   - spatialFreq   — Spatial frequency in cycles per pixel.
//   - temporalFreq  — Drift speed in Hz.
//   - contrast      — Michelson contrast in [0, 1].
//   - bgLuminance   — Mean luminance in [0, 1].
//   - maxDurationMs — Maximum display time in ms; 0 = run until a response.
//   - interruptKeys — Keycodes that end the display; nil = ignore all keys.
//   - catchMouse    — If true, any mouse button press ends the display.
func PresentMovingGabor(
	screen *apparatus.Screen,
	size float32,
	sigma float64,
	center sdl.FPoint,
	orientation float64,
	spatialFreq float64,
	temporalFreq float64,
	contrast float64,
	bgLuminance float64,
	maxDurationMs int64,
	interruptKeys []sdl.Keycode,
	catchMouse bool,
) (MotionResult, error) {
	g := buildGratingState(int(size), int(size), orientation, spatialFreq, bgLuminance, contrast, sigma)
	return presentGratingLoop(screen, &g, center, temporalFreq, maxDurationMs, interruptKeys, catchMouse)
}
