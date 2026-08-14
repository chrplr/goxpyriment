// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Tearing Test
//
// Displays a full-height vertical white bar sweeping horizontally across
// the screen.  Screen tearing appears as a horizontal discontinuity in the
// bar edge.
//
// On exit, frame-interval statistics (identical to the jitter sub-test of
// Timing-Tests) are printed to the console.
//
// Controls:
//
//	↑ / ↓   — speed   +/- 50 px/s  (range 50 – 3000)
//	← / →   — width   +/- 1 px     (range 1 – 200)
//	ESC / Q — quit
//
// Flags:
//
//	-bar-width <px>  bar width in pixels  (default 4)
//	-v <px/s>        speed in pixels/sec  (default 800)
//	-w               windowed mode (1024×768 instead of fullscreen)
//	-d <n>           display index where the window opens (-1 = primary)
//	-s <id>          subject ID
package main

import (
	"flag"
	"fmt"
	"log"
	"runtime/debug"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/tests/internal/timingstats"
)

const (
	speedStep   = float32(50)
	minSpeed    = float32(50)
	maxSpeed    = float32(3000)
	minBarWidth = float32(1)
	maxBarWidth = float32(200)
	hudFontSize = float32(18)
	warmup      = 10 // frames discarded from statistics at startup
)

// ── Drawing helpers ───────────────────────────────────────────────────────────

// textItem caches a TextLine and only reallocates the GPU texture when the
// string content changes.
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

// vertBar draws a filled vertical bar whose SDL x-centre is cx, spanning the
// full screen height.
func vertBar(screen *apparatus.Screen, cx, w float32, color sdl.Color) {
	_ = screen.Renderer.SetDrawColor(color.R, color.G, color.B, color.A)
	_ = screen.Renderer.RenderFillRect(&sdl.FRect{
		X: cx - w/2,
		Y: 0,
		W: w,
		H: float32(screen.Height),
	})
}

// ── Entry point ───────────────────────────────────────────────────────────────

func main() {
	// Not -w: that name belongs to the framework's windowed-mode flag, which
	// NewExperimentFromFlags registers below on the same flag.CommandLine.
	// Defining it here too panicked the program at startup ("flag redefined: w")
	// before it could draw anything.
	barWidthFlag := flag.Float64("bar-width", 4, "bar width in pixels")
	speedFlag := flag.Float64("v", 800, "speed in pixels per second")

	exp := control.NewExperimentFromFlags("Tearing Test", control.Black, control.White, hudFontSize)
	defer exp.End()

	barWidth := float32(*barWidthFlag)
	speed := float32(*speedFlag)

	if err := exp.Run(func() error {
		return animLoop(exp, barWidth, speed)
	}); err != nil && !control.IsEndLoop(err) {
		log.Fatalf("run: %v", err)
	}
}

// ── Animation loop ────────────────────────────────────────────────────────────

func animLoop(exp *control.Experiment, initBarWidth, initSpeed float32) error {
	// Disable GC to avoid jitter in the VSYNC-locked loop.
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	screen := exp.Screen
	sw := float32(screen.Width)
	sh := float32(screen.Height)

	barWidth := initBarWidth
	speed := initSpeed

	// barX is the SDL x-coordinate of the bar centre (0 = left edge of screen).
	barX := float32(0)

	hudColor := sdl.Color{R: 200, G: 200, B: 200, A: 255}
	hudY := -sh/2 + 10 // centre-relative: near top of screen

	var hud textItem
	defer hud.unload()

	// Drain stale events before the loop.
	var ev sdl.Event
	for sdl.PollEvent(&ev) {
	}

	var (
		lastTime  = time.Now()
		fpsCnt    int
		fps       float32
		fpsTimer  = time.Now()
		frame     int
		prevT     float64
		intervals []float64
	)

	for {
		// ---- Timing ----------------------------------------------------------------
		now := time.Now()
		dt := float32(now.Sub(lastTime).Seconds())
		if dt > 0.05 {
			dt = 0.05
		}
		lastTime = now

		fpsCnt++
		if elapsed := now.Sub(fpsTimer).Seconds(); elapsed >= 0.5 {
			fps = float32(fpsCnt) / float32(elapsed)
			fpsCnt = 0
			fpsTimer = now
		}

		// ---- Physics ---------------------------------------------------------------
		barX += speed * dt
		// Wrap: re-enter from the left once the bar fully exits the right edge.
		if barX-barWidth/2 > sw {
			barX = -barWidth / 2
		}

		// ---- Render ----------------------------------------------------------------
		_ = screen.Clear()
		vertBar(screen, barX, barWidth, control.White)

		hudText := fmt.Sprintf(
			"FPS: %4.1f   Speed: %.0f px/s [↑↓]   Width: %.0f px [←→]   Quit [ESC/Q]",
			fps, speed, barWidth,
		)
		hud.draw(screen, hudText, 0, hudY, hudColor)

		_ = screen.Update() // blocks until VSYNC

		// ---- Frame-interval measurement (mirrors runJitter) ------------------------
		tA := float64(clock.GetTimeNS()) / 1e6 // ms with sub-ms precision
		if prevT > 0 && frame >= warmup {
			intervals = append(intervals, tA-prevT)
		}
		prevT = tA
		frame++

		// ---- Events ----------------------------------------------------------------
		quit := false
		state := exp.PollEvents(func(e sdl.Event) bool {
			if e.Type != sdl.EVENT_KEY_DOWN {
				return false
			}
			switch e.KeyboardEvent().Key {
			case sdl.K_UP:
				speed += speedStep
				if speed > maxSpeed {
					speed = maxSpeed
				}
			case sdl.K_DOWN:
				speed -= speedStep
				if speed < minSpeed {
					speed = minSpeed
				}
			case sdl.K_RIGHT:
				barWidth++
				if barWidth > maxBarWidth {
					barWidth = maxBarWidth
				}
			case sdl.K_LEFT:
				barWidth--
				if barWidth < minBarWidth {
					barWidth = minBarWidth
				}
			case sdl.K_Q:
				quit = true
			}
			return false
		})

		if state.QuitRequested || quit {
			break
		}
	}

	// ---- Print statistics (same format as the jitter sub-test) -----------------
	s := timingstats.ComputeStats(intervals, 16.67) // first pass
	estimatedHz := 0.0
	if s.Mean > 0 {
		estimatedHz = 1000.0 / s.Mean
		s = timingstats.ComputeStats(intervals, s.Mean) // recompute late counts against actual mean
	}
	fmt.Printf("\nEstimated refresh rate: %.3f Hz\n", estimatedHz)
	timingstats.PrintStats("Frame intervals", s, s.Mean)

	exp.AddDataVariableNames([]string{"frame", "interval_ms"})
	exp.Data.WriteComment(fmt.Sprintf("tearing estimated_hz=%.4f", estimatedHz))
	timingstats.Save(exp.Data, "tearing frame_intervals", s, s.Mean)

	return control.EndLoop
}
