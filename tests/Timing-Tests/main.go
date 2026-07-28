// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
//
// Timing-Tests — hardware timing verification suite
//
// Six sub-tests, selected with -test <name>. `av` is the default and the one
// that matters; the rest are diagnostics for when it reports something odd.
//
// Needs no hardware:
//
//	check    Go/no-go: does this machine display and make noise at all
//	display  Frame-interval statistics and the true refresh rate
//	latency  Audio pipeline delay — how long does SDL hold PCM before it sounds
//
// Needs a photodiode and/or a trigger box:
//
//	av       THE stimulus timing test. Five squares + a synchronised tone + a
//	         TTL pulse, every cycle. -no-sound / -no-ttl drop a modality.
//	vrr      Variable-refresh sweep: 1 ms to N ms in 1 ms steps
//	rt       SDL3 event-timestamp reaction-time precision
//
// Two tests that used to live here now have their own directories:
// tests/test_gv_sync (.gv playback synchronisation) and tests/test_dlpio8
// (DLP-IO8-G square-wave characterisation).
//
// Why the console numbers are not the answer
//
// Every statistic printed below comes from software timestamps — when
// goxpyriment *believes* a flip happened. Those stayed textbook-perfect
// throughout a presentation bug that left the panel showing stale frames for
// seconds at a time. Judge timing from the photodiode and TTL recording; treat
// the console output as a cross-check on the software side only.
//
// Usage:
//
//	go run ./tests/Timing-Tests                          # av, the default
//	go run ./tests/Timing-Tests -frames-on 12 -frames-off 18 -cycles 100
//	go run ./tests/Timing-Tests -no-sound -no-ttl        # visual only, no hardware
//	go run ./tests/Timing-Tests -test display
//
// Common flags:
//
//	-test string      Sub-test to run (default "av")
//	-port string      Serial port for DLP-IO8-G (default: auto-detect)
//	-trigger-pin int  Output pin on DLP-IO8-G (default 1)
//	-trigger-ms int   Trigger pulse duration in ms (default 5)
//	-cycles int       Number of cycles (default 120)
//	-warmup int       Leading cycles discarded from statistics (default 10)
//	-w                Windowed mode: 1024×768 window instead of fullscreen
//	-d int            Display index: monitor to use (-1 = primary)
//	-sysinfo          Print system information and exit
//	-outdir string    Where to write the .csv/-info.txt results
//	                  (default $HOME/goxpy_data)
//	-gc               Leave the garbage collector running during timing loops.
//	                  Off by default. Run a test twice, with and without, to
//	                  measure the collector's effect.
//
// Per-test flags — av:
//
//	-frames-on int    Bright frames per cycle (default 12 = 200 ms at 60 Hz)
//	-frames-off int   Dark frames per cycle (default 18 = 300 ms at 60 Hz)
//	-square-px int    Side of each of the five squares, renderer px (default 200)
//	-level-a int      Dark luminance 0–255 (default 0)
//	-level-b int      Bright luminance 0–255 (default 255)
//	-soa-ms float     Visual-to-audio SOA in ms; negative = audio first (default 0)
//	-freq-hz float    Tone frequency in Hz (default 1000)
//	-hz float         Refresh rate used to derive the tone duration (default 60)
//	-no-sound         Do not play the tone
//	-no-ttl           Do not fire the TTL trigger
//	-paced-flip       Use PacedFlip() instead of Update()
//	-audio-frames int Audio hardware buffer, sample frames (0 = SDL default).
//	                  Sets the floor on audio-onset quantisation: 256 frames at
//	                  44100 Hz quantises tone onsets to 5.8 ms steps.
//
// Per-test flags — display:  -duration-s float (default 10)
// Per-test flags — latency:  -freq-hz float, -drain-reps int (default 10)
// Per-test flags — vrr:      -vrr-max-ms int (default 50), -cycles
// Per-test flags — rt:       -iti-ms float, mean ITI jittered ±50 % (default 1000)
//
// Placing the photodiodes
//
// The five squares sit at the four corners and the centre. An LCD scans out
// top-to-bottom, so a bottom square lags a top one by close to a full frame
// period — put a photodiode on a top square and another on a bottom one and the
// difference is your panel's scan-out gradient, in µs per pixel row. Two squares
// on the same row are only microseconds apart and serve as a sanity check.
//
// See docs/TimingTests.md for equipment setup and interpretation.

package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/sysinfo"
	"github.com/chrplr/goxpyriment/tests/internal/timingstats"
	"github.com/chrplr/goxpyriment/triggers"
)

// ── Flags ─────────────────────────────────────────────────────────────────────

