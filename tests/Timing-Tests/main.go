// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
//
// Timing-Tests — hardware timing verification suite
//
// Provides eleven independent sub-tests, selected with -test <name>.
// Tests are organised in four tiers; run them in this order:
//
// Tier 0 — sanity check (no equipment, no measurement):
//
//	check   Verify display flash and audio output (replaces old "audio")
//
// Tier 1 — self-contained measurements (computer only):
//
//	display Frame-interval statistics and true refresh rate  (alias: jitter)
//	latency Audio pipeline latency — how long does SDL hold PCM?  (alias: drain)
//	stream  Sequential-stimulus (RSVP) onset / duration accuracy + triggers
//	vrr     Variable Refresh Rate sweep: 1 ms to N ms in 1 ms steps
//
// Tier 2 — trigger device characterisation (DLP-IO8-G + oscilloscope):
//
//	trigger Square-wave output for DLP-IO8-G precision test  (alias: square)
//
// Tier 3 — stimulus timing validation (photodiode + oscilloscope):
//
//	frames  Alternating luminance: visual onset vs. trigger alignment
//	        (alias: flash — single-frame capability is the special case frames-on=1)
//	tones   Regular tone stream: audio onset jitter over time  (alias: sound)
//	av      Audio–visual synchrony with controllable SOA
//
// Tier 4 — response timing:
//
//	rt      SDL3 event-timestamp reaction-time precision test
//
// See README.md for full usage, equipment setup, and interpretation.
//
// Usage:
//
//	go run main.go -test <name> [flags]
//
// Common flags:
//
//	-test string      Sub-test to run (required)
//	-port string      Serial port for DLP-IO8-G (default: auto-detect)
//	-trigger-pin int  Output pin on DLP-IO8-G (default 1)
//	-trigger-ms  int  Trigger pulse duration in ms (default 5)
//	-cycles int       Number of cycles / flashes (default 120)
//	-w                Windowed mode: 1024×768 window instead of fullscreen
//	-d int            Display index: monitor to use (-1 = primary, default -1)
//	-paced-flip       Use PacedFlip() instead of Update() for frame pacing in frames/av tests
//
// Per-test flags — rt:
//
//	-iti-ms float     Mean inter-trial interval ms (jittered ±50 %; default 1000)
//
// Per-test flags — frames (alias: flash):
//
//	-level-a int      Dark luminance 0–255 (default 0)
//	-level-b int      Bright luminance 0–255 (default 255)
//	-frames-on  int   Bright frames per cycle (default 1)
//	-frames-off int   Dark frames per cycle (default 9)
//	-warmup int       Cycles discarded from statistics at start (default 10)
//
// Per-test flags — av:
//
//	-soa-ms float     Visual-to-audio SOA in ms; negative = audio first (default 0)
//	-frames-on  int   Bright frames per cycle; tone duration = frames-on × refresh period (default 1)
//	-frames-off int   Dark frames per cycle (ITI between stimuli) (default 60)
//	-freq-hz float    Tone frequency in Hz (default 1000)
//
// Per-test flags — jitter:
//
//	-duration-s float Duration of frame-rate measurement in seconds (default 10)
//
// Per-test flags — square:
//
//	-period-ms  float Square-wave period in ms (default 100)
//	-duty       float Duty cycle 0–100 % (default 50)
//	-duration-s float Duration of square-wave output in seconds (default 30)
//
// Per-test flags — sound:
//
//	-cycles int       Number of tones in the stream (default 60)
//	-tone-ms int      Duration of each tone in ms (default 50)
//	-iti-ms float     Silence between tones (ISI) in ms (default 450)
//	-freq-hz float    Tone frequency in Hz (default 1000)
//
// Per-test flags — drain:
//
//	-freq-hz float    Tone frequency in Hz (default 1000)
//	-drain-reps int   Repetitions per tone duration (default 10)

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
	fTest        = flag.String("test", "", "Sub-test: check | display | latency | stream | vrr | trigger | frames | tones | av | rt\n\t(legacy aliases: jitter=display  drain=latency  square=trigger  sound=tones  audio=check  flash=frames)")
	fPort        = flag.String("port", "", "Serial port for DLP-IO8-G (empty = auto-detect)")
	fTriggerPin  = flag.Int("trigger-pin", 1, "DLP-IO8-G output pin (1–8)")
	fTriggerMs   = flag.Int("trigger-ms", 5, "Trigger pulse duration (ms)")
	fCycles      = flag.Int("cycles", 120, "Number of cycles / flashes")
	fLevelA      = flag.Int("level-a", 0, "Dark luminance 0–255")
	fLevelB      = flag.Int("level-b", 255, "Bright luminance 0–255")
	fFramesOn    = flag.Int("frames-on", 1, "Bright frames per cycle [frames / stream / av tests]")
	fFramesOff   = flag.Int("frames-off", 9, "Dark frames per cycle [frames / stream / av tests]")
	fSoaMs       = flag.Float64("soa-ms", 0, "Visual-to-audio SOA ms; negative = audio first [av test]")
	fItiMs       = flag.Float64("iti-ms", 1000, "Inter-trial interval ms [av test]")
	fFreqHz      = flag.Float64("freq-hz", 1000, "Tone frequency Hz [av test]")
	fToneMs      = flag.Int("tone-ms", 50, "Tone duration ms [av test]")
	fDurationS   = flag.Float64("duration-s", 10, "Measurement duration in seconds [jitter / square]")
	fPeriodMs    = flag.Float64("period-ms", 100, "Square-wave period ms [square test]")
	fDuty        = flag.Float64("duty", 50, "Duty cycle 0–100 %% [square test]")
	fAudioFrames = flag.Int("audio-frames", 0, "Audio hardware buffer size in sample frames (0=SDL default). Must be set before SDL audio opens; e.g. 256, 512, 1024.")
	fHz          = flag.Float64("hz", 60.0, "Expected display refresh rate in Hz; used to compute frame-interval targets [stream / display]")
	fWarmup      = flag.Int("warmup", 10, "Frames to discard at the start of visual tests before recording statistics")
	fDrainReps   = flag.Int("drain-reps", 10, "Repetitions per tone duration [drain test]")
	fVRRMaxMs    = flag.Int("vrr-max-ms", 50, "Maximum stimulus duration to sweep in VRR test (ms, in 1 ms steps) [vrr test]")
	fWindowed    = flag.Bool("w", false, "Windowed mode (1024×768 window instead of fullscreen)")
	fDisplay     = flag.Int("d", -1, "Display index: monitor where the window/fullscreen will open (-1 = primary)")
	fSysInfo     = flag.Bool("sysinfo", false, "Print system information and exit")
	fPacedFlip   = flag.Bool("paced-flip", false, "Use PacedFlip() instead of Update() for frame pacing in frames/av tests")
)

