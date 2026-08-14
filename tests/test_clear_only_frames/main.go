// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Clear-Only Frames — regression test for a compositor presentation bug.
//
// This test deliberately presents frames whose only content is a clear: it sets
// a draw colour, calls SDL_RenderClear, and presents. No draw calls are ever
// issued. It alternates between two luminance levels so a photodiode (or the
// naked eye) can tell whether every committed frame actually reached the panel.
//
// # Why this exists
//
// On Linux under a compositor (reproduced on GNOME/Mutter, both native Wayland
// and Xwayland, on Intel Meteor Lake / i915, with the opengl, vulkan and
// software SDL renderers), a frame containing nothing but SDL_RenderClear is not
// reliably scanned out. The panel can hold a stale frame for seconds at a time
// while the client and the compositor both report every frame presented on
// schedule — the application's own flip timestamps look perfect throughout, so
// the fault is invisible without a photodiode. Adding a single real draw call
// per frame makes it disappear. The same loop is correct on the kmsdrm backend,
// where no compositor is involved.
//
// The library defends against this in apparatus.Screen, so ordinary experiment
// code cannot hit it. This test bypasses that defence on purpose, by driving
// Screen.Renderer directly, and is the only way to verify the defence still
// works. If it ever starts failing again, the guarantee has regressed.
//
// Expected result
//
//	PASS  a clean square wave: N bright frames, N dark frames, forever
//	FAIL  frozen frames lasting seconds, or a flicker unrelated to the pattern
//
// The app-side statistics printed on exit are NOT a pass/fail signal — they were
// textbook-perfect throughout the original bug. Judge by the screen.
//
// Usage:
//
//	go run ./tests/test_clear_only_frames                  # bypass the library defence (default)
//	go run ./tests/test_clear_only_frames -guarded         # go through Screen.Clear instead
//	go run ./tests/test_clear_only_frames -frames-on 12 -frames-off 18
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/tests/internal/report"
	"github.com/chrplr/goxpyriment/tests/internal/timingstats"
)

var (
	fFramesOn  = flag.Int("frames-on", 12, "Bright frames per cycle")
	fFramesOff = flag.Int("frames-off", 18, "Dark frames per cycle")
	fCycles    = flag.Int("cycles", 100, "Number of cycles")
	fLevelA    = flag.Int("level-a", 0, "Dark luminance 0–255")
	fLevelB    = flag.Int("level-b", 255, "Bright luminance 0–255")
	fGuarded   = flag.Bool("guarded", false, "Use Screen.Clear (the guarded library path) instead of\n\tdriving Screen.Renderer directly. The screen should be stable either\n\tway; if -guarded is stable but the default is not, the library defence\n\tis working and the raw renderer remains exposed.")
	fWindowed  = flag.Bool("w", false, "Windowed mode (1024×768 window instead of fullscreen)")
	fDisplay   = flag.Int("d", -1, "Display index (-1 = primary)")
)

func main() {
	flag.Parse()

	width, height, fullscreen := 0, 0, true
	if *fWindowed {
		width, height, fullscreen = 1024, 768, false
	}
	exp := control.NewExperiment("Clear-Only Frames", width, height, fullscreen,
		control.Black, control.White, 24)
	if *fDisplay >= 0 {
		exp.ScreenNumber = *fDisplay
	}
	if err := exp.Initialize(); err != nil {
		exp.End()
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	mode := "raw renderer (library defence bypassed)"
	if *fGuarded {
		mode = "Screen.Clear (guarded library path)"
	}
	fmt.Printf("clear-only: %s\n", mode)
	fmt.Printf("clear-only: %d cycles of %d bright / %d dark frames — watch the SCREEN, not the numbers\n",
		*fCycles, *fFramesOn, *fFramesOff)

	var periods []float64
	var prevOnset float64

	err := exp.Run(func() error {
		r := exp.Screen.Renderer
		bLevel, dLevel := byte(*fLevelB), byte(*fLevelA)

		// present emits one frame at the given level and flips. In the default
		// (unguarded) mode this is the exact shape that triggers the bug:
		// SetDrawColor + Clear + Present, with no draw call in between.
		present := func(level byte) {
			if *fGuarded {
				exp.Screen.BgColor = control.RGB(level, level, level)
				_ = exp.Screen.Clear()
			} else {
				_ = r.SetDrawColor(level, level, level, 255)
				_ = r.Clear()
			}
			_ = exp.Screen.Update()
			control.PumpEvents()
		}

		for cycle := 0; cycle < *fCycles; cycle++ {
			var onset float64
			for f := 0; f < *fFramesOn; f++ {
				present(bLevel)
				if f == 0 {
					onset = float64(clock.GetTimeNS()) / 1e6
				}
			}
			for f := 0; f < *fFramesOff; f++ {
				present(dLevel)
			}

			if prevOnset > 0 {
				periods = append(periods, onset-prevOnset)
			}
			prevOnset = onset

			if exp.PollEvents(nil).QuitRequested {
				return control.EndLoop
			}
		}
		return control.EndLoop
	})
	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("test error: %v", err)
	}

	// The verdict here is what the SCREEN did, which only a human can supply, so
	// the mode and the settings are recorded to give that judgement something to
	// attach to. The periods go in as supporting evidence, still not a pass/fail.
	exp.Data.WriteComment(fmt.Sprintf("clearonly mode=%s guarded=%t cycles=%d frames_on=%d frames_off=%d level_bright=%d level_dark=%d",
		mode, *fGuarded, *fCycles, *fFramesOn, *fFramesOff, *fLevelB, *fLevelA))
	out := &report.Tee{}
	defer out.Flush(exp.Data, "clear-only frames report")
	out.Printf("clear-only: %s\n", mode)
	if len(periods) > 0 {
		s := timingstats.ComputeStats(periods, 0)
		s = timingstats.ComputeStats(periods, s.Mean)
		timingstats.FprintStats(out, "Cycle period (app-side — NOT a pass/fail signal)", s, s.Mean)
		exp.AddDataVariableNames([]string{"cycle", "period_ms"})
		timingstats.Save(exp.Data, "clearonly cycle_period (app-side — NOT a pass/fail signal)", s, s.Mean)
	}
	// The verdict is what the screen did, so the file records the criteria
	// rather than an answer — whoever watched it has to supply that.
	out.Println("\nPASS = the screen showed a clean square wave throughout.")
	out.Println("FAIL = frozen frames lasting seconds, or flicker unrelated to the pattern.")
}