var (
	fTest        = flag.String("test", "av", "Sub-test: av | vrr | rt | check | display | latency")
	fPort        = flag.String("port", "", "Serial port for DLP-IO8-G (empty = auto-detect)")
	fTriggerPin  = flag.Int("trigger-pin", 1, "DLP-IO8-G output pin (1–8)")
	fTriggerMs   = flag.Int("trigger-ms", 5, "Trigger pulse duration (ms)")
	fCycles      = flag.Int("cycles", 120, "Number of cycles [av / vrr / rt]")
	fLevelA      = flag.Int("level-a", 0, "Dark luminance 0–255 (surround) [av / vrr]")
	fLevelB      = flag.Int("level-b", 255, "Bright luminance 0–255 (squares) [av / vrr]")
	fFramesOn    = flag.Int("frames-on", 12, "Bright frames per cycle (12 = 200 ms at 60 Hz) [av]")
	fFramesOff   = flag.Int("frames-off", 18, "Dark frames per cycle (18 = 300 ms at 60 Hz) [av]")
	fSoaMs       = flag.Float64("soa-ms", 0, "Visual-to-audio SOA ms; negative = audio first [av]")
	fItiMs       = flag.Float64("iti-ms", 1000, "Mean inter-trial interval ms, jittered ±50 % [rt]")
	fFreqHz      = flag.Float64("freq-hz", 1000, "Tone frequency Hz [av / latency]")
	fDurationS   = flag.Float64("duration-s", 10, "Measurement duration in seconds [display]")
	fAudioFrames = flag.Int("audio-frames", 0, "Audio hardware buffer size in sample frames (0=SDL default). Must be set before SDL audio opens; e.g. 256, 512, 1024.")
	fHz          = flag.Float64("hz", 60.0, "Expected display refresh rate in Hz; sets the tone duration (frames-on × 1/hz) [av]")
	fWarmup      = flag.Int("warmup", 10, "Leading cycles discarded from statistics [av / display]")
	fDrainReps   = flag.Int("drain-reps", 10, "Repetitions per tone duration [latency]")
	fVRRMaxMs    = flag.Int("vrr-max-ms", 50, "Maximum stimulus duration to sweep, in 1 ms steps [vrr]")
	fWindowed    = flag.Bool("w", false, "Windowed mode (1024×768 window instead of fullscreen)")
	fDisplay     = flag.Int("d", -1, "Display index: monitor where the window/fullscreen will open (-1 = primary)")
	fSysInfo     = flag.Bool("sysinfo", false, "Print system information and exit")
	fOutDir      = flag.String("outdir", "", "Directory for the .csv/-info.txt results (default: $HOME/goxpy_data).\n\tUse it to keep a session's data files beside its other outputs.")
	fPacedFlip   = flag.Bool("paced-flip", false, "Use PacedFlip() instead of Update() for frame pacing [av]")
	fGC          = flag.Bool("gc", false, "Leave the garbage collector RUNNING during timing-critical loops.\n\tBy default the collector is suspended; pass -gc to measure its effect on timing\n\t(run the same test twice, with and without, to obtain the comparison).")
	fSquarePx    = flag.Int("square-px", 0, "Side of each of the five stimulus squares, in renderer pixels;\n\t0 = one quarter of the render height, which keeps each square's centre\n\tclear of the bezel and fixes the top↔bottom separation at 0.750 [av / vrr]")
	fNoSound     = flag.Bool("no-sound", false, "Do not play the tone [av]")
	fNoTTL       = flag.Bool("no-ttl", false, "Do not fire the TTL trigger [av]")
)

// ── Garbage-collector control ─────────────────────────────────────────────────

// suspendGC turns the garbage collector off for the duration of a
// timing-critical loop and returns the function that restores it.
func suspendGC() func() {
	if *fGC {
		return func() {}
	}
	oldGC := debug.SetGCPercent(-1)
	return func() { debug.SetGCPercent(oldGC) }
}

// gcLabel describes the collector state, for report headers and data comments.
func gcLabel() string {
	if *fGC {
		return "on"
	}
	return "suspended"
}

// ── Screen fill helper ─────────────────────────────────────────────────────────

// gray builds an opaque gray from a single 0–255 level.
func gray(level byte) control.Color { return control.RGB(level, level, level) }

// paintFunc renders one stimulus frame into the backbuffer: fg is the stimulus
// colour, bg the surround. It does not present — the caller flips.
type paintFunc func(fg, bg control.Color)

// newPainter builds the per-frame paint function shared by every visual test.
//
// It paints five squares of -square-px side — one at each screen corner and one
// centred — over a bg-coloured surround. Placing a photodiode on each square
// measures the delay between screen regions: an LCD scans out top-to-bottom, so
// the bottom squares lag the top ones by close to a full frame period. (The two
// squares on a given row are only microseconds apart, being on the same
// scanline; they serve as a sanity check rather than a measurement.)
//
// The rectangles are laid out once, here, rather than per frame: the geometry
// never changes, and the logical-size query is a CGo call that has no business
// inside a VSYNC-locked loop.
//
// Coordinates are the renderer's own — the logical presentation size when one is
// set — so -square-px is in renderer pixels and the corners stay at the corners
// under HiDPI scaling. A square's *physical* size therefore varies with the
// logical/physical scale factor, which matters when comparing photodiode signal
// amplitude across video drivers or displays.
func newPainter(exp *control.Experiment) (paintFunc, error) {
	r := exp.Screen.Renderer

	// Drawing happens in the logical presentation space when one is set; fall
	// back to the raw output size when it is not.
	w, h, _, err := r.LogicalPresentation()
	if err != nil || w <= 0 || h <= 0 {
		w, h, err = exp.Screen.Size()
		if err != nil {
			return nil, fmt.Errorf("newPainter: querying render size: %w", err)
		}
	}

	fw, fh := float32(w), float32(h)

	// Default the square to a quarter of the render height rather than a fixed
	// pixel count. That size scales with the panel, is large enough on any
	// display for a photodiode to sit at the square's centre well clear of the
	// bezel — and, because the centres then land at H/8 and 7H/8, it fixes the
	// top↔bottom separation at exactly 0.750 of screen height on every display,
	// so the figure the gradient is divided by no longer varies per machine.
	side := float32(*fSquarePx)
	sideAuto := ""
	if *fSquarePx <= 0 {
		side = fh / 4
		sideAuto = " (auto: ¼ of render height)"
	}
	if side*2 > fw || side*2 > fh {
		return nil, fmt.Errorf("newPainter: a %.0f px square does not fit a %dx%d render area "+
			"(corner squares would overlap)", side, w, h)
	}

	rects := []control.FRect{
		{X: 0, Y: 0, W: side, H: side},                             // top-left
		{X: fw - side, Y: 0, W: side, H: side},                     // top-right
		{X: 0, Y: fh - side, W: side, H: side},                     // bottom-left
		{X: fw - side, Y: fh - side, W: side, H: side},             // bottom-right
		{X: (fw - side) / 2, Y: (fh - side) / 2, W: side, H: side}, // centre
	}
	// Report the geometry rather than leaving it to be reconstructed later. The
	// scan-out gradient is (bottom onset − top onset) divided by the separation
	// below, so that fraction is part of the measurement, not a detail — and
	// recomputing it after the fact is exactly how a run gets misread.
	//
	// The separation holds only if each photodiode sits at its square's CENTRE.
	// A small square puts that centre close to the bezel, where a diode cannot
	// lie flat; it then ends up further out, the true separation exceeds the
	// figure below, and the derived sweep time comes out too large — possibly
	// larger than a frame period, which is the tell that this has happened.
	// The default of a quarter of the render height keeps the centres clear of
	// the edge on any panel; -square-px overrides it when you need a specific
	// size.
	sepPx := fh - side
	sepFrac := float64(sepPx) / float64(fh)
	cxL, cxR, cxC := side/2, fw-side/2, fw/2
	cyT, cyB, cyC := side/2, fh-side/2, fh/2

	fmt.Printf("stimulus: five %.0f px squares%s (corners + centre) on a %.0fx%.0f render area\n",
		side, sideAuto, fw, fh)
	fmt.Printf("  centres:  x = %.0f / %.0f / %.0f    y = %.0f / %.0f / %.0f\n",
		cxL, cxC, cxR, cyT, cyC, cyB)
	fmt.Printf("  top↔bottom separation: %.0f px = %.4f of screen height\n", sepPx, sepFrac)
	fmt.Printf("  place each photodiode at its square's CENTRE — the separation assumes it\n")

	if exp.Data != nil {
		exp.Data.WriteComment(fmt.Sprintf("stim square-px: %.0f", side))
		exp.Data.WriteComment(fmt.Sprintf("stim render-area: %.0fx%.0f", fw, fh))
		exp.Data.WriteComment(fmt.Sprintf("stim square-centres-x: %.0f,%.0f,%.0f", cxL, cxC, cxR))
		exp.Data.WriteComment(fmt.Sprintf("stim square-centres-y: %.0f,%.0f,%.0f", cyT, cyC, cyB))
		exp.Data.WriteComment(fmt.Sprintf("stim top-bottom-separation-px: %.0f", sepPx))
		exp.Data.WriteComment(fmt.Sprintf("stim top-bottom-separation-frac: %.4f", sepFrac))
	}

	return func(fg, bg control.Color) {
		r.SetDrawColor(bg.R, bg.G, bg.B, 255)
		r.Clear()
		r.SetDrawColor(fg.R, fg.G, fg.B, 255)
		for i := range rects {
			_ = r.RenderFillRect(&rects[i])
		}
	}, nil
}