// ── Screen fill helper ─────────────────────────────────────────────────────────

// fillGray fills the screen with a uniform gray level (0–255) and presents it.
// Returns the time just before and just after RenderPresent (the VSYNC wait),
// in milliseconds with sub-millisecond precision (3 decimal places).
func fillGray(exp *control.Experiment, level byte) (tBefore, tAfter float64) {
	r := exp.Screen.Renderer
	r.SetDrawColor(level, level, level, 255)
	r.Clear()
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

// ── Test: frames (alias: flash) ────────────────────────────────────────────────

// runFrames alternates between a bright phase (*fFramesOn frames at level-b) and
// a dark phase (*fFramesOff frames at level-a) for *fCycles complete cycles.
// A trigger pulse is sent at the onset of each bright phase.
//
// Two statistics are reported from software-side timestamps:
//   - Bright-phase duration: time from the first bright flip to the first dark
//     flip. Should equal frames-on × frame_interval.
//   - Period: time from one bright onset to the next.
//     Should equal (frames-on + frames-off) × frame_interval.
//
// No -hz flag is needed: frame counts are the authoritative unit and the
// measured mean is used as the deviation reference in statistics.
func runFrames(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	framesOn := *fFramesOn
	framesOff := *fFramesOff

	fmt.Printf("frames: level-a=%d level-b=%d frames-on=%d frames-off=%d cycles=%d warmup=%d\n",
		*fLevelA, *fLevelB, framesOn, framesOff, *fCycles, *fWarmup)

	exp.Data.WriteComment(fmt.Sprintf("test=frames level-a=%d level-b=%d frames-on=%d frames-off=%d cycles=%d warmup=%d",
		*fLevelA, *fLevelB, framesOn, framesOff, *fCycles, *fWarmup))
	exp.AddDataVariableNames([]string{
		"cycle", "t_before_ms", "t_after_ms", "bright_duration_ms", "period_ms",
	})

	var brightDurations []float64
	var periods []float64
	var prevBrightStartT float64

	return exp.Run(func() error {
		oldGC := debug.SetGCPercent(-1)
		defer debug.SetGCPercent(oldGC)

		r := exp.Screen.Renderer
		bLevel, dLevel := byte(*fLevelB), byte(*fLevelA)

		flip := exp.Screen.Update
		if *fPacedFlip {
			flip = exp.Screen.PacedFlip
		}

		for cycle := 0; cycle < *fCycles; cycle++ {
			// ── Bright phase ──────────────────────────────────────────────────
			// Re-draw each frame so double-buffering never shows the opposite color.
			var tBrightBefore, tBrightStart float64
			for f := 0; f < framesOn; f++ {
				r.SetDrawColor(bLevel, bLevel, bLevel, 255)
				r.Clear()
				tB := float64(clock.GetTimeNS()) / 1e6
				flip()
				tA := float64(clock.GetTimeNS()) / 1e6
				if f == 0 {
					tBrightBefore = tB
					tBrightStart = tA
					go triggers.FireTrigger(trig, *fTriggerPin, time.Duration(*fTriggerMs)*time.Millisecond)
				}
			}

			// ── Dark phase ────────────────────────────────────────────────────
			var tDarkStart float64
			for f := 0; f < framesOff; f++ {
				r.SetDrawColor(dLevel, dLevel, dLevel, 255)
				r.Clear()
				flip()
				if f == 0 {
					tDarkStart = float64(clock.GetTimeNS()) / 1e6
				}
			}

			state := exp.PollEvents(nil)
			if state.QuitRequested {
				return control.EndLoop
			}

			// ── Record measurements ───────────────────────────────────────────
			// bright_duration_ms = first dark flip − first bright flip = frames-on frame intervals
			// period_ms          = this bright onset − previous bright onset = (on+off) frame intervals
			brightDurMs := tDarkStart - tBrightStart
			var periodMs float64
			if prevBrightStartT > 0 {
				periodMs = tBrightStart - prevBrightStartT
			}

			if cycle >= *fWarmup {
				brightDurations = append(brightDurations, brightDurMs)
				if periodMs > 0 {
					periods = append(periods, periodMs)
				}
			}

			exp.Data.Add(cycle,
				fmt.Sprintf("%.3f", tBrightBefore),
				fmt.Sprintf("%.3f", tBrightStart),
				fmt.Sprintf("%.3f", brightDurMs),
				fmt.Sprintf("%.3f", periodMs))

			prevBrightStartT = tBrightStart
		}

		// Use measured mean as deviation reference — no -hz needed.
		sDur := timingstats.ComputeStats(brightDurations, 0)
		sDur = timingstats.ComputeStats(brightDurations, sDur.Mean)
		timingstats.PrintStats(fmt.Sprintf("Bright-phase duration (frames-on=%d)", framesOn), sDur, sDur.Mean)

		sPer := timingstats.ComputeStats(periods, 0)
		sPer = timingstats.ComputeStats(periods, sPer.Mean)
		timingstats.PrintStats(fmt.Sprintf("Period (frames-on=%d + frames-off=%d)", framesOn, framesOff), sPer, sPer.Mean)

		return control.EndLoop
	})
}

// ── Test: av ──────────────────────────────────────────────────────────────────

// runAV presents periodic visual flashes paired with tones at a configurable SOA.
//
// The bright phase lasts frames-on frames; the tone duration matches that duration
// (frames-on × refresh period, derived from -hz). The dark ITI between stimuli
// lasts frames-off frames. -iti-ms and -tone-ms are not used by this test.
func runAV(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	framesOn := *fFramesOn
	framesOff := *fFramesOff
	frameMs := 1000.0 / *fHz
	toneDurMs := int(math.Round(float64(framesOn) * frameMs))

	syncMethod := "goroutine"
	if *fSoaMs == 0 {
		syncMethod = "PlaySyncedWithFlip"
	}
	fmt.Printf("av: soa=%.1f ms  freq=%.0f Hz  tone=%d ms (frames-on=%d × %.2f ms)  frames-off=%d  cycles=%d  sync=%s\n",
		*fSoaMs, *fFreqHz, toneDurMs, framesOn, frameMs, framesOff, *fCycles, syncMethod)

	tone := stimuli.NewTone(*fFreqHz, toneDurMs, 0.8)
	if err := tone.PreloadDevice(exp.AudioDevice); err != nil {
		return fmt.Errorf("av: preload tone: %w", err)
	}
	defer tone.Unload()

	exp.AddDataVariableNames([]string{
		"trial",
		"t_visual_before_ms", "t_visual_after_ms",
		"t_audio_queued_ms",
		"soa_intended_ms", "soa_actual_ms",
	})

	soaDur := time.Duration(math.Abs(*fSoaMs) * float64(time.Millisecond))
	audioFirst := *fSoaMs < 0

	return exp.Run(func() error {
		r := exp.Screen.Renderer
		bLevel, dLevel := byte(*fLevelB), byte(*fLevelA)

		flip := exp.Screen.Update
		if *fPacedFlip {
			flip = exp.Screen.PacedFlip
		}

		// holdBright redraws bright into the backbuffer for each remaining frame
		// so double-buffering never flips the opposite color into view.
		holdBright := func(remaining int) {
			for f := 0; f < remaining; f++ {
				r.SetDrawColor(bLevel, bLevel, bLevel, 255)
				r.Clear()
				flip()
			}
		}

		for trial := 0; trial < *fCycles; trial++ {
			var tVisB, tVisA, tAudioQ float64

			if audioFirst {
				tAudioQ = float64(clock.GetTimeNS()) / 1e6
				_ = tone.Play()
				time.Sleep(soaDur)
				r.SetDrawColor(bLevel, bLevel, bLevel, 255)
				r.Clear()
				tVisB = float64(clock.GetTimeNS()) / 1e6
				flip()
				tVisA = float64(clock.GetTimeNS()) / 1e6
				go triggers.FireTrigger(trig, *fTriggerPin, time.Duration(*fTriggerMs)*time.Millisecond)
				holdBright(framesOn - 1)
			} else if soaDur == 0 {
				// SOA=0: use PlaySyncedWithFlip so the audio buffer is pre-filled
				// before the flip and the device resumes immediately after VSYNC.
				// This eliminates goroutine scheduling jitter and primes the audio
				// pipeline at flip time; onset lags by at most one callback period.
				r.SetDrawColor(bLevel, bLevel, bLevel, 255)
				r.Clear()
				tVisB = float64(clock.GetTimeNS()) / 1e6
				flipNS, _ := tone.PlaySyncedWithFlip(exp.Screen)
				tVisA = float64(flipNS) / 1e6
				tAudioQ = tVisA // audio pre-buffered; onset ≤ tVisA + 1 callback period
				go triggers.FireTrigger(trig, *fTriggerPin, time.Duration(*fTriggerMs)*time.Millisecond)
				holdBright(framesOn - 1)
			} else {
				r.SetDrawColor(bLevel, bLevel, bLevel, 255)
				r.Clear()
				tVisB = float64(clock.GetTimeNS()) / 1e6
				flip()
				tVisA = float64(clock.GetTimeNS()) / 1e6
				go triggers.FireTrigger(trig, *fTriggerPin, time.Duration(*fTriggerMs)*time.Millisecond)
				// Schedule audio at the requested SOA relative to visual onset;
				// holdBright keeps the bright screen for the remaining frames concurrently.
				tAudioQCh := make(chan float64, 1)
				go func() {
					time.Sleep(soaDur)
					tAudioQCh <- float64(clock.GetTimeNS()) / 1e6
					_ = tone.Play()
				}()
				holdBright(framesOn - 1)
				tAudioQ = <-tAudioQCh
			}

			soaActual := tAudioQ - tVisA
			exp.Data.Add(trial,
				fmt.Sprintf("%.3f", tVisB), fmt.Sprintf("%.3f", tVisA), fmt.Sprintf("%.3f", tAudioQ),
				fmt.Sprintf("%.1f", *fSoaMs),
				fmt.Sprintf("%.1f", soaActual))

			// Dark phase: frames-off frames as ITI between stimuli.
			for f := 0; f < framesOff; f++ {
				r.SetDrawColor(dLevel, dLevel, dLevel, 255)
				r.Clear()
				flip()
			}

			state := exp.PollEvents(nil)
			if state.QuitRequested {
				return control.EndLoop
			}
		}
		fmt.Printf("\nav: %d trials complete. Check oscilloscope for AV sync.\n", *fCycles)
		return control.EndLoop
	})
}

// ── Test: jitter ───────────────────────────────────────────────────────────────

// runJitter measures raw frame-interval variance by repeatedly flipping a gray screen.
func runJitter(exp *control.Experiment) error {
	nFrames := int(*fDurationS * *fHz) // approximate; actual count depends on refresh rate
	fmt.Printf("jitter: ~%d frames over %.1f s  warmup=%d  (ESC to stop early)\n", nFrames, *fDurationS, *fWarmup)

	exp.Data.WriteComment(fmt.Sprintf("test=jitter duration-s=%.1f hz=%.2f warmup=%d", *fDurationS, *fHz, *fWarmup))
	exp.AddDataVariableNames([]string{"frame", "t_before_ms", "t_after_ms", "interval_ms"})

	var intervals []float64
	var prevT float64

	return exp.Run(func() error {
		oldGC := debug.SetGCPercent(-1)
		defer debug.SetGCPercent(oldGC)

		level := byte(128)
		deadline := time.Now().Add(time.Duration(*fDurationS * float64(time.Second)))
		frame := 0

		for time.Now().Before(deadline) {
			tB, tA := fillGray(exp, level)

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
		fmt.Printf("\nEstimated refresh rate: %.3f Hz  (use -hz %.2f for frames/flash targets)\n",
			estimatedHz, estimatedHz)
		timingstats.PrintStats("Frame intervals", s, s.Mean)
		return control.EndLoop
	})
}

// ── Test: square ──────────────────────────────────────────────────────────────

// runSquare outputs a square wave on the specified DLP-IO8 pin for the
// configured duration. No display is needed; the test shows a status screen.
func runSquare(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	if _, isNull := trig.(triggers.NullOutputTTLDevice); isNull {
		return fmt.Errorf("square test requires a DLP-IO8-G (no device found)")
	}

	period := time.Duration(*fPeriodMs * float64(time.Millisecond))
	highDur := time.Duration(float64(period) * *fDuty / 100.0)
	totalDur := time.Duration(*fDurationS * float64(time.Second))
	expectedCycles := int(totalDur / period)

	fmt.Printf("square: period=%.1f ms  duty=%.0f %%  pin=%d  duration=%.0f s  (~%d cycles)\n",
		*fPeriodMs, *fDuty, *fTriggerPin, *fDurationS, expectedCycles)

	exp.AddDataVariableNames([]string{
		"cycle", "edge", "target_ms", "actual_ms", "jitter_ms",
	})

	var riseJitter, fallJitter []float64

	return exp.Run(func() error {
		// Show a status screen.
		status := stimuli.NewTextLine(
			fmt.Sprintf("Square wave: %.1f ms period, %.0f%% duty, pin %d — press ESC to stop",
				*fPeriodMs, *fDuty, *fTriggerPin),
			0, 0, control.White)
		if err := exp.Show(status); err != nil {
			return err
		}

		start := time.Now()
		deadline := start.Add(totalDur)

		for cycle := 0; time.Now().Before(deadline); cycle++ {
			// ── Rising edge ────────────────────────────────────────────────
			targetRise := start.Add(time.Duration(cycle) * period)
			sleepUntil(targetRise)
			tRise := time.Now()
			if err := trig.SetHigh(*fTriggerPin); err != nil {
				return err
			}
			jRise := tRise.Sub(targetRise).Seconds() * 1000
			riseJitter = append(riseJitter, jRise)
			exp.Data.Add(cycle, "rise",
				fmt.Sprintf("%.3f", targetRise.Sub(start).Seconds()*1000),
				fmt.Sprintf("%.3f", tRise.Sub(start).Seconds()*1000),
				fmt.Sprintf("%.3f", jRise))

			// ── Falling edge ───────────────────────────────────────────────
			targetFall := targetRise.Add(highDur)
			sleepUntil(targetFall)
			tFall := time.Now()
			if err := trig.SetLow(*fTriggerPin); err != nil {
				return err
			}
			jFall := tFall.Sub(targetFall).Seconds() * 1000
			fallJitter = append(fallJitter, jFall)
			exp.Data.Add(cycle, "fall",
				fmt.Sprintf("%.3f", targetFall.Sub(start).Seconds()*1000),
				fmt.Sprintf("%.3f", tFall.Sub(start).Seconds()*1000),
				fmt.Sprintf("%.3f", jFall))

			// Allow ESC / window-close to abort.
			state := exp.PollEvents(nil)
			if state.QuitRequested {
				break
			}

			// Sleep until the end of the low phase to keep the loop near-idle.
			nextRise := targetRise.Add(period)
			slack := nextRise.Sub(time.Now()) - 2*time.Millisecond
			if slack > 0 {
				time.Sleep(slack)
			}
		}

		_ = trig.SetLow(*fTriggerPin)
		timingstats.PrintStats("Rising-edge jitter (ms from target)", timingstats.ComputeStats(riseJitter, 0), 0)
		timingstats.PrintStats("Falling-edge jitter (ms from target)", timingstats.ComputeStats(fallJitter, 0), 0)
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

// ── Test: sound ───────────────────────────────────────────────────────────────

// runSound plays a regular tone stream via stimuli.PlayStreamOfSounds and
// reports onset-jitter statistics from the returned TimingLog.
//
// If a DLP-IO8-G is connected, pin *fTriggerPin is set HIGH immediately before
// PlayStreamOfSounds and LOW immediately after it returns, producing a single
// square pulse whose width equals the total stream duration. Connect pin 1 to
// oscilloscope channel 2 and the audio line-out to channel 1 to compare the
// measured pulse width against the nominal nTones × SOA value.
func runSound(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	toneDur := time.Duration(*fToneMs) * time.Millisecond
	isiDur := time.Duration(*fItiMs) * time.Millisecond
	soa := toneDur + isiDur
	nTones := *fCycles

	_, isNull := trig.(triggers.NullOutputTTLDevice)

	fmt.Printf("sound: %d tones  %.0f Hz  %d ms on  %.0f ms ISI  SOA %d ms  total ~%.1f s",
		nTones, *fFreqHz, *fToneMs, *fItiMs, soa.Milliseconds(),
		float64(nTones)*soa.Seconds())
	if !isNull {
		fmt.Printf("  trigger pin %d (high→low brackets full stream)", *fTriggerPin)
	}
	fmt.Println()

	exp.Data.WriteComment(fmt.Sprintf("test=sound cycles=%d freq-hz=%.0f tone-ms=%d iti-ms=%.0f soa-ms=%d",
		nTones, *fFreqHz, *fToneMs, *fItiMs, soa.Milliseconds()))

	tone := stimuli.NewTone(*fFreqHz, *fToneMs, 0.8)
	if err := tone.PreloadDevice(exp.AudioDevice); err != nil {
		return fmt.Errorf("sound: preload tone: %w", err)
	}
	defer tone.Unload()

	sounds := make([]stimuli.AudioPlayable, nTones)
	for i := range sounds {
		sounds[i] = tone
	}
	elements := stimuli.MakeRegularSoundStream(sounds, toneDur, isiDur)

	exp.AddDataVariableNames([]string{
		"tone_num",
		"target_onset_ms", "actual_onset_ms", "onset_error_ms",
		"actual_offset_ms",
		"ioi_ms", "ioi_error_ms",
	})

	return exp.Run(func() error {
		status := stimuli.NewTextLine(
			fmt.Sprintf("Audio timing: %d × %.0f Hz tones, %d ms on / %.0f ms ISI — ESC to stop",
				nTones, *fFreqHz, *fToneMs, *fItiMs),
			0, 0, control.White)
		if err := exp.Show(status); err != nil {
			return err
		}

		if !isNull {
			_ = trig.SetHigh(*fTriggerPin)
		}
		_, timing, err := stimuli.PlayStreamOfSounds(elements)
		if !isNull {
			_ = trig.SetLow(*fTriggerPin)
		}
		if control.IsEndLoop(err) {
			return control.EndLoop
		}
		if err != nil {
			return err
		}

		soaMs := float64(soa) / 1e6 // nanoseconds → milliseconds
		var onsetErrors, ioiVals []float64
		var prevActualMs float64

		var ref uint64
		if len(timing) > 0 {
			ref = timing[0].OnsetNS // stream start on the SDL clock (same clock as the trigger pulse); OnsetNS/OffsetNS are authoritative (see TimingLog)
		}
		for _, tl := range timing {
			targetOnsetMs := float64(tl.Index) * soaMs
			actualOnsetMs := float64(tl.OnsetNS-ref) / 1e6
			actualOffsetMs := float64(tl.OffsetNS-ref) / 1e6
			onsetErr := actualOnsetMs - targetOnsetMs

			var ioiMs, ioiErr float64
			if tl.Index > 0 {
				ioiMs = actualOnsetMs - prevActualMs
				ioiErr = ioiMs - soaMs
				ioiVals = append(ioiVals, ioiMs)
			}
			onsetErrors = append(onsetErrors, onsetErr)
			prevActualMs = actualOnsetMs

			exp.Data.Add(
				tl.Index,
				fmt.Sprintf("%.3f", targetOnsetMs),
				fmt.Sprintf("%.3f", actualOnsetMs),
				fmt.Sprintf("%.3f", onsetErr),
				fmt.Sprintf("%.3f", actualOffsetMs),
				fmt.Sprintf("%.3f", ioiMs),
				fmt.Sprintf("%.3f", ioiErr),
			)
		}

		timingstats.PrintStats("Onset error vs target (ms)", timingstats.ComputeStats(onsetErrors, 0), 0)
		timingstats.PrintStats("Inter-onset interval (ms)", timingstats.ComputeStats(ioiVals, soaMs), soaMs)
		return control.EndLoop
	})
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

		oldGC := debug.SetGCPercent(-1)
		defer debug.SetGCPercent(oldGC)

		for i := 0; i < nTrials; i++ {
			// Jittered ITI: meanItiMs ± 50 %
			jitter := (rand.Float64() - 0.5) * meanItiMs
			itiDur := time.Duration((meanItiMs + jitter) * float64(time.Millisecond))
			exp.Screen.Clear()
			exp.Screen.Update()
			time.Sleep(itiDur)

			// Flash: draw white screen and flip, capturing SDL nanosecond onset.
			exp.Screen.Renderer.SetDrawColor(255, 255, 255, 255)
			exp.Screen.Renderer.Clear()
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

// ── Test: stream ──────────────────────────────────────────────────────────────

// runStream measures the timing accuracy of sequential (RSVP-style) stimulus
// presentations — the kind of trial loop a psychologist would actually run in
// a rapid serial visual presentation paradigm.
//
// *fCycles elements are presented. Each element consists of *fFramesOn
// bright frames (luminance *fLevelB) followed by *fFramesOff dark frames
// (luminance *fLevelA). If a DLP-IO8-G is connected, a trigger pulse is fired
// on *fTriggerPin at the onset of every bright phase so that the software
// timestamps can be validated against a photodiode on the oscilloscope.
//
// Two statistics are reported after the run:
//   - Duration error  : actual on-duration − target on-duration (ms).
//     A non-zero mean indicates a systematic off-by-one-frame bug or driver
//     double-buffering; high SD indicates frame-drop events.
//   - SOA error       : actual onset-to-onset interval − target SOA (ms).
//     This is the quantity that matters most for RSVP experiments: if the SOA
//     is 100 ms but the SD is 3 ms, word presentations drift in and out of
//     phase with any auditory rhythm you might be synchronising to.
//
// The first *fWarmup elements are excluded from statistics (GPU pipeline warm-up).
func runStream(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	n := *fCycles
	onFrames := *fFramesOn
	offFrames := *fFramesOff
	targetFrameMs := 1000.0 / *fHz
	targetOnMs := float64(onFrames) * targetFrameMs
	targetOffMs := float64(offFrames) * targetFrameMs
	targetSOAms := targetOnMs + targetOffMs

	_, isNull := trig.(triggers.NullOutputTTLDevice)
	trigDur := time.Duration(*fTriggerMs) * time.Millisecond

	fmt.Printf("stream: %d elements  on=%d frames (%.2f ms)  off=%d frames (%.2f ms)  SOA=%.2f ms  hz=%.2f  warmup=%d",
		n, onFrames, targetOnMs, offFrames, targetOffMs, targetSOAms, *fHz, *fWarmup)
	if !isNull {
		fmt.Printf("  trigger pin %d (%d ms pulse)", *fTriggerPin, *fTriggerMs)
	}
	fmt.Println()

	exp.Data.WriteComment(fmt.Sprintf(
		"test=stream cycles=%d frames-on=%d frames-off=%d hz=%.2f warmup=%d level-a=%d level-b=%d",
		n, onFrames, offFrames, *fHz, *fWarmup, *fLevelA, *fLevelB))
	exp.AddDataVariableNames([]string{
		"element",
		"t_onset_ms", "t_offset_ms",
		"onset_ns", "offset_ns",
		"duration_ms", "duration_error_ms",
		"interval_ms", "interval_error_ms",
		"trigger",
	})

	return exp.Run(func() error {
		status := stimuli.NewTextLine(
			fmt.Sprintf("Stream timing: %d elements, %d on / %d off frames — press ESC to stop",
				n, onFrames, offFrames),
			0, 0, control.White)
		if err := exp.Show(status); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)

		oldGC := debug.SetGCPercent(-1)
		defer debug.SetGCPercent(oldGC)

		var durationErrors, intervalErrors []float64
		streamStartMs := float64(clock.GetTimeNS()) / 1e6
		var prevOnsetNS uint64

		r := exp.Screen.Renderer
		bR, bG, bB := byte(*fLevelB), byte(*fLevelB), byte(*fLevelB) // bright
		dR, dG, dD := byte(*fLevelA), byte(*fLevelA), byte(*fLevelA) // dark

		for elem := 0; elem < n; elem++ {
			// ── ON phase ──────────────────────────────────────────────────────
			var onsetNS uint64
			var tOnsetMs float64

			for f := 0; f < onFrames; f++ {
				r.SetDrawColor(bR, bG, bB, 255)
				r.Clear()
				if f == 0 {
					ns, _ := exp.Screen.FlipTS()
					onsetNS = ns
					tOnsetMs = float64(clock.GetTimeNS())/1e6 - streamStartMs
					if !isNull {
						go triggers.FireTrigger(trig, *fTriggerPin, trigDur)
					}
				} else {
					exp.Screen.Update()
				}
			}

			// ── OFF phase (ISI) ───────────────────────────────────────────────
			var offsetNS uint64
			var tOffsetMs float64

			for f := 0; f < offFrames; f++ {
				r.SetDrawColor(dR, dG, dD, 255)
				r.Clear()
				if f == 0 {
					ns, _ := exp.Screen.FlipTS()
					offsetNS = ns
					tOffsetMs = float64(clock.GetTimeNS())/1e6 - streamStartMs
				} else {
					exp.Screen.Update()
				}
			}

			// ── Statistics ────────────────────────────────────────────────────
			durationMs := tOffsetMs - tOnsetMs
			durationError := durationMs - targetOnMs

			var intervalMs, intervalError float64
			if prevOnsetNS > 0 {
				intervalMs = float64(onsetNS-prevOnsetNS) / 1e6
				intervalError = intervalMs - targetSOAms
				if elem >= *fWarmup {
					intervalErrors = append(intervalErrors, intervalError)
				}
			}
			if elem >= *fWarmup {
				durationErrors = append(durationErrors, durationError)
			}
			prevOnsetNS = onsetNS

			exp.Data.Add(
				elem,
				fmt.Sprintf("%.3f", tOnsetMs),
				fmt.Sprintf("%.3f", tOffsetMs),
				onsetNS, offsetNS,
				fmt.Sprintf("%.3f", durationMs),
				fmt.Sprintf("%.3f", durationError),
				fmt.Sprintf("%.3f", intervalMs),
				fmt.Sprintf("%.3f", intervalError),
				!isNull,
			)

			state := exp.PollEvents(nil)
			if state.QuitRequested {
				return control.EndLoop
			}
		}

		timingstats.PrintStats(fmt.Sprintf("Duration error (target %.2f ms)", targetOnMs), timingstats.ComputeStats(durationErrors, 0), 0)
		if len(intervalErrors) > 0 {
			timingstats.PrintStats(fmt.Sprintf("SOA error (target %.2f ms)", targetSOAms), timingstats.ComputeStats(intervalErrors, 0), 0)
		}
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

		oldGC := debug.SetGCPercent(-1)
		defer debug.SetGCPercent(oldGC)

		r := exp.Screen.Renderer

		for targetMs := 1; targetMs <= maxMs; targetMs++ {
			targetDur := time.Duration(targetMs) * time.Millisecond
			var durationErrors []float64

			for rep := 0; rep < reps; rep++ {
				// ── ISI: blank screen ────────────────────────────────────────
				r.SetDrawColor(byte(*fLevelA), byte(*fLevelA), byte(*fLevelA), 255)
				r.Clear()
				exp.Screen.Flip() // non-blocking with vsync=0
				time.Sleep(200 * time.Millisecond)

				// ── Onset: bright screen ─────────────────────────────────────
				if !isNull {
					_ = trig.SetHigh(*fTriggerPin)
				}
				r.SetDrawColor(byte(*fLevelB), byte(*fLevelB), byte(*fLevelB), 255)
				r.Clear()
				onsetNS, _ := exp.Screen.FlipTS() // returns immediately (vsync=0)

				// ── Hold for exactly targetDur using busy-wait ────────────────
				sleepUntil(time.Now().Add(targetDur))

				// ── Offset: blank screen ─────────────────────────────────────
				r.SetDrawColor(byte(*fLevelA), byte(*fLevelA), byte(*fLevelA), 255)
				r.Clear()
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
		}

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

		oldGC := debug.SetGCPercent(-1)
		defer debug.SetGCPercent(oldGC)

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

	if *fTest == "" {
		exp.End() // release any SDL subsystems already initialised before exiting
		log.Fatal("usage: go run main.go -test <check|display|latency|stream|vrr|trigger|frames|flash|tones|av|rt> [flags]\n" +
			"       (legacy aliases: jitter=display  drain=latency  square=trigger  sound=tones  audio=check)")
	}

	// Log actual audio device format so the user can verify the buffer size.
	if spec, frames, err := exp.AudioDevice.Format(); err == nil {
		fmt.Printf("audio: %d Hz  %d ch  %d sample frames (~%.1f ms latency)\n",
			spec.Freq, spec.Channels, frames,
			float64(frames)/float64(spec.Freq)*1000)
	}

	trig, _ := setupTrigger()
	defer trig.Close()

	var runErr error
	switch *fTest {
	// ── Tier 0: sanity check ─────────────────────────────────────────────────
	case "check", "audio": // "audio" is the legacy name
		runErr = runCheck(exp)
	// ── Tier 1: self-contained measurements ──────────────────────────────────
	case "display", "jitter": // "jitter" is the legacy name
		runErr = runJitter(exp)
	case "latency", "drain": // "drain" is the legacy name
		runErr = runDrain(exp)
	case "stream":
		runErr = runStream(exp, trig)
	case "vrr":
		runErr = runVRR(exp, trig)
	// ── Tier 2: trigger device characterisation ───────────────────────────────
	case "trigger", "square": // "square" is the legacy name
		runErr = runSquare(exp, trig)
	// ── Tier 3: stimulus timing validation ───────────────────────────────────
	case "frames", "flash": // "flash" is the legacy name (frames-on=1 frames-off=N)
		runErr = runFrames(exp, trig)
	case "tones", "sound": // "sound" is the legacy name
		runErr = runSound(exp, trig)
	case "av":
		runErr = runAV(exp, trig)
	// ── Tier 4: response timing ───────────────────────────────────────────────
	case "rt":
		runErr = runRT(exp, trig)
	default:
		exp.End() // release any SDL subsystems already initialised before exiting
		log.Fatalf("unknown test %q — choose from: check display latency stream vrr trigger frames flash tones av rt\n"+
			"  (legacy aliases: audio=check  jitter=display  drain=latency  square=trigger  sound=tones)", *fTest)
	}

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("test error: %v", runErr)
	}
}
