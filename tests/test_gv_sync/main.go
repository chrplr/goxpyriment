// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// .gv playback synchronisation test.
//
// Measures whether stimuli.PlayGvFunc puts a video frame on screen when it says
// it does. It synthesises a .gv flash train (bright square on a dark screen),
// plays it with PlayGvFunc, and fires a TTL pulse from the playback callback at
// the onset of every bright phase.
//
// A photodiode on the square and the TTL line on a second channel give the
// quantity of interest: the delay between the trigger edge and the actual
// luminance step, and how much that delay varies across cycles.
//
// Software timing alone cannot answer this — it only shows when goxpyriment
// *believes* the flip happened. The useful output is the recording, not the
// console summary. (The wider Timing-Tests suite exists for stimulus-timing
// questions; this test covers the media-playback path specifically.)
//
// Usage:
//
//	go run ./tests/test_gv_sync                                  # 10 cycles, 1280x720 @ 60 fps
//	go run ./tests/test_gv_sync -frames-on 6 -frames-off 24 -cycles 20
//	go run ./tests/test_gv_sync -file clip.gv                    # play an existing .gv
//	go run ./tests/test_gv_sync -keep                            # regenerate and keep the stimulus
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/tests/internal/timingstats"
	"github.com/chrplr/goxpyriment/triggers"
)

var (
	fPort       = flag.String("port", "", "Serial port for DLP-IO8-G (empty = auto-detect)")
	fTriggerPin = flag.Int("trigger-pin", 1, "DLP-IO8-G output pin (1–8)")
	fTriggerMs  = flag.Int("trigger-ms", 5, "Trigger pulse duration (ms)")
	fCycles     = flag.Int("cycles", 10, "Repetitions of the flash cycle")
	fWarmup     = flag.Int("warmup", 0, "Leading cycles excluded from statistics")
	fLevelA     = flag.Int("level-a", 0, "Dark luminance 0–255")
	fLevelB     = flag.Int("level-b", 255, "Bright luminance 0–255")
	fFPS        = flag.Float64("fps", 60, "Frame rate of the generated .gv (must divide the display refresh rate)")
	fFramesOn   = flag.Int("frames-on", 6, "Bright frames per cycle (6 @ 60 fps = 100 ms)")
	fFramesOff  = flag.Int("frames-off", 24, "Dark frames per cycle (24 @ 60 fps = 400 ms)")
	fWidth      = flag.Int("width", 1280, "Stimulus width in px")
	fHeight     = flag.Int("height", 720, "Stimulus height in px")
	fSquarePx   = flag.Int("square-px", 200, "Side of the centred bright square; 0 = fill the frame")
	fFile       = flag.String("file", "", "Play this .gv instead of generating one")
	fKeep       = flag.Bool("keep", false, "Regenerate and keep the stimulus instead of using a temp file")
	fWindowed   = flag.Bool("w", false, "Windowed mode (1024×768 window instead of fullscreen)")
	fDisplay    = flag.Int("d", -1, "Display index (-1 = primary)")
)

// setupTrigger opens the DLP-IO8-G, by explicit port or auto-detection.
// Returns a no-op device when none is found, so the test still runs (and still
// reports playback timing) without a trigger box attached.
func setupTrigger() (triggers.OutputTTLDevice, string) {
	if *fPort != "" {
		d, err := triggers.NewDLPIO8(*fPort)
		if err != nil {
			log.Printf("warning: DLP-IO8 on %s: %v — triggers disabled", *fPort, err)
			return triggers.NullOutputTTLDevice{}, ""
		}
		return d, *fPort
	}
	trig, portName, err := triggers.AutoDetectDLPIO8()
	if err != nil {
		log.Printf("warning: DLP-IO8 auto-detect: %v — triggers disabled", err)
	}
	return trig, portName
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func main() {
	flag.Parse()

	if *fWarmup >= *fCycles {
		log.Fatalf("-warmup %d discards all %d cycles; lower it or raise -cycles", *fWarmup, *fCycles)
	}

	trig, portName := setupTrigger()
	if portName != "" {
		fmt.Printf("DLP-IO8-G found on %s (trigger pin %d, pulse %d ms)\n", portName, *fTriggerPin, *fTriggerMs)
	}

	width, height, fullscreen := 0, 0, true
	if *fWindowed {
		width, height, fullscreen = 1024, 768, false
	}
	exp := control.NewExperiment("GV Sync Test", width, height, fullscreen,
		control.Black, control.White, 24)
	if *fDisplay >= 0 {
		exp.ScreenNumber = *fDisplay
	}
	if err := exp.Initialize(); err != nil {
		exp.End()
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	if err := run(exp, trig); err != nil {
		log.Fatalf("test error: %v", err)
	}
}

func run(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	spec := gvStimSpec{
		Width:     *fWidth,
		Height:    *fHeight,
		FPS:       *fFPS,
		FramesOn:  *fFramesOn,
		FramesOff: *fFramesOff,
		Cycles:    *fCycles,
		SquarePx:  *fSquarePx,
		Bright:    byte(*fLevelB),
		Dark:      byte(*fLevelA),
	}

	// The refresh rate must be an integer multiple of the file's rate or PlayGv
	// refuses the clip; say so here rather than after generating it.
	refresh := float64(time.Second) / float64(exp.Screen.FrameDuration())
	onMs := float64(spec.FramesOn) / spec.FPS * 1000
	offMs := float64(spec.FramesOff) / spec.FPS * 1000

	fmt.Printf("gvsync: %dx%d @ %g fps  square=%dpx  on=%d frames (%.1f ms)  off=%d frames (%.1f ms)  cycles=%d\n",
		spec.Width, spec.Height, spec.FPS, spec.SquarePx,
		spec.FramesOn, onMs, spec.FramesOff, offMs, spec.Cycles)
	fmt.Printf("gvsync: display refresh ≈ %.2f Hz, trigger pin %d for %d ms at each bright onset\n",
		refresh, *fTriggerPin, *fTriggerMs)

	path := *fFile
	if path == "" {
		path = filepath.Join(os.TempDir(), fmt.Sprintf("gvsync_%dx%d_%gfps_%d-%d.gv",
			spec.Width, spec.Height, spec.FPS, spec.FramesOn, spec.FramesOff))
	}
	if *fKeep || !fileExists(path) {
		if err := writeGVStim(path, spec); err != nil {
			return fmt.Errorf("generating stimulus: %w", err)
		}
	}
	if st, err := os.Stat(path); err == nil {
		fmt.Printf("gvsync: stimulus %s (%d frames, %.1f MB)\n",
			path, spec.FrameCount(), float64(st.Size())/(1024*1024))
	}
	if !*fKeep && *fFile == "" {
		defer os.Remove(path)
	}

	exp.Data.WriteComment(fmt.Sprintf(
		"test=gvsync size=%dx%d fps=%g square-px=%d frames-on=%d frames-off=%d cycles=%d warmup=%d trigger-pin=%d trigger-ms=%d refresh-hz=%.3f",
		spec.Width, spec.Height, spec.FPS, spec.SquarePx,
		spec.FramesOn, spec.FramesOff, spec.Cycles, *fWarmup, *fTriggerPin, *fTriggerMs, refresh))
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

			if cycle >= *fWarmup {
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