// fillGray paints one stimulus frame at a uniform gray level (0–255) and
// presents it. Returns the time just before and just after RenderPresent (the
// VSYNC wait), in milliseconds with sub-millisecond precision.
func fillGray(exp *control.Experiment, paint paintFunc, level byte) (tBefore, tAfter float64) {
	paint(gray(level), gray(byte(*fLevelA)))
	tBefore = float64(clock.GetTimeNS()) / 1e6
	exp.Screen.Update() // blocks until VSYNC
	tAfter = float64(clock.GetTimeNS()) / 1e6
	return
}

// ── Trigger setup ──────────────────────────────────────────────────────────────

func setupTrigger() (triggers.OutputTTLDevice, string) {
	var trig triggers.OutputTTLDevice
	var portName string
	var err error

	if *fPort != "" {
		d, openErr := triggers.NewDLPIO8(*fPort)
		if openErr != nil {
			log.Printf("warning: DLP-IO8 on %s: %v — triggers disabled", *fPort, openErr)
			trig = triggers.NullOutputTTLDevice{}
		} else {
			trig, portName = d, *fPort
		}
	} else {
		trig, portName, err = triggers.AutoDetectDLPIO8()
		if err != nil {
			log.Printf("warning: DLP-IO8 auto-detect: %v — triggers disabled", err)
		}
	}
	if portName != "" {
		fmt.Printf("DLP-IO8-G found on %s (trigger pin %d, pulse %d ms)\n",
			portName, *fTriggerPin, *fTriggerMs)
	}
	return trig, portName
}

// printStatsVsMean reports a series against its own measured mean rather than a
// target derived from -hz. ComputeStats must run twice: the first pass exists
// only to obtain the mean that the second pass uses as the deviation reference.
func printStatsVsMean(label string, xs []float64) {
	if len(xs) == 0 {
		fmt.Printf("\n%s: no samples\n", label)
		return
	}
	s := timingstats.ComputeStats(xs, 0)
	s = timingstats.ComputeStats(xs, s.Mean)
	timingstats.PrintStats(label, s, s.Mean)
}

// ── Test: av ──────────────────────────────────────────────────────────────────

