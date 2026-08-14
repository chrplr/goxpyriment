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
		//
		// The pacing tallies are reset here and read below. They are the
		// verdict, not the medians: pacing exists precisely to make the median
		// interval come out right, so comparing medians cannot detect a driver
		// that never blocks. Measured on an Intel/Mesa laptop under Wayland,
		// the three medians agreed to 3 µs while 99.7 % of presents were in
		// fact returning a mean 6.5 ms early — this test called that BLOCKING
		// until it counted the branches instead.
		exp.Screen.ResetPacingStats()
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

		// Save the intervals in the order they happened, before the sort below
		// reorders them for the median. Everything this test computes otherwise
		// goes to stdout and nowhere else, which is no use on a machine whose
		// terminal someone else is watching.
		exp.AddDataVariableNames([]string{"frame", "paced_interval_ms"})
		for i, v := range paced {
			exp.Data.Add(i, fmt.Sprintf("%.6f", v))
		}

		sort.Float64s(paced)
		pacedMs := paced[len(paced)/2]
		shortPaced := 0
		for _, v := range paced {
			if v < 0.9*nominalMs {
				shortPaced++
			}
		}

		ps := exp.Screen.PacingStats()
		totalBranches := ps.Blocked + ps.Paced
		pacedPct := 0.0
		if totalBranches > 0 {
			pacedPct = 100 * float64(ps.Paced) / float64(totalBranches)
		}

		var verdict, recommendation string
		switch {
		case unaidedMs > 1.1*nominalMs:
			verdict = "DROPPING FRAMES — presents are arriving slower than the display refresh."
			recommendation = "Pacing cannot fix this: it enforces a minimum frame time, not a maximum.\n" +
				"Check for a compositor throttling this window (try fullscreen, and keep it focused)."
		case pacedPct > 50:
			verdict = fmt.Sprintf("NON-BLOCKING — %.1f %% of presents returned early.", pacedPct)
			recommendation = "Update's frame pacing is doing real work here; without it, stimulus\n" +
				"frames would be swallowed before the panel scans them out. But it also\n" +
				"means FlipTS is reporting the SCHEDULE, not a hardware instant, so onsets\n" +
				"drift against the panel by however wrong the nominal refresh rate is.\n" +
				"Treat photodiode onsets on this machine as relative, not absolute."
		case pacedPct > 5:
			verdict = fmt.Sprintf("MOSTLY BLOCKING — but %.1f %% of presents returned early.", pacedPct)
			recommendation = "Most frames carry a hardware timestamp; a minority are stamped with\n" +
				"the schedule. Worth a photodiode check before quoting absolute onsets."
		default:
			verdict = "BLOCKING — the driver honours VSYNC on its own."
			recommendation = "Update's pacing exits immediately here and costs nothing.\n" +
				"FlipTS carries the present's own return instant on every frame."
		}

		exp.Data.WriteComment(fmt.Sprintf("vsync nominal_ms=%.5f unaided_ms=%.5f paced_ms=%.5f short=%d/%d",
			nominalMs, unaidedMs, pacedMs, shortPaced, len(paced)))
		exp.Data.WriteComment(fmt.Sprintf("vsync blocked=%d paced=%d paced_pct=%.1f wait_mean_ms=%.3f wait_max_ms=%.3f",
			ps.Blocked, ps.Paced, pacedPct,
			float64(ps.WaitMean().Nanoseconds())/1e6, float64(ps.WaitMax.Nanoseconds())/1e6))
		exp.Data.WriteComment("vsync verdict: " + verdict)

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
				"Present branches     : %d blocked / %d paced  (%.1f %% paced)\n"+
				"Early-return wait    : mean %.3f ms  max %.3f ms\n"+
				"Rates at native size : %s\n\n"+
				"%s\n\n%s\n\n"+
				"Press any key to exit.",
			nFrames, nominalMs, 1000/nominalMs, unaidedMs, 1000/unaidedMs,
			pacedMs, 1000/pacedMs, shortPaced, len(paced),
			ps.Blocked, ps.Paced, pacedPct,
			float64(ps.WaitMean().Nanoseconds())/1e6, float64(ps.WaitMax.Nanoseconds())/1e6,
			modeList,
			verdict, recommendation,
		)

		log.Printf("nominal %.3f ms | unaided %.3f ms | paced %.3f ms | short %d/%d",
			nominalMs, unaidedMs, pacedMs, shortPaced, len(paced))
		log.Printf("branches: %d blocked / %d paced (%.1f %% paced), wait mean %.3f ms max %.3f ms",
			ps.Blocked, ps.Paced, pacedPct,
			float64(ps.WaitMean().Nanoseconds())/1e6, float64(ps.WaitMax.Nanoseconds())/1e6)
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
