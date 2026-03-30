// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Motion Blur & Phantom Array Stimulus
//
// Demonstrates the "TestUFO" perceptual effect using a VSYNC-locked 60 Hz loop:
//
//   - Lane 1 (top):    Static fixation cross + moving vertical white bar.
//     Staring at the cross reveals the "Phantom Array" — the
//     bar appears to split into ghost copies.
//   - Lane 2 (bottom): Moving white bar + co-moving green square (50 px ahead).
//     Tracking the square makes the bar look wide and blurry
//     ("Retinal Blur").
//
// Sync-strobe mode (toggle with S) draws the bar only on even frames (50% duty
// cycle), which sharpens the phantom effect.
//
// Measurement mode (M): a static white comparison rectangle appears at the
// bottom.  Adjust its width with ← / → to match the perceived blur width, then
// press Enter to record the match.
//
// Controls:
//
//	S          — toggle strobe mode
//	↑ / ↓      — velocity  +/- 50 px/s  (100 – 1500)
//	← / →      — bar width +/- 1 px     (1 – 10)  [normal mode]
//	            comparison width ± 1 px          [measure mode]
//	M          — toggle measurement mode
//	Enter      — record perceived width (measure mode)
//	ESC        — quit
//
// Usage:
//
//	go run main.go [-d] [-s <subject_id>]
package main

import (
	"fmt"
	"log"
	"runtime/debug"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	defaultVelocity = float32(800)
	velocityStep    = float32(50)
	minVelocity     = float32(100)
	maxVelocity     = float32(1500)

	defaultBarWidth = float32(2)
	minBarWidth     = float32(1)
	maxBarWidth     = float32(10)

	squareSize   = float32(20) // green tracking square side length
	squareLeadPx = float32(50) // bar is this many px behind the square

	measRectH    = float32(20) // height of comparison rectangle
	minMeasWidth = float32(1)
	maxMeasWidth = float32(500)

	hudFontSize = float32(18)
)

// ---------------------------------------------------------------------------
// Experiment state
// ---------------------------------------------------------------------------

type appState struct {
	velocity   float32
	barWidth   float32
	strobeMode bool
	frameCount int

	barX float32 // current bar position (center-relative x)

	measMode  bool
	measWidth float32

	trialID int

	fps      float32
	fpsCnt   int
	fpsTimer time.Time
}

// ---------------------------------------------------------------------------
// Cached text item  (only re-renders the GPU texture when the string changes)
// ---------------------------------------------------------------------------

type textItem struct {
	tl   *stimuli.TextLine
	text string
}

func (ti *textItem) draw(screen *apparatus.Screen, newText string, x, y float32, color sdl.Color) {
	if newText != ti.text || ti.tl == nil {
		if ti.tl != nil {
			_ = ti.tl.Unload()
		}
		ti.tl = stimuli.NewTextLine(newText, x, y, color)
		ti.text = newText
	}
	_ = ti.tl.Draw(screen)
}

func (ti *textItem) unload() {
	if ti.tl != nil {
		_ = ti.tl.Unload()
		ti.tl = nil
	}
}

// ---------------------------------------------------------------------------
// Low-level drawing helpers
// ---------------------------------------------------------------------------

// vertBar draws a vertical bar centred at (cx, cy) with the given dimensions.
func vertBar(screen *apparatus.Screen, cx, cy, w, h float32, color sdl.Color) {
	sdlX, sdlY := screen.CenterToSDL(cx, cy)
	_ = screen.Renderer.SetDrawColor(color.R, color.G, color.B, color.A)
	_ = screen.Renderer.RenderFillRect(&sdl.FRect{
		X: sdlX - w/2,
		Y: sdlY - h/2,
		W: w,
		H: h,
	})
}

// fillRect draws a filled rectangle centred at (cx, cy).
func fillRect(screen *apparatus.Screen, cx, cy, w, h float32, color sdl.Color) {
	sdlX, sdlY := screen.CenterToSDL(cx, cy)
	_ = screen.Renderer.SetDrawColor(color.R, color.G, color.B, color.A)
	_ = screen.Renderer.RenderFillRect(&sdl.FRect{
		X: sdlX - w/2,
		Y: sdlY - h/2,
		W: w,
		H: h,
	})
}