// runAV is the suite's primary stimulus-timing test. By default every cycle
// presents all three modalities at once:
//
//   - visual: five squares (four corners + centre) go bright for -frames-on
//     frames, then the screen is dark for -frames-off frames
//   - audio:  a tone of frames-on × frame-period, synchronised to the visual
//     onset (or offset from it by -soa-ms)
//   - TTL:    a -trigger-ms pulse at the visual onset
//
// -no-sound and -no-ttl drop a modality, which is how this test covers what used
// to be three separate ones: -no-sound is a pure visual-onset test, and the audio
// channel of a recording gives tone-onset jitter over a long session.
//
// Put a photodiode on one square and the trigger on a second recorder channel;
// the quantity of interest is the offset between them and its stability. Software
// timestamps alone cannot answer that — they only show when goxpyriment
// *believes* the flip happened, which stayed textbook-perfect throughout a
// presentation bug that left the panel showing stale frames for seconds. The
// recording is the ground truth; the console statistics below are a cross-check.
//
// Two squares at different heights measure the panel's scan-out gradient: an LCD
// draws top-to-bottom, so a bottom square lags a top one by close to a full frame
// period, and a stimulus's onset therefore depends on where it sits on screen.
func runAV(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	framesOn, framesOff := *fFramesOn, *fFramesOff
	frameMs := 1000.0 / *fHz
	toneDurMs := int(math.Round(float64(framesOn) * frameMs))

	withSound := !*fNoSound
	withTTL := !*fNoTTL
	if _, isNull := trig.(triggers.NullOutputTTLDevice); isNull && withTTL {
		fmt.Println("av: no DLP-IO8-G found — running without triggers")
		withTTL = false
	}

	modalities := "visual"
	if withSound {
		modalities += "+audio"
	}
	if withTTL {
		modalities += "+ttl"
	}
	syncMethod := "goroutine"
	if *fSoaMs == 0 {
		syncMethod = "PlaySyncedWithFlip"
	}

	fmt.Printf("av: %s  frames-on=%d (%.1f ms) frames-off=%d (%.1f ms)  cycles=%d  warmup=%d\n",
		modalities, framesOn, float64(framesOn)*frameMs, framesOff, float64(framesOff)*frameMs,
		*fCycles, *fWarmup)
	if withSound {
		fmt.Printf("av: tone %.0f Hz for %d ms  soa=%.1f ms  sync=%s\n",
			*fFreqHz, toneDurMs, *fSoaMs, syncMethod)
	}

	if *fWarmup >= *fCycles {
		return fmt.Errorf("av: -warmup %d discards all %d cycles; lower it or raise -cycles",
			*fWarmup, *fCycles)
	}

	exp.Data.WriteComment(fmt.Sprintf(
		"test=av level-a=%d level-b=%d square-px=%d frames-on=%d frames-off=%d cycles=%d warmup=%d hz=%.3f sound=%v ttl=%v soa-ms=%.1f freq-hz=%.0f",
		*fLevelA, *fLevelB, *fSquarePx, framesOn, framesOff, *fCycles, *fWarmup, *fHz,
		withSound, withTTL, *fSoaMs, *fFreqHz))
	exp.AddDataVariableNames([]string{
		"cycle",
		"t_visual_before_ms", "t_visual_after_ms",
		"bright_duration_ms", "period_ms",
		"t_audio_queued_ms", "soa_intended_ms", "soa_actual_ms",
	})

	var tone *stimuli.Tone
	if withSound {
		tone = stimuli.NewTone(*fFreqHz, toneDurMs, 0.8)
		if err := tone.PreloadDevice(exp.AudioDevice); err != nil {
			return fmt.Errorf("av: preload tone: %w", err)
		}
		defer tone.Unload()
	}

	soaDur := time.Duration(math.Abs(*fSoaMs) * float64(time.Millisecond))
	audioFirst := *fSoaMs < 0

	var brightDurations, periods, soaActuals []float64
	var prevBrightStart float64

	return exp.Run(func() error {
		restoreGC := suspendGC()
		defer restoreGC()

		flip := exp.Screen.Update
		if *fPacedFlip {
			flip = exp.Screen.PacedFlip
		}

		paint, err := newPainter(exp)
		if err != nil {
			return err
		}
		bright, dark := gray(byte(*fLevelB)), gray(byte(*fLevelA))

		// fire pulses the trigger on its own goroutine: FireTrigger holds the
		// line high for -trigger-ms, which would otherwise stall the frame loop.
		fire := func() {
			if withTTL {
				go triggers.FireTrigger(trig, *fTriggerPin, time.Duration(*fTriggerMs)*time.Millisecond)
			}
		}

		// showFrame paints one frame and presents it. PumpEvents runs every
		// frame, not once per cycle: under a compositor the backend must be
		// serviced on the same cadence as the flips.
		showFrame := func(fg control.Color) {
			paint(fg, dark)
			flip()
			control.PumpEvents()
		}

		// holdBright keeps the bright squares up for the remaining frames of the
		// phase, redrawing each one so double-buffering never flips dark into view.
		holdBright := func(remaining int) {
			for f := 0; f < remaining; f++ {
				showFrame(bright)
			}
		}

		for cycle := 0; cycle < *fCycles; cycle++ {
			var tVisB, tVisA, tAudioQ float64

			switch {
			case withSound && audioFirst:
				// Audio leads: queue the tone, wait out the SOA, then flip.
				tAudioQ = float64(clock.GetTimeNS()) / 1e6
				_ = tone.Play()
				time.Sleep(soaDur)
				paint(bright, dark)
				tVisB = float64(clock.GetTimeNS()) / 1e6
				flip()
				tVisA = float64(clock.GetTimeNS()) / 1e6
				control.PumpEvents()
				fire()
				holdBright(framesOn - 1)

			case withSound && soaDur == 0:
				// SOA=0: PlaySyncedWithFlip pre-fills the audio buffer before the
				// flip and resumes the device immediately after VSYNC. This removes
				// goroutine scheduling jitter from the audio path; the onset then
				// lags by at most one callback period, which is why -audio-frames
				// sets the floor on audio-onset quantisation.
				paint(bright, dark)
				tVisB = float64(clock.GetTimeNS()) / 1e6
				flipNS, _ := tone.PlaySyncedWithFlip(exp.Screen)
				tVisA = float64(flipNS) / 1e6
				tAudioQ = tVisA
				control.PumpEvents()
				fire()
				holdBright(framesOn - 1)

			case withSound:
				// Visual leads by soaDur: schedule the tone off the visual onset
				// while holdBright keeps the screen bright concurrently.
				paint(bright, dark)
				tVisB = float64(clock.GetTimeNS()) / 1e6
				flip()
				tVisA = float64(clock.GetTimeNS()) / 1e6
				control.PumpEvents()
				fire()
				tAudioQCh := make(chan float64, 1)
				go func() {
					time.Sleep(soaDur)
					tAudioQCh <- float64(clock.GetTimeNS()) / 1e6
					_ = tone.Play()
				}()
				holdBright(framesOn - 1)
				tAudioQ = <-tAudioQCh

			default:
				// Visual only.
				paint(bright, dark)
				tVisB = float64(clock.GetTimeNS()) / 1e6
				flip()
				tVisA = float64(clock.GetTimeNS()) / 1e6
				control.PumpEvents()
				fire()
				holdBright(framesOn - 1)
			}

			// Dark phase: frames-off frames as the ITI between stimuli. Its first
			// flip ends the bright phase, so its timestamp gives the duration.
			var tDarkStart float64
			for f := 0; f < framesOff; f++ {
				showFrame(dark)
				if f == 0 {
					tDarkStart = float64(clock.GetTimeNS()) / 1e6
				}
			}

			brightMs := tDarkStart - tVisA
			var periodMs float64
			if prevBrightStart > 0 {
				periodMs = tVisA - prevBrightStart
			}
			prevBrightStart = tVisA

			if cycle >= *fWarmup {
				brightDurations = append(brightDurations, brightMs)
				if periodMs > 0 {
					periods = append(periods, periodMs)
				}
				if withSound {
					soaActuals = append(soaActuals, tAudioQ-tVisA)
				}
			}

			exp.Data.Add(cycle,
				fmt.Sprintf("%.3f", tVisB), fmt.Sprintf("%.3f", tVisA),
				fmt.Sprintf("%.3f", brightMs), fmt.Sprintf("%.3f", periodMs),
				fmt.Sprintf("%.3f", tAudioQ),
				fmt.Sprintf("%.1f", *fSoaMs), fmt.Sprintf("%.3f", tAudioQ-tVisA))

			if exp.PollEvents(nil).QuitRequested {
				return control.EndLoop
			}
		}

		// Frame counts are the authoritative unit, so the measured mean is the
		// deviation reference rather than a target derived from -hz.
		printStatsVsMean(fmt.Sprintf("Bright-phase duration (frames-on=%d)", framesOn), brightDurations)
		printStatsVsMean(fmt.Sprintf("Period (frames-on=%d + frames-off=%d)", framesOn, framesOff), periods)
		if withSound {
			printStatsVsMean(fmt.Sprintf("Audio queued − visual onset (intended SOA %.1f ms)", *fSoaMs), soaActuals)
		}

		fmt.Printf("\nav: %d cycles complete (%d discarded as warm-up).\n", *fCycles, *fWarmup)
		fmt.Println("Software timestamps only — the photodiode/TTL recording is the ground truth.")
		return control.EndLoop
	})
}

