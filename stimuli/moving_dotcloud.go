// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

// moving_dotcloud.go — animated random-dot cloud with response detection.
//
// PresentMovingDotCloud runs a VSYNC-locked animation in which each dot
// moves at a constant speed in a randomly chosen direction and is respawned
// at a random position inside the cloud whenever it exits the boundary.
//
// The loop ends when:
//   - an interrupt key is pressed (if interruptKeys != nil),
//   - any mouse button is pressed (if catchMouse is true),
//   - the maximum duration elapses, or
//   - Escape / window-close is received (returns sdl.EndLoop error).
//
// RTms in the returned MotionResult is measured from the moment the first
// frame is presented to the moment the event is detected; it will be within
// one display frame of the true response time.

import (
	"math"
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// MotionResult holds the outcome of a PresentMovingDotCloud call.
type MotionResult struct {
	// Key is the keycode of the interrupt key pressed; 0 on timeout or mouse.
	Key sdl.Keycode
	// Button is the mouse button pressed (1=left, 2=middle, 3=right); 0 on timeout or key.
	Button uint8
	// RTms is the elapsed time in milliseconds from the first presented frame
	// to the response event. Equals the actual elapsed time on timeout.
	RTms int64
}

// movingDot holds the per-dot animation state.
type movingDot struct {
	x, y   float32 // center-relative position (pixels)
	vx, vy float32 // velocity per frame (pixels/frame at current refresh rate)
}

// newMovingDot returns a dot at a random position within the cloud circle,
// moving at speedPxPerSec pixels/second in a random direction.
// dt is the frame duration in seconds (1 / refreshRate).
func newMovingDot(cloudRadius, dotRadius, speedPxPerSec, dt float32) movingDot {
	// Random direction.
	angle := rand.Float64() * 2 * math.Pi
	vx := float32(math.Cos(angle)) * speedPxPerSec * dt
	vy := float32(math.Sin(angle)) * speedPxPerSec * dt

	// Uniform-area random position inside the cloud, keeping the dot fully inside.
	inner := cloudRadius - dotRadius
	if inner < 0 {
		inner = 0
	}
	r := float32(math.Sqrt(rand.Float64())) * inner
	posAngle := rand.Float64() * 2 * math.Pi
	x := r * float32(math.Cos(posAngle))
	y := r * float32(math.Sin(posAngle))

	return movingDot{x: x, y: y, vx: vx, vy: vy}
}

// drawFilledCircle draws a solid filled circle into the renderer at SDL-space
// coordinates (cx, cy) with the given radius. The draw color must be set
// by the caller before calling this helper.
func drawFilledCircle(r *sdl.Renderer, cx, cy, radius float32) {
	for dy := -radius; dy <= radius; dy++ {
		dx := float32(math.Sqrt(float64(radius*radius - dy*dy)))
		r.RenderLine(cx-dx, cy+dy, cx+dx, cy+dy)
	}
}

// PresentMovingDotCloud displays an animated cloud of randomly moving dots
// for up to maxDurationMs milliseconds and optionally waits for a response.
//
// Parameters:
//   - screen         SDL screen (window + renderer).
//   - nDots          Number of dots in the cloud.
//   - dotRadius      Radius of each individual dot in pixels.
//   - cloudRadius    Radius of the circular cloud boundary in pixels.
//   - center         Cloud centre in screen-centre coordinates (0,0 = screen centre).
//   - speedPxPerSec  Speed of every dot in pixels per second.
//   - maxDurationMs  Maximum display time in milliseconds (0 = infinite).
//   - interruptKeys  Keys that immediately end the display; nil = no key interrupt.
//   - catchMouse     If true, any mouse button press ends the display.
//   - dotColor       Colour of the dots.
//   - bgColor        Colour of the circular cloud background (A=0 → transparent).
//
// Returns a MotionResult with the response key/button and RT, plus any error.
// On ESC or window-close the error is sdl.EndLoop.
func PresentMovingDotCloud(
	screen *apparatus.Screen,
	nDots int,
	dotRadius float32,
	cloudRadius float32,
	center sdl.FPoint,
	speedPxPerSec float32,
	maxDurationMs int64,
	interruptKeys []sdl.Keycode,
	catchMouse bool,
	dotColor sdl.Color,
	bgColor sdl.Color,
) (MotionResult, error) {

	// ── Refresh rate ──────────────────────────────────────────────────────────
	var refreshRate float64 = 60.0
	displayID := sdl.GetDisplayForWindow(screen.Window)
	if mode, err := displayID.CurrentDisplayMode(); err == nil && mode != nil && mode.RefreshRate > 0 {
		refreshRate = float64(mode.RefreshRate)
	}
	dt := float32(1.0 / refreshRate) // seconds per frame

	// ── Initialise dots ───────────────────────────────────────────────────────
	dots := make([]movingDot, nDots)
	for i := range dots {
		dots[i] = newMovingDot(cloudRadius, dotRadius, speedPxPerSec, dt)
	}

	// ── Drain stale events ────────────────────────────────────────────────────
	var ev sdl.Event
	for sdl.PollEvent(&ev) {
	}

	// ── Disable GC during the animation loop ──────────────────────────────────
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	// SDL-space centre coordinates (computed once).
	cx, cy := screen.CenterToSDL(center.X, center.Y)

	start := time.Now()

	for {
		// ── Draw ─────────────────────────────────────────────────────────────

		if err := screen.Clear(); err != nil {
			return MotionResult{}, err
		}

		// Optional cloud background.
		if bgColor.A > 0 {
			if err := screen.Renderer.SetDrawColor(bgColor.R, bgColor.G, bgColor.B, bgColor.A); err != nil {
				return MotionResult{}, err
			}
			drawFilledCircle(screen.Renderer, cx, cy, cloudRadius)
		}

		// Dots.
		if err := screen.Renderer.SetDrawColor(dotColor.R, dotColor.G, dotColor.B, dotColor.A); err != nil {
			return MotionResult{}, err
		}
		for _, d := range dots {
			px, py := screen.CenterToSDL(center.X+d.x, center.Y+d.y)
			drawFilledCircle(screen.Renderer, px, py, dotRadius)
		}

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

		// ── Timeout check ─────────────────────────────────────────────────────
		if maxDurationMs > 0 && rtMs >= maxDurationMs {
			return MotionResult{RTms: rtMs}, nil
		}

		// ── Update dot positions ──────────────────────────────────────────────
		for i := range dots {
			dots[i].x += dots[i].vx
			dots[i].y += dots[i].vy
			// Respawn dots that have left the cloud boundary.
			distSq := float64(dots[i].x*dots[i].x + dots[i].y*dots[i].y)
			boundary := float64(cloudRadius - dotRadius)
			if math.Sqrt(distSq) > boundary {
				dots[i] = newMovingDot(cloudRadius, dotRadius, speedPxPerSec, dt)
			}
		}
	}
}
