// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

// The `gvsync` sub-test measures whether stimuli.PlayGvFunc puts a video frame
// on screen when it says it does.
//
// It synthesises a .gv flash train (bright square / dark screen), plays it with
// PlayGvFunc, and fires a TTL pulse from the callback at the onset of every
// bright phase. A photodiode on the square and the TTL line on a second channel
// give the quantity of interest: the delay between the trigger edge and the
// actual luminance step, and how much that delay varies.
//
// Software timing alone cannot answer this — it only shows when goxpyriment
// *believes* the flip happened. That is why the useful output is the recording,
// not the console summary.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/tests/internal/timingstats"
	"github.com/chrplr/goxpyriment/triggers"
)

// gvSyncDefaultCycles is the repetition count when -cycles is not given.
// The shared default (120) makes for a 60 s recording; 10 cycles is enough to
// read the alignment off a scope trace.
const gvSyncDefaultCycles = 10

// flagWasSet reports whether the user passed a flag explicitly, so a per-test
// default can be applied without overriding an explicit choice.
func flagWasSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func runGvSync(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	cycles := *fCycles
	if !flagWasSet("cycles") {
		cycles = gvSyncDefaultCycles
	}
	// The shared -warmup default (10) would discard every cycle of a 10-cycle
	// run. The first cycle contributes no period anyway, so nothing needs
	// discarding here by default.
	warmup := *fWarmup
	if !flagWasSet("warmup") {
		warmup = 0
	}
	if warmup >= cycles {
		return fmt.Errorf("gvsync: -warmup %d discards all %d cycles; lower it or raise -cycles", warmup, cycles)
	}

	spec := gvStimSpec{
		Width:     *fGvWidth,
		Height:    *fGvHeight,
		FPS:       *fGvFPS,
		FramesOn:  *fGvFramesOn,
		FramesOff: *fGvFramesOff,
		Cycles:    cycles,
		SquarePx:  *fGvSquarePx,
		Bright:    byte(*fLevelB),
		Dark:      byte(*fLevelA),
	}

	// The refresh rate must be an integer multiple of the file's rate or
	// PlayGv refuses the clip; say so here rather than after generating it.
	refresh := float64(time.Second) / float64(exp.Screen.FrameDuration())
	onMs := float64(spec.FramesOn) / spec.FPS * 1000
	offMs := float64(spec.FramesOff) / spec.FPS * 1000

	fmt.Printf("gvsync: %dx%d @ %g fps  square=%dpx  on=%d frames (%.1f ms)  off=%d frames (%.1f ms)  cycles=%d\n",
		spec.Width, spec.Height, spec.FPS, spec.SquarePx,
		spec.FramesOn, onMs, spec.FramesOff, offMs, spec.Cycles)
	fmt.Printf("gvsync: display refresh ≈ %.2f Hz, trigger pin %d for %d ms at each bright onset\n",
		refresh, *fTriggerPin, *fTriggerMs)

	path := *fGvFile
	if path == "" {
		path = filepath.Join(os.TempDir(), fmt.Sprintf("gvsync_%dx%d_%gfps_%d-%d.gv",
			spec.Width, spec.Height, spec.FPS, spec.FramesOn, spec.FramesOff))
	}
	if *fGvKeep || !fileExists(path) {
		if err := writeGVStim(path, spec); err != nil {
			return fmt.Errorf("gvsync: generating stimulus: %w", err)
		}
	}
	if st, err := os.Stat(path); err == nil {
		fmt.Printf("gvsync: stimulus %s (%d frames, %.1f MB)\n",
			path, spec.FrameCount(), float64(st.Size())/(1024*1024))
	}
	if !*fGvKeep && *fGvFile == "" {
		defer os.Remove(path)
	}

	exp.Data.WriteComment(fmt.Sprintf(
		"test=gvsync size=%dx%d fps=%g square-px=%d frames-on=%d frames-off=%d cycles=%d warmup=%d trigger-pin=%d trigger-ms=%d refresh-hz=%.3f",
		spec.Width, spec.Height, spec.FPS, spec.SquarePx,
		spec.FramesOn, spec.FramesOff, spec.Cycles, warmup, *fTriggerPin, *fTriggerMs, refresh))
	exp.AddDataVariableNames([]string{
		"cycle", "frame", "onset_ms", "period_ms", "trigger_lag_us",
	})

	var periods []float64
	var lags []float64
	var prevOnsetNS uint64
	cycle := 0

	err := exp.Run(func() error {
		_, logs, err := stimuli.PlayGvFunc(exp.Screen, path, 0, 0, func(ctx stimuli.GvFrameContext) error {
			if !spec.IsOnset(ctx.Frame) {
				return nil
			}

			// control.TicksNS shares its origin with ctx.OnsetNS. Do not use
			// clock.GetTimeNS here: it counts from a different origin, so
			// subtracting the two would be meaningless.
			fired := control.TicksNS()

			// Raise the line as close to the flip as possible. FireTrigger runs
			// on its own goroutine because it holds the line high for
			// -trigger-ms, which would otherwise stall this callback and delay
			// the next frame. Only one fires at a time (they are a full cycle
			// apart), so the device is never driven concurrently.
			go triggers.FireTrigger(trig, *fTriggerPin, time.Duration(*fTriggerMs)*time.Millisecond)

			// Software-side lag: flip timestamp to the moment we dispatched the
			// trigger. This is a lower bound on the true electrical delay and
			// does not include the device's own latency — the photodiode
			// recording is what measures that.
			lagUS := float64(fired-ctx.OnsetNS) / 1000

			var periodMs float64
			if prevOnsetNS > 0 {
				periodMs = float64(ctx.OnsetNS-prevOnsetNS) / 1e6
			}
			prevOnsetNS = ctx.OnsetNS

			if cycle >= warmup {
				lags = append(lags, lagUS)
				if periodMs > 0 {
					periods = append(periods, periodMs)
				}
			}
			exp.Data.Add(cycle, ctx.Frame,
				fmt.Sprintf("%.3f", float64(ctx.OnsetNS)/1e6),
				fmt.Sprintf("%.3f", periodMs),
				fmt.Sprintf("%.1f", lagUS))
			cycle++
			return nil
		})
		if err != nil {
			return err
		}

		// PlayGv reports, per frame, how many display refreshes elapsed beyond
		// the expected hold. Anything non-zero means a frame was shown late and
		// the photodiode trace will contain a correspondingly stretched phase.
		skipped, worst := 0, 0
		for _, l := range logs {
			skipped += l.SkippedFrames
			if l.SkippedFrames > worst {
				worst = l.SkippedFrames
			}
		}
		fmt.Printf("\nplayback: %d frames, %d skipped display frames (worst single frame: %d)\n",
			len(logs), skipped, worst)
		if skipped > 0 {
			fmt.Printf("  WARNING: frames were dropped; onsets in the recording will not be evenly spaced.\n")
		}
		return control.EndLoop
	})
	if err != nil && !control.IsEndLoop(err) {
		return err
	}

	expectedPeriodMs := float64(spec.FramesOn+spec.FramesOff) / spec.FPS * 1000
	sPer := timingstats.ComputeStats(periods, expectedPeriodMs)
	timingstats.PrintStats(
		fmt.Sprintf("Bright-onset period (expected %.2f ms)", expectedPeriodMs), sPer, expectedPeriodMs)

	sLag := timingstats.ComputeStats(lags, 0)
	sLag = timingstats.ComputeStats(lags, sLag.Mean)
	timingstats.PrintStats("Flip → trigger dispatch (µs, software only)", sLag, sLag.Mean)

	fmt.Printf("\nWhat to check on the recording:\n")
	fmt.Printf("  1. Photodiode step and TTL edge should coincide; their offset is the\n")
	fmt.Printf("     end-to-end presentation lag, and it should be constant across cycles.\n")
	fmt.Printf("  2. Bright phase should measure %.1f ms, period %.1f ms.\n", onMs, expectedPeriodMs)
	fmt.Printf("  3. Any cycle whose onset drifts by ≥ one frame (%.2f ms) indicates a dropped frame.\n",
		float64(time.Second)/refresh/1e6)
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