// ── Test: jitter ───────────────────────────────────────────────────────────────

// runJitter measures raw frame-interval variance by repeatedly flipping a gray screen.
func runJitter(exp *control.Experiment) error {
	fmt.Printf("display: %.1f s of frames  warmup=%d  (ESC to stop early)\n", *fDurationS, *fWarmup)

	exp.Data.WriteComment(fmt.Sprintf("test=display duration-s=%.1f warmup=%d", *fDurationS, *fWarmup))
	exp.AddDataVariableNames([]string{"frame", "t_before_ms", "t_after_ms", "interval_ms"})

	var intervals []float64
	var prevT float64

	return exp.Run(func() error {
		restoreGC := suspendGC()
		defer restoreGC()

		paint, err := newPainter(exp)
		if err != nil {
			return err
		}

		level := byte(128)
		deadline := time.Now().Add(time.Duration(*fDurationS * float64(time.Second)))
		frame := 0

		for time.Now().Before(deadline) {
			tB, tA := fillGray(exp, paint, level)

			var intervalMs float64
			if prevT > 0 {
				intervalMs = tA - prevT
				if frame >= *fWarmup {
					intervals = append(intervals, intervalMs)
				}
			}
			prevT = tA
			exp.Data.Add(frame, fmt.Sprintf("%.3f", tB), fmt.Sprintf("%.3f", tA), fmt.Sprintf("%.3f", intervalMs))
			frame++

			state := exp.PollEvents(nil)
			if state.QuitRequested {
				break
			}
		}

		// Compute stats using the measured mean as target so that >0.5 ms / >1.0 ms
		// counts reflect deviation from actual frame rate, not a hardcoded 60 Hz assumption.
		s := timingstats.ComputeStats(intervals, 16.67) // first pass to obtain mean
		estimatedHz := 0.0
		if s.Mean > 0 {
			estimatedHz = 1000.0 / s.Mean
			s = timingstats.ComputeStats(intervals, s.Mean) // recompute late counts against actual mean
		}
		fmt.Printf("\nEstimated refresh rate: %.3f Hz  (pass -hz %.2f to av so the tone matches a frame)\n",
			estimatedHz, estimatedHz)
		timingstats.PrintStats("Frame intervals", s, s.Mean)
		return control.EndLoop
	})
}

// sleepUntil sleeps until the given absolute time, with sub-millisecond
// busy-spin for the last 500 µs to reduce overshoot on Linux.
func sleepUntil(t time.Time) {
	remaining := time.Until(t)
	if remaining > 500*time.Microsecond {
		time.Sleep(remaining - 500*time.Microsecond)
	}
	for time.Now().Before(t) {
		// busy-spin
	}
}

// ── Test: rt ──────────────────────────────────────────────────────────────────

