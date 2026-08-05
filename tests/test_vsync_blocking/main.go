// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// VSYNC Blocking Test
//
// Reports what this display/driver actually does, in three numbers:
//
//   - NOMINAL — the frame period SDL derives from the current display mode.
//   - UNAIDED — the period measured presenting directly, with Update's frame
//     pacing bypassed (Screen.CalibrateRefresh). This is what SDL_RenderPresent
//     does on its own.
//   - PACED — the period measured through Screen.Update, which every stimulus
//     path uses.
//
// UNAIDED well below NOMINAL means SDL_RenderPresent does not block to the
// retrace (triple/mailbox buffering) and frames would be swallowed without
// pacing. UNAIDED well above NOMINAL means frames are being dropped before
// they reach the panel — a compositor throttling an unfocused or occluded
// window does this — which pacing cannot fix, since it enforces a minimum
// frame time, not a maximum.
//
// Also prints the display mode list, which on a variable-refresh-rate panel
// exposes the supported rate range. Pacing targets the nominal (maximum) rate,
// so on VRR it acts as a floor and never caps the frame rate.
//
// Usage:
//
//	go run ./tests/test_vsync_blocking
//	go run ./tests/test_vsync_blocking -w     # windowed mode
package main

import (
	"fmt"
	"log"
	"sort"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const nFrames = 120

func main() {
	exp := control.NewExperimentFromFlags("VSYNC Blocking Test", control.Black, control.White, 24)
	defer exp.End()

	err := exp.Run(func() error {
		nominalMs := float64(exp.Screen.FrameDuration().Nanoseconds()) / 1e6

		// UNAIDED: bypasses pacing and presents directly.
		unaided, err := exp.Screen.CalibrateRefresh(nFrames)
		if err != nil {
			return err
		}
		unaidedMs := float64(unaided.Nanoseconds()) / 1e6

		// PACED: the normal Update path every stimulus goes through.
		msg := stimuli.NewTextLine("measuring paced frames…", 0, 0, control.White)
		var paced []float64
		var prevTS uint64
		for i := 0; i < nFrames; i++ {
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := msg.Draw(exp.Screen); err != nil {
				return err
			}
			ts, err := exp.Screen.FlipTS()
			if err != nil {
				return err
			}
			if i > 5 { // discard warm-up frames
				paced = append(paced, float64(ts-prevTS)/1e6)
			}
			prevTS = ts
		}
		_ = msg.Unload()
		sort.Float64s(paced)
		pacedMs := paced[len(paced)/2]
		shortPaced := 0
		for _, v := range paced {
			if v < 0.9*nominalMs {
				shortPaced++
			}
		}

		var verdict, recommendation string
		switch {
		case unaidedMs < 0.9*nominalMs:
			verdict = "NON-BLOCKING — SDL_RenderPresent returns before the retrace."
			recommendation = "Update's frame pacing is doing real work on this system.\n" +
				"Without it, stimulus frames would be swallowed before the panel scans them out."
		case unaidedMs > 1.1*nominalMs:
			verdict = "DROPPING FRAMES — presents are arriving slower than the display refresh."
			recommendation = "Pacing cannot fix this: it enforces a minimum frame time, not a maximum.\n" +
				"Check for a compositor throttling this window (try fullscreen, and keep it focused)."
		default:
			verdict = "BLOCKING — the driver honours VSYNC on its own."
			recommendation = "Update's pacing spin exits immediately here and costs nothing."
		}

		id := sdl.GetDisplayForWindow(exp.Screen.Window)
		modeList := ""
		if modes, err := id.FullscreenDisplayModes(); err == nil && len(modes) > 0 {
			seen := map[string]bool{}
			for _, m := range modes {
				if m.W != modes[0].W || m.H != modes[0].H {
					continue
				}
				k := fmt.Sprintf("%.2f", m.RefreshRate)
				if !seen[k] {
					seen[k] = true
					modeList += k + "Hz "
				}
			}
		}

		resultText := fmt.Sprintf(
			"RESULTS (median of %d frames):\n\n"+
				"Nominal frame period : %6.3f ms  (%.2f Hz)\n"+
				"Unaided present      : %6.3f ms  (%.2f Hz)\n"+
				"Paced (Screen.Update): %6.3f ms  (%.2f Hz)\n"+
				"Short paced frames   : %d / %d\n"+
				"Rates at native size : %s\n\n"+
				"%s\n\n%s\n\n"+
				"Press any key to exit.",
			nFrames, nominalMs, 1000/nominalMs, unaidedMs, 1000/unaidedMs,
			pacedMs, 1000/pacedMs, shortPaced, len(paced), modeList,
			verdict, recommendation,
		)

		log.Printf("nominal %.3f ms | unaided %.3f ms | paced %.3f ms | short %d/%d",
			nominalMs, unaidedMs, pacedMs, shortPaced, len(paced))
		log.Printf("verdict: %s", verdict)

		tb := stimuli.NewTextBox(resultText, 900, control.Origin(), control.White)
		if err := exp.Show(tb); err != nil {
			return err
		}

		if _, err := exp.Keyboard.Wait(); err != nil {
			return err
		}
		// One measurement per run: returning nil here would send Run around the
		// loop and measure again indefinitely.
		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