// hLine draws a horizontal line across the full screen width at center-y = cy.
func hLine(screen *apparatus.Screen, cy float32, color sdl.Color) {
	sw := float32(screen.Width)
	sx1, sy := screen.CenterToSDL(-sw/2, cy)
	sx2, _ := screen.CenterToSDL(sw/2, cy)
	_ = screen.Renderer.SetDrawColor(color.R, color.G, color.B, color.A)
	_ = screen.Renderer.RenderLine(sx1, sy, sx2, sy)
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	exp := control.NewExperimentFromFlags("Motion-Blur", control.Black, control.White, hudFontSize)
	defer exp.End()

	exp.AddDataVariableNames([]string{
		"trial_id", "velocity_px_per_sec", "actual_bar_width_px",
		"strobe_status", "perceived_width_px", "timestamp_ms",
	})

	if err := exp.Run(func() error {
		return animLoop(exp)
	}); err != nil && !control.IsEndLoop(err) {
		log.Fatalf("run: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Animation loop
// ---------------------------------------------------------------------------

func animLoop(exp *control.Experiment) error {
	// Disable GC to avoid jitter in the VSYNC-locked loop.
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	screen := exp.Screen
	sw := float32(screen.Width)
	sh := float32(screen.Height)

	// ---- Layout (center-relative coordinates; positive Y = up) ----
	//
	//   sh/2  ┌──────────── top of screen ──────────────┐
	//         │  HUD strip (one text line)               │
	//         │  ─────────────────────────────────────── │ ← hudLineY
	//         │  Lane 1 (top half)                       │
	//         │      fixation cross + moving bar         │
	//         │  instructions                            │
	//     0   ├──────────── lane separator ──────────────┤
	//         │  Lane 2 (bottom half)                    │
	//         │      bar + green square                  │
	//         │  instructions                            │
	//  -sh/2  └──────────── bottom of screen ────────────┘

	hudLineY := sh/2 - 12          // centre of the single HUD strip
	laneH := sh/2 - 2              // usable height inside each lane
	topLaneY := sh / 4             // centre of top lane
	botLaneY := -sh / 4            // centre of bottom lane
	instrOffY := float32(-10)      // instruction text offset below lane centre
	measRectY := -sh/2 + measRectH // comparison rect near bottom

	// Colours
	dimGray := sdl.Color{R: 55, G: 55, B: 55, A: 255}
	midGray := sdl.Color{R: 110, G: 110, B: 110, A: 255}
	green := sdl.Color{R: 0, G: 220, B: 0, A: 255}
	hudColor := sdl.Color{R: 200, G: 200, B: 200, A: 255}
	instrColor := sdl.Color{R: 85, G: 85, B: 85, A: 255}

	// ---- Persistent stimuli ----
	fixCross := stimuli.NewFixCross(18, 2, control.White)
	fixCross.SetPosition(sdl.FPoint{X: 0, Y: topLaneY})

	// Static instruction labels (text never changes).
	instrTop := stimuli.NewTextLine(
		`Stare at the Cross: Notice the bar splits into a "Phantom Array" of ghost bars.`,
		0, topLaneY+instrOffY, instrColor,
	)
	instrBot := stimuli.NewTextLine(
		`Follow the Green Square: Notice the bar becomes wide and blurry (Retinal Blur).`,
		0, botLaneY+instrOffY, instrColor,
	)

	// Cached HUD text items (refreshed only when value changes).
	var hudStatus, hudMeas, hudKeys textItem
	defer func() {
		hudStatus.unload()
		hudMeas.unload()
		hudKeys.unload()
	}()

	// ---- App state ----
	s := &appState{
		velocity:  defaultVelocity,
		barWidth:  defaultBarWidth,
		barX:      -sw / 2,
		measWidth: squareSize,
		fpsTimer:  time.Now(),
	}

	// Drain stale events before starting.
	var ev sdl.Event
	for sdl.PollEvent(&ev) {
	}

	lastTime := time.Now()

	for {
		// ---- Timing ----------------------------------------------------------------
		now := time.Now()
		dt := float32(now.Sub(lastTime).Seconds())
		if dt > 0.05 {
			dt = 0.05 // cap at 50 ms to absorb any hiccup
		}
		lastTime = now

		// FPS counter (updated twice per second).
		s.fpsCnt++
		if elapsed := now.Sub(s.fpsTimer).Seconds(); elapsed >= 0.5 {
			s.fps = float32(s.fpsCnt) / float32(elapsed)
			s.fpsCnt = 0
			s.fpsTimer = now
		}

		// ---- Physics ---------------------------------------------------------------
		s.barX += s.velocity * dt
		// Wrap: re-enter from the opposite edge the instant the bar exits.
		halfBar := s.barWidth / 2
		if s.barX-halfBar > sw/2 {
			s.barX = -sw/2 - halfBar
		}

		// ---- Render ----------------------------------------------------------------
		_ = screen.Clear()

		// Lane separator.
		hLine(screen, 0, dimGray)

		// ── Top lane ──
		_ = fixCross.Draw(screen)

		drawBars := !s.strobeMode || s.frameCount%2 == 0
		if drawBars {
			vertBar(screen, s.barX, topLaneY, s.barWidth, laneH, control.White)
		}

		// ── Bottom lane ──
		squareX := s.barX + squareLeadPx
		// Wrap the square independently so it doesn't vanish mid-screen.
		if squareX-squareSize/2 > sw/2 {
			squareX -= sw + squareSize
		}
		fillRect(screen, squareX, botLaneY, squareSize, squareSize, green)

		if drawBars {
			vertBar(screen, s.barX, botLaneY, s.barWidth, laneH, control.White)
		}

		// ── Instructions (drawn over lanes at a fixed dim offset) ──
		_ = instrTop.Draw(screen)
		_ = instrBot.Draw(screen)

		// ── Measurement comparison rectangle ──
		if s.measMode {
			fillRect(screen, 0, measRectY, s.measWidth, measRectH, control.White)
			// Thin bracket lines to help judge width.
			bracketH := measRectH + 6
			bY1sdl, _ := screen.CenterToSDL(0, measRectY-bracketH/2)
			bY2sdl, _ := screen.CenterToSDL(0, measRectY+bracketH/2)
			bXLsdl, bYmidsdl := screen.CenterToSDL(-s.measWidth/2, measRectY)
			bXRsdl, _ := screen.CenterToSDL(s.measWidth/2, measRectY)
			_ = screen.Renderer.SetDrawColor(midGray.R, midGray.G, midGray.B, midGray.A)
			_ = screen.Renderer.RenderLine(bXLsdl, bY1sdl, bXLsdl, bY2sdl)
			_ = screen.Renderer.RenderLine(bXRsdl, bY1sdl, bXRsdl, bY2sdl)
			_ = bYmidsdl
		}

		// ── HUD strip ──
		strobeStr := "OFF"
		if s.strobeMode {
			strobeStr = "ON"
		}
		hudText := fmt.Sprintf(
			"FPS: %4.1f   Vel: %4.0f px/s [↑↓]   Bar: %.0f px [←→]   Strobe: %s [S]   Measure [M]   Quit [ESC]",
			s.fps, s.velocity, s.barWidth, strobeStr,
		)
		hudStatus.draw(screen, hudText, 0, hudLineY, hudColor)

		if s.measMode {
			measText := fmt.Sprintf(
				"MEASURE MODE — Comparison width: %.0f px  [←/→] adjust  [Enter] record  [M] exit",
				s.measWidth,
			)
			hudMeas.draw(screen, measText, 0, measRectY+measRectH+12, control.White)
		}

		_ = instrTop // already drawn above; suppress "unused" warning
		_ = instrBot

		// VSYNC-locked flip.
		_ = screen.Update()
		s.frameCount++

		// ---- Event handling --------------------------------------------------------
		state := exp.PollEvents(func(e sdl.Event) bool {
			if e.Type != sdl.EVENT_KEY_DOWN {
				return false
			}
			switch e.KeyboardEvent().Key {
			case sdl.K_S:
				s.strobeMode = !s.strobeMode

			case sdl.K_UP:
				s.velocity += velocityStep
				if s.velocity > maxVelocity {
					s.velocity = maxVelocity
				}
			case sdl.K_DOWN:
				s.velocity -= velocityStep
				if s.velocity < minVelocity {
					s.velocity = minVelocity
				}

			case sdl.K_LEFT:
				if s.measMode {
					s.measWidth--
					if s.measWidth < minMeasWidth {
						s.measWidth = minMeasWidth
					}
				} else {
					s.barWidth--
					if s.barWidth < minBarWidth {
						s.barWidth = minBarWidth
					}
				}
			case sdl.K_RIGHT:
				if s.measMode {
					s.measWidth++
					if s.measWidth > maxMeasWidth {
						s.measWidth = maxMeasWidth
					}
				} else {
					s.barWidth++
					if s.barWidth > maxBarWidth {
						s.barWidth = maxBarWidth
					}
				}

			case sdl.K_M:
				s.measMode = !s.measMode

			case sdl.K_RETURN, sdl.K_KP_ENTER:
				if s.measMode {
					s.trialID++
					status := "off"
					if s.strobeMode {
						status = "on"
					}
					exp.Data.Add(
						s.trialID,
						s.velocity,
						s.barWidth,
						status,
						s.measWidth,
						clock.GetTime(),
					)
				}
			}
			return false
		})

		if state.QuitRequested {
			return control.EndLoop
		}
	}
}