// runRT measures keyboard reaction time using SDL3 event timestamps.
//
// Each trial: a white flash appears for one frame; the participant presses any
// key as fast as possible. RT is computed as event.Timestamp − onset_ns, where
// onset_ns is the SDL nanosecond tick captured by Screen.FlipTS() immediately
// after SDL_RenderPresent returns.
//
// Because both timestamps come from the same SDL nanosecond clock (SDL_GetTicksNS),
// RT reflects the interval between actual display output and the hardware
// keyboard interrupt — without any polling latency on the response side.
//
// Use with a hardware response box connected as a USB keyboard for ground-truth
// RT validation. Compare against the photodiode onset (frames test) to obtain
// the full stimulus-onset → button-press chain in nanoseconds.
func runRT(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	nTrials := *fCycles
	meanItiMs := *fItiMs
	fmt.Printf("rt: %d trials  mean ITI %.0f ms  press any key each flash\n", nTrials, meanItiMs)

	exp.Data.WriteComment(fmt.Sprintf("test=rt cycles=%d iti-ms=%.0f", nTrials, meanItiMs))
	exp.AddDataVariableNames([]string{
		"trial",
		"onset_ns", "event_ts_ns", "rt_ns", "rt_ms",
	})

	var rtValues []float64 // milliseconds for statistics

	return exp.Run(func() error {
		instructions := stimuli.NewTextLine(
			"Press any key as fast as possible when the screen flashes white.",
			0, 50, control.White,
		)
		hint := stimuli.NewTextLine("(press SPACE to start)", 0, -50, control.Gray)
		exp.Screen.Clear()
		instructions.Draw(exp.Screen)
		hint.Draw(exp.Screen)
		exp.Screen.Update()
		exp.Keyboard.WaitKey(control.K_SPACE)

		paint, err := newPainter(exp)
		if err != nil {
			return err
		}

		restoreGC := suspendGC()
		defer restoreGC()

		for i := 0; i < nTrials; i++ {
			// Jittered ITI: meanItiMs ± 50 %
			jitter := (rand.Float64() - 0.5) * meanItiMs
			itiDur := time.Duration((meanItiMs + jitter) * float64(time.Millisecond))
			exp.Screen.Clear()
			exp.Screen.Update()
			time.Sleep(itiDur)

			// Flash: paint the stimulus and flip, capturing SDL nanosecond onset.
			paint(control.White, control.Black)
			onsetNS, _ := exp.Screen.FlipTS()

			// Trigger pulse after VSYNC: pixels are now on screen.
			_, isNull := trig.(triggers.NullOutputTTLDevice)
			if !isNull {
				go triggers.FireTrigger(trig, *fTriggerPin, time.Duration(*fTriggerMs)*time.Millisecond)
			}

			// Wait for keypress — returns SDL event timestamp (nanoseconds).
			_, eventTS, err := exp.Keyboard.GetKeyEventTS(nil, 5000)
			if control.IsEndLoop(err) {
				return control.EndLoop
			}

			rtNS := int64(eventTS - onsetNS)
			rtMs := float64(rtNS) / 1e6
			rtValues = append(rtValues, rtMs)

			exp.Data.Add(i, onsetNS, eventTS, rtNS, fmt.Sprintf("%.3f", rtMs))
			fmt.Printf("trial %3d  RT = %.1f ms\n", i, rtMs)
		}

		timingstats.PrintStats("RT (ms, event-timestamp method)", timingstats.ComputeStats(rtValues, 0), 0)
		return control.EndLoop
	})
}

// ── Test: check ───────────────────────────────────────────────────────────────

// runCheck is a combined go/no-go sanity check for both display and audio.
// It shows a bright white screen for one second (verify you see a flash on the
// monitor), then plays a buzzer followed by a ping (verify you hear both
// sounds through your speakers or headphones).
// No data is recorded; this is a "does it basically work?" step before running
// any of the quantitative tests.
func runCheck(exp *control.Experiment) error {
	fmt.Println("check: verifying display and audio output — watch for a bright flash, then listen for two sounds")
	exp.Data.WriteComment("test=check")
	return exp.Run(func() error {
		// ── Step 1: bright flash on display ───────────────────────────────────
		label := stimuli.NewTextLine("DISPLAY CHECK — you should see this bright screen for ~1 second.", 0, 0, control.Black)
		r := exp.Screen.Renderer
		r.SetDrawColor(255, 255, 255, 255)
		r.Clear()
		label.Draw(exp.Screen)
		exp.Screen.Update()
		time.Sleep(1 * time.Second)

		// Brief return to dark so the transition is clearly visible.
		r.SetDrawColor(0, 0, 0, 255)
		r.Clear()
		exp.Screen.Update()
		time.Sleep(300 * time.Millisecond)

		// ── Step 2: buzzer ────────────────────────────────────────────────────
		msg1 := stimuli.NewTextLine("AUDIO CHECK — listen for a buzzer…", 0, 0, control.White)
		if err := exp.Show(msg1); err != nil {
			return err
		}
		fmt.Println("check: playing buzzer…")
		if err := stimuli.PlayBuzzer(exp.AudioDevice); err != nil {
			log.Printf("check: error playing buzzer: %v", err)
		}
		clock.Wait(1000)

		// ── Step 3: ping ──────────────────────────────────────────────────────
		msg2 := stimuli.NewTextLine("AUDIO CHECK — …then a ping.", 0, 0, control.White)
		if err := exp.Show(msg2); err != nil {
			return err
		}
		fmt.Println("check: playing ping…")
		if err := stimuli.PlayPing(exp.AudioDevice); err != nil {
			log.Printf("check: error playing ping: %v", err)
		}
		clock.Wait(1000)

		fmt.Println("check: done. Did you see the bright flash and hear both sounds? If yes, proceed to the measurement tests.")
		return control.EndLoop
	})
}

// ── Test: vrr ─────────────────────────────────────────────────────────────────

// runVRR characterises Variable Refresh Rate (VRR / FreeSync / G-Sync /
// Adaptive-Sync) stimulus timing by sweeping target durations from 1 ms to
// *fVRRMaxMs in 1 ms steps, with *fCycles repetitions per step.
//
// VSync is disabled for the duration of the test (restored on exit) so that
// every SDL_RenderPresent call returns immediately without blocking for a
// VSYNC edge. On a VRR-capable display the panel dynamically adjusts its
// refresh interval to match the time between consecutive Presents, allowing
// stimuli to be shown for durations that are NOT multiples of the nominal
// frame period (e.g. 1 ms, 7 ms, 17 ms, 23 ms).
//
// At each repetition:
//  1. A bright screen is presented; onsetNS = sdl.TicksNS() is captured
//     immediately after Present() returns.
//  2. A busy-wait loop (sub-millisecond precision) holds for the target duration.
//  3. A blank screen is presented; offsetNS = sdl.TicksNS() is captured.
//  4. actual_ms = (offsetNS − onsetNS) / 1e6.
//
// If a DLP-IO8-G is available, a trigger pulse is sent at each onset, allowing
// the software timestamps to be cross-validated against a photodiode on the
// oscilloscope.
//
// Interpreting the results:
//   - On a VRR display: duration errors should be small (< 0.5 ms) across the
//     entire sweep, confirming arbitrary-duration stimulus presentation works.
//   - On a non-VRR display: duration errors cluster at multiples of the frame
//     period (±half a frame); the test self-diagnoses the absence of VRR.
//   - VRR panels have a supported refresh range (e.g. 48–144 Hz = 6.9–20.8 ms).
//     Outside this range the panel reverts to fixed-rate behaviour: errors grow
//     sharply at the boundary durations, revealing the VRR window directly from
//     the CSV data.
//
// Note: onsetNS / offsetNS are captured right after Present() returns (GPU
// submission time), not at photon emission. The full software-to-photon latency
// is a constant that can be measured independently with the frames test + a
// photodiode. Because this latency is constant, duration accuracy is not
// affected by it.
func runVRR(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	maxMs := *fVRRMaxMs
	reps := *fCycles
	_, isNull := trig.(triggers.NullOutputTTLDevice)
	trigDur := time.Duration(*fTriggerMs) * time.Millisecond

	fmt.Printf("vrr: sweep 1–%d ms in 1 ms steps  reps=%d  level-a=%d  level-b=%d",
		maxMs, reps, *fLevelA, *fLevelB)
	if !isNull {
		fmt.Printf("  trigger pin %d (%d ms pulse)", *fTriggerPin, *fTriggerMs)
	}
	fmt.Println()
	fmt.Println("vrr: disabling VSync — use a VRR-capable monitor for meaningful sub-frame durations")

	if err := exp.Screen.SetVSync(0); err != nil {
		return fmt.Errorf("vrr: could not disable VSync: %w", err)
	}
	defer func() {
		_ = exp.Screen.SetVSync(1)
		fmt.Println("vrr: VSync re-enabled")
	}()

	// Let the driver settle after the vsync change.
	time.Sleep(100 * time.Millisecond)

	exp.Data.WriteComment(fmt.Sprintf(
		"test=vrr vrr-max-ms=%d cycles=%d level-a=%d level-b=%d",
		maxMs, reps, *fLevelA, *fLevelB))
	exp.AddDataVariableNames([]string{
		"target_ms", "rep",
		"actual_ms", "duration_error_ms",
		"onset_ns", "offset_ns",
		"trigger",
	})

	return exp.Run(func() error {
		status := stimuli.NewTextLine(
			fmt.Sprintf("VRR sweep: 1–%d ms, %d reps — press ESC to stop", maxMs, reps),
			0, 0, control.White)
		if err := exp.Show(status); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)

		restoreGC := suspendGC()
		defer restoreGC()

		paint, err := newPainter(exp)
		if err != nil {
			return err
		}
		bright, dark := gray(byte(*fLevelB)), gray(byte(*fLevelA))

		var allErrors []float64

		for targetMs := 1; targetMs <= maxMs; targetMs++ {
			targetDur := time.Duration(targetMs) * time.Millisecond
			var durationErrors []float64

			for rep := 0; rep < reps; rep++ {
				// ── ISI: blank screen ────────────────────────────────────────
				paint(dark, dark)
				exp.Screen.Flip() // non-blocking with vsync=0
				time.Sleep(200 * time.Millisecond)

				// ── Onset: bright screen ─────────────────────────────────────
				if !isNull {
					_ = trig.SetHigh(*fTriggerPin)
				}
				paint(bright, dark)
				onsetNS, _ := exp.Screen.FlipTS() // returns immediately (vsync=0)

				// ── Hold for exactly targetDur using busy-wait ────────────────
				sleepUntil(time.Now().Add(targetDur))

				// ── Offset: blank screen ─────────────────────────────────────
				paint(dark, dark)
				offsetNS, _ := exp.Screen.FlipTS()

				if !isNull {
					go func() {
						time.Sleep(trigDur)
						_ = trig.SetLow(*fTriggerPin)
					}()
				}

				// ── Log ───────────────────────────────────────────────────────
				actualMs := float64(offsetNS-onsetNS) / 1e6
				durationError := actualMs - float64(targetMs)
				durationErrors = append(durationErrors, durationError)

				exp.Data.Add(
					targetMs, rep,
					fmt.Sprintf("%.3f", actualMs),
					fmt.Sprintf("%.3f", durationError),
					onsetNS, offsetNS,
					!isNull,
				)
				fmt.Printf("  %3d ms  rep %2d:  actual=%6.3f ms  error=%+6.3f ms\n",
					targetMs, rep, actualMs, durationError)

				state := exp.PollEvents(nil)
				if state.QuitRequested {
					return control.EndLoop
				}
			}

			s := timingstats.ComputeStats(durationErrors, 0)
			fmt.Printf("── %3d ms: mean=%+.3f ms  SD=%.3f ms\n", targetMs, s.Mean, s.SD)
			allErrors = append(allErrors, durationErrors...)
		}

		timingstats.PrintStats("Duration error across the whole sweep (ms)",
			timingstats.ComputeStats(allErrors, 0), 0)
		return control.EndLoop
	})
}

// ── Test: drain ───────────────────────────────────────────────────────────────

// runDrain measures audio pipeline latency without any external equipment.
//
// For each tone duration in a fixed set (25, 50, 100, 200, 500 ms) it repeats
// *fDrainReps trials.  Each trial:
//  1. Calls tone.Play() (which queues PCM data into the SDL audio stream).
//  2. Polls stream.Queued() in a tight loop until the device has consumed all
//     queued bytes (Queued returns 0).
//  3. Records drain_ms = elapsed wall-clock time from Play() to drain complete.
//
// The audio pipeline latency is drain_ms − nominal_ms.  It reflects the
// hardware-buffer delay between PutData() and the last sample exiting the DAC.
// The SD of drain_ms across reps captures trial-to-trial jitter in the audio
// scheduler — without needing a microphone or oscilloscope.
func runDrain(exp *control.Experiment) error {
	durations := []int{25, 50, 100, 200, 500} // nominal tone durations in ms
	reps := *fDrainReps
	freqHz := *fFreqHz

	fmt.Printf("drain: freq=%.0f Hz  reps=%d  durations=%v ms\n", freqHz, reps, durations)
	exp.Data.WriteComment(fmt.Sprintf("test=drain freq-hz=%.0f reps=%d durations_ms=%v",
		freqHz, reps, durations))
	exp.AddDataVariableNames([]string{
		"duration_ms", "rep", "drain_ms", "overhead_ms",
	})

	return exp.Run(func() error {
		status := stimuli.NewTextLine(
			fmt.Sprintf("Audio drain test: %.0f Hz tone, %d reps — please wait…", freqHz, reps),
			0, 0, control.White)
		if err := exp.Show(status); err != nil {
			return err
		}

		restoreGC := suspendGC()
		defer restoreGC()

		for _, durMs := range durations {
			tone := stimuli.NewTone(freqHz, durMs, 0.8)
			if err := tone.PreloadDevice(exp.AudioDevice); err != nil {
				return fmt.Errorf("drain: preload tone %d ms: %w", durMs, err)
			}

			var drainVals []float64
			for rep := 0; rep < reps; rep++ {
				// Brief silence between reps so stream is fully empty before Play().
				time.Sleep(50 * time.Millisecond)

				tPlay := time.Now()
				_ = tone.Play()

				// Spin-poll until the device has consumed all queued bytes.
				for {
					queued, err := tone.Stream.Queued()
					if err != nil || queued <= 0 {
						break
					}
					time.Sleep(500 * time.Microsecond)
				}
				drainMs := float64(time.Since(tPlay).Nanoseconds()) / 1e6
				overheadMs := drainMs - float64(durMs)
				drainVals = append(drainVals, drainMs)

				exp.Data.Add(
					durMs, rep,
					fmt.Sprintf("%.3f", drainMs),
					fmt.Sprintf("%.3f", overheadMs),
				)
				fmt.Printf("  %3d ms  rep %2d:  drain=%.1f ms  overhead=%+.1f ms\n",
					durMs, rep, drainMs, overheadMs)

				state := exp.PollEvents(nil)
				if state.QuitRequested {
					tone.Unload()
					return control.EndLoop
				}
			}

			tone.Unload()
			// Report drain_ms statistics with nominal duration as the target.
			// mean − target = audio pipeline latency; SD = drain-time jitter.
			s := timingstats.ComputeStats(drainVals, float64(durMs))
			fmt.Printf("\n")
			timingstats.PrintStats(fmt.Sprintf("Drain time for %d ms tone (latency = mean − target)", durMs),
				s, float64(durMs))
			fmt.Printf("  pipeline latency ≈ %.1f ms\n", s.Mean-float64(durMs))
		}

		return control.EndLoop
	})
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	// Parse flags early so we can act on -audio-frames before SDL opens the
	// audio device inside NewExperimentFromFlags. flag.Parse() is idempotent;
	// NewExperimentFromFlags will call it again harmlessly.
	flag.Parse()
	if *fSysInfo {
		sysinfo.Collect().Print()
		return
	}
	if *fAudioFrames > 0 {
		control.SetAudioSampleFrames(*fAudioFrames)
		fmt.Printf("audio: requesting %d sample frames hardware buffer\n", *fAudioFrames)
	}

	width, height, fullscreen := 0, 0, true
	if *fWindowed {
		width, height, fullscreen = 1024, 768, false
	}
	exp := control.NewExperiment("Timing-Tests", width, height, fullscreen, control.Black, control.White, 24)
	if *fDisplay >= 0 {
		exp.ScreenNumber = *fDisplay
	}
	// Must precede Initialize: that is where the data file is created.
	if *fOutDir != "" {
		exp.SetOutputDirectory(*fOutDir)
	}
	if err := exp.Initialize(); err != nil {
		exp.End() // release any SDL subsystems already initialised before exiting
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// Handle Ctrl-C (SIGINT) and SIGTERM so the process exits cleanly.
	// Only save data here — do NOT call exp.End() (which calls sdl.Quit via
	// CGo) from this goroutine while the main goroutine may be inside an SDL
	// CGo call.  Concurrent SDL access from two OS threads causes a SIGSEGV.
	// os.Exit skips deferred functions, so SDL is never touched from here.
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		if exp.Data != nil {
			exp.Data.WriteEndTime()
			if err := exp.Data.Save(); err == nil {
				log.Printf("Results saved in %s", exp.Data.FullPath)
			}
		}
		os.Exit(0)
	}()

	// Log actual audio device format so the user can verify the buffer size.
	if spec, frames, err := exp.AudioDevice.Format(); err == nil {
		fmt.Printf("audio: %d Hz  %d ch  %d sample frames (~%.1f ms latency)\n",
			spec.Freq, spec.Channels, frames,
			float64(frames)/float64(spec.Freq)*1000)
	}

	// Record the collector state in both the console report and the data-file
	// header, so GC-on and GC-off runs cannot be confused during analysis.
	fmt.Printf("gc: %s during timing-critical loops\n", gcLabel())
	if exp.Data != nil {
		exp.Data.WriteComment("gc=" + gcLabel())
	}

	trig, _ := setupTrigger()
	defer trig.Close()

	var runErr error
	switch *fTest {
	// ── No hardware required ─────────────────────────────────────────────────
	case "check":
		runErr = runCheck(exp)
	case "display":
		runErr = runJitter(exp)
	case "latency":
		runErr = runDrain(exp)
	// ── Photodiode / trigger box required ────────────────────────────────────
	case "av":
		runErr = runAV(exp, trig)
	case "vrr":
		runErr = runVRR(exp, trig)
	case "rt":
		runErr = runRT(exp, trig)
	default:
		exp.End() // release any SDL subsystems already initialised before exiting
		log.Fatalf("unknown test %q — choose from: av vrr rt check display latency\n"+
			"  (gvsync moved to tests/test_gv_sync; trigger moved to tests/test_dlpio8)", *fTest)
	}

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("test error: %v", runErr)
	}
}
