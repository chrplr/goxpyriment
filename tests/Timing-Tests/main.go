// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
//
// Timing-Tests — hardware timing verification suite
//
// Provides five independent sub-tests, selected with -test <name>:
//
//	frames  Alternating luminance (photodiode test)
//	flash   Single-frame white flashes (minimum-duration test)
//	av      Audio-visual synchrony (oscilloscope + photodiode)
//	jitter  Pure frame-interval statistics (no external device needed)
//	square  DLP-IO8 square-wave trigger (oscilloscope only)
//	sound   Long regular tone stream, onset-jitter statistics
//	rt      SDL event-timestamp RT precision test (keyboard or response box)
//
// See README.md for full usage, equipment setup, and interpretation.
//
// Usage:
//
//	go run main.go -test <name> [flags] [-d]
//
// Common flags:
//
//	-test string      Sub-test to run (required)
//	-port string      Serial port for DLP-IO8-G (default: auto-detect)
//	-trigger-pin int  Output pin on DLP-IO8-G (default 1)
//	-trigger-ms  int  Trigger pulse duration in ms (default 5)
//	-cycles int       Number of cycles / flashes (default 60)
//	-d                Developer mode: windowed 1024×768
//
// Per-test flags — rt:
//
//	-iti-ms float     Mean inter-trial interval ms (jittered ±50 %; default 1000)
//
// Per-test flags — frames / flash:
//
//	-level-a int      Dark luminance 0–255 (default 0)
//	-level-b int      Bright luminance 0–255 (default 255)
//	-frames-per-phase int  Frames at each luminance (default 2)   [frames]
//	-isi-frames int   Frames between flashes (default 60)         [flash]
//
// Per-test flags — av:
//
//	-soa-ms float     Visual-to-audio SOA in ms; negative = audio first (default 0)
//	-iti-ms float     Inter-trial interval in ms (default 1000)
//	-freq-hz float    Tone frequency in Hz (default 1000)
//	-tone-ms int      Tone duration in ms (default 50)
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

package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"runtime/debug"
	"sort"
	"time"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/triggers"
)

// ── Flags ─────────────────────────────────────────────────────────────────────

var (
	fTest           = flag.String("test", "", "Sub-test: frames | flash | av | jitter | square")
	fPort           = flag.String("port", "", "Serial port for DLP-IO8-G (empty = auto-detect)")
	fTriggerPin     = flag.Int("trigger-pin", 1, "DLP-IO8-G output pin (1–8)")
	fTriggerMs      = flag.Int("trigger-ms", 5, "Trigger pulse duration (ms)")
	fCycles         = flag.Int("cycles", 60, "Number of cycles / flashes")
	fLevelA         = flag.Int("level-a", 0, "Dark luminance 0–255")
	fLevelB         = flag.Int("level-b", 255, "Bright luminance 0–255")
	fFramesPerPhase = flag.Int("frames-per-phase", 2, "Frames at each luminance [frames test]")
	fIsiFrames      = flag.Int("isi-frames", 60, "Frames between flashes [flash test]")
	fSoaMs          = flag.Float64("soa-ms", 0, "Visual-to-audio SOA ms; negative = audio first [av test]")
	fItiMs          = flag.Float64("iti-ms", 1000, "Inter-trial interval ms [av test]")
	fFreqHz         = flag.Float64("freq-hz", 1000, "Tone frequency Hz [av test]")
	fToneMs         = flag.Int("tone-ms", 50, "Tone duration ms [av test]")
	fDurationS      = flag.Float64("duration-s", 10, "Measurement duration in seconds [jitter / square]")
	fPeriodMs       = flag.Float64("period-ms", 100, "Square-wave period ms [square test]")
	fDuty           = flag.Float64("duty", 50, "Duty cycle 0–100 %% [square test]")
	fAudioFrames    = flag.Int("audio-frames", 0, "Audio hardware buffer size in sample frames (0=SDL default). Must be set before SDL audio opens; e.g. 256, 512, 1024.")
)

// ── Statistics helper ──────────────────────────────────────────────────────────

type stats struct {
	mean, sd, minV, maxV, p5, p95 float64
	late05, late1                 int // count > 0.5 ms and > 1 ms from target
	n                             int
}

func computeStats(deltas []float64, targetMs float64) stats {
	n := len(deltas)
	if n == 0 {
		return stats{}
	}
	var sum float64
	mn, mx := deltas[0], deltas[0]
	for _, v := range deltas {
		sum += v
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	mean := sum / float64(n)
	var sqSum float64
	var late05, late1 int
	for _, v := range deltas {
		sqSum += (v - mean) * (v - mean)
		dev := math.Abs(v - targetMs)
		if dev > 0.5 {
			late05++
		}
		if dev > 1.0 {
			late1++
		}
	}
	sd := math.Sqrt(sqSum / float64(n))
	sorted := make([]float64, n)
	copy(sorted, deltas)
	sort.Float64s(sorted)
	p5 := sorted[n*5/100]
	p95 := sorted[n*95/100]
	return stats{mean, sd, mn, mx, p5, p95, late05, late1, n}
}

func printStats(label string, s stats, targetMs float64) {
	fmt.Printf("\n── %s ───────────────────────────────\n", label)
	fmt.Printf("  n       : %d\n", s.n)
	fmt.Printf("  target  : %.3f ms\n", targetMs)
	fmt.Printf("  mean    : %.3f ms\n", s.mean)
	fmt.Printf("  SD      : %.3f ms\n", s.sd)
	fmt.Printf("  min/max : %.3f / %.3f ms\n", s.minV, s.maxV)
	fmt.Printf("  p5/p95  : %.3f / %.3f ms\n", s.p5, s.p95)
	fmt.Printf("  >0.5 ms : %d (%.1f %%)\n", s.late05, 100*float64(s.late05)/float64(s.n))
	fmt.Printf("  >1.0 ms : %d (%.1f %%)\n", s.late1, 100*float64(s.late1)/float64(s.n))
}

// ── Screen fill helper ─────────────────────────────────────────────────────────

// fillGray fills the screen with a uniform gray level (0–255) and presents it.
// Returns the time just before and just after RenderPresent (the VSYNC wait).
func fillGray(exp *control.Experiment, level byte) (tBefore, tAfter int64) {
	r := exp.Screen.Renderer
	r.SetDrawColor(level, level, level, 255)
	r.Clear()
	tBefore = clock.GetTime()
	exp.Screen.Update() // blocks until VSYNC
	tAfter = clock.GetTime()
	return
}

// ── Trigger setup ──────────────────────────────────────────────────────────────

func setupTrigger() (triggers.Trigger, string) {
	var trig triggers.Trigger
	var portName string
	var err error

	if *fPort != "" {
		d, openErr := triggers.NewDLPIO8(*fPort)
		if openErr != nil {
			log.Printf("warning: DLP-IO8 on %s: %v — triggers disabled", *fPort, openErr)
			trig = triggers.NullTrigger{}
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

// ── Test: frames ───────────────────────────────────────────────────────────────

// runFrames alternates between two luminance levels for *fCycles complete cycles.
// A trigger pulse is sent on each transition to the bright phase.
func runFrames(exp *control.Experiment, trig triggers.Trigger) error {
	fmt.Printf("frames: level-a=%d level-b=%d frames-per-phase=%d cycles=%d\n",
		*fLevelA, *fLevelB, *fFramesPerPhase, *fCycles)

	exp.AddDataVariableNames([]string{
		"cycle", "phase", "frame",
		"t_before_ms", "t_after_ms", "interval_ms", "trigger",
	})

	var intervals []float64
	var prevT int64
	totalFrames := *fCycles * 2 * *fFramesPerPhase
	frame := 0

	return exp.Run(func() error {
		for cycle := 0; cycle < *fCycles; cycle++ {
			for phase := 0; phase < 2; phase++ {
				level := byte(*fLevelA)
				isBright := phase == 1
				if isBright {
					level = byte(*fLevelB)
				}
				for f := 0; f < *fFramesPerPhase; f++ {
					triggered := false
					if isBright && f == 0 {
						// Send trigger just before the flip so it precedes the onset.
						_ = trig.SetHigh(*fTriggerPin)
						triggered = true
					}

					tB, tA := fillGray(exp, level)

					if triggered {
						go func() {
							time.Sleep(time.Duration(*fTriggerMs) * time.Millisecond)
							_ = trig.SetLow(*fTriggerPin)
						}()
					}

					var intervalMs float64
					if prevT > 0 {
						intervalMs = float64(tA - prevT)
						intervals = append(intervals, intervalMs)
					}
					prevT = tA

					exp.Data.Add(cycle, phase, frame, tB, tA, fmt.Sprintf("%.1f", intervalMs), triggered)
					frame++

					// Check for ESC / quit.
					state := exp.PollEvents(nil)
					if state.QuitRequested {
						return control.EndLoop
					}
				}
			}
		}
		_ = totalFrames
		printStats("Frame intervals", computeStats(intervals, float64(*fFramesPerPhase)*16.67), 16.67)
		return control.EndLoop
	})
}

// ── Test: flash ────────────────────────────────────────────────────────────────

// runFlash presents a single bright frame every *fIsiFrames dark frames.
func runFlash(exp *control.Experiment, trig triggers.Trigger) error {
	fmt.Printf("flash: level-a=%d level-b=%d isi-frames=%d cycles=%d\n",
		*fLevelA, *fLevelB, *fIsiFrames, *fCycles)

	exp.AddDataVariableNames([]string{
		"flash_num", "t_before_ms", "t_after_ms", "interval_ms",
	})

	var flashIntervals []float64
	var prevFlashT int64

	return exp.Run(func() error {
		for flash := 0; flash < *fCycles; flash++ {
			// ISI: dark frames
			for f := 0; f < *fIsiFrames; f++ {
				fillGray(exp, byte(*fLevelA))
				state := exp.PollEvents(nil)
				if state.QuitRequested {
					return control.EndLoop
				}
			}

			// Single bright frame + trigger
			_ = trig.SetHigh(*fTriggerPin)
			tB, tA := fillGray(exp, byte(*fLevelB))
			go func() {
				time.Sleep(time.Duration(*fTriggerMs) * time.Millisecond)
				_ = trig.SetLow(*fTriggerPin)
			}()

			var intervalMs float64
			if prevFlashT > 0 {
				intervalMs = float64(tA - prevFlashT)
				flashIntervals = append(flashIntervals, intervalMs)
			}
			prevFlashT = tA
			exp.Data.Add(flash, tB, tA, fmt.Sprintf("%.1f", intervalMs))
		}

		expectedMs := float64(*fIsiFrames+1) * 16.67
		printStats("Flash intervals", computeStats(flashIntervals, expectedMs), expectedMs)
		return control.EndLoop
	})
}

// ── Test: av ──────────────────────────────────────────────────────────────────

// runAV presents periodic visual flashes paired with tones at a configurable SOA.
func runAV(exp *control.Experiment, trig triggers.Trigger) error {
	fmt.Printf("av: soa=%.1f ms  iti=%.0f ms  tone=%.0f Hz / %d ms  cycles=%d\n",
		*fSoaMs, *fItiMs, *fFreqHz, *fToneMs, *fCycles)

	tone := stimuli.NewTone(*fFreqHz, *fToneMs, 0.8)
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
		for trial := 0; trial < *fCycles; trial++ {
			var tVisB, tVisA, tAudioQ int64

			if audioFirst {
				tAudioQ = clock.GetTime()
				_ = tone.Play()
				time.Sleep(soaDur)
				_ = trig.SetHigh(*fTriggerPin)
				tVisB, tVisA = fillGray(exp, byte(*fLevelB))
				go func() {
					time.Sleep(time.Duration(*fTriggerMs) * time.Millisecond)
					_ = trig.SetLow(*fTriggerPin)
				}()
			} else {
				_ = trig.SetHigh(*fTriggerPin)
				tVisB, tVisA = fillGray(exp, byte(*fLevelB))
				go func() {
					time.Sleep(time.Duration(*fTriggerMs) * time.Millisecond)
					_ = trig.SetLow(*fTriggerPin)
				}()
				time.Sleep(soaDur)
				tAudioQ = clock.GetTime()
				_ = tone.Play()
			}

			soaActual := float64(tAudioQ - tVisA)
			exp.Data.Add(trial, tVisB, tVisA, tAudioQ,
				fmt.Sprintf("%.1f", *fSoaMs),
				fmt.Sprintf("%.1f", soaActual))

			// ITI: dark screen
			fillGray(exp, byte(*fLevelA))
			remaining := time.Duration(*fItiMs*float64(time.Millisecond)) - 16*time.Millisecond
			if remaining > 0 {
				time.Sleep(remaining)
			}

			state := exp.PollEvents(nil)
			if state.QuitRequested {
				return control.EndLoop
			}
		}
		fmt.Printf("\nav: %d trials complete. Check oscilloscope for audio latency.\n", *fCycles)
		return control.EndLoop
	})
}

// ── Test: jitter ───────────────────────────────────────────────────────────────

// runJitter measures raw frame-interval variance by repeatedly flipping a gray screen.
func runJitter(exp *control.Experiment) error {
	nFrames := int(*fDurationS * 60) // approximate; actual count depends on refresh rate
	fmt.Printf("jitter: ~%d frames over %.1f s (ESC to stop early)\n", nFrames, *fDurationS)

	exp.AddDataVariableNames([]string{"frame", "t_before_ms", "t_after_ms", "interval_ms"})

	var intervals []float64
	var prevT int64

	return exp.Run(func() error {
		level := byte(128)
		deadline := time.Now().Add(time.Duration(*fDurationS * float64(time.Second)))
		frame := 0

		for time.Now().Before(deadline) {
			tB, tA := fillGray(exp, level)

			var intervalMs float64
			if prevT > 0 {
				intervalMs = float64(tA - prevT)
				intervals = append(intervals, intervalMs)
			}
			prevT = tA
			exp.Data.Add(frame, tB, tA, fmt.Sprintf("%.3f", intervalMs))
			frame++

			state := exp.PollEvents(nil)
			if state.QuitRequested {
				break
			}
		}

		// Estimate frame rate from mean interval.
		s := computeStats(intervals, 16.67)
		estimatedHz := 0.0
		if s.mean > 0 {
			estimatedHz = 1000.0 / s.mean
		}
		fmt.Printf("\nEstimated refresh rate: %.2f Hz\n", estimatedHz)
		printStats("Frame intervals", s, s.mean)
		return control.EndLoop
	})
}

// ── Test: square ──────────────────────────────────────────────────────────────

// runSquare outputs a square wave on the specified DLP-IO8 pin for the
// configured duration. No display is needed; the test shows a status screen.
func runSquare(exp *control.Experiment, trig triggers.Trigger) error {
	if _, isNull := trig.(triggers.NullTrigger); isNull {
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
		printStats("Rising-edge jitter (ms from target)", computeStats(riseJitter, 0), 0)
		printStats("Falling-edge jitter (ms from target)", computeStats(fallJitter, 0), 0)
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

// runSound plays a long regular tone stream and reports onset-jitter statistics.
//
// A DLP-IO8-G trigger pulse is sent on *fTriggerPin just before each tone's
// Play() call. Connect pin 1 to oscilloscope channel 2 and the audio line-out
// to channel 1 to measure the actual software-to-acoustic latency per tone.
//
// GC is disabled for the duration of the stream (mirrors PlayStreamOfSounds).
// The SOA is toneDur + isiDur; a 300-tone stream at 50 ms / 450 ms ISI runs
// ~2.5 minutes — long enough to reveal cumulative drift and scheduling outliers.
func runSound(exp *control.Experiment, trig triggers.Trigger) error {
	toneDur := time.Duration(*fToneMs) * time.Millisecond
	isiDur := time.Duration(*fItiMs) * time.Millisecond
	soa := toneDur + isiDur
	nTones := *fCycles
	triggerDur := time.Duration(*fTriggerMs) * time.Millisecond

	_, isNull := trig.(triggers.NullTrigger)

	fmt.Printf("sound: %d tones  %.0f Hz  %d ms on  %.0f ms ISI  SOA %d ms  total ~%.1f s",
		nTones, *fFreqHz, *fToneMs, int64(*fItiMs), soa.Milliseconds(),
		float64(nTones)*soa.Seconds())
	if !isNull {
		fmt.Printf("  trigger pin %d (%d ms pulse)", *fTriggerPin, *fTriggerMs)
	}
	fmt.Println()

	tone := stimuli.NewTone(*fFreqHz, *fToneMs, 0.8)
	if err := tone.PreloadDevice(exp.AudioDevice); err != nil {
		return fmt.Errorf("sound: preload tone: %w", err)
	}
	defer tone.Unload()

	exp.AddDataVariableNames([]string{
		"tone_num",
		"target_onset_ms", "actual_onset_ms", "onset_error_ms",
		"actual_offset_ms",
		"ioi_ms", "ioi_error_ms",
		"trigger_sent",
	})

	return exp.Run(func() error {
		status := stimuli.NewTextLine(
			fmt.Sprintf("Audio timing: %d × %.0f Hz tones, %d ms on / %.0f ms ISI — ESC to stop",
				nTones, *fFreqHz, *fToneMs, *fItiMs),
			0, 0, control.White)
		if err := exp.Show(status); err != nil {
			return err
		}

		oldGC := debug.SetGCPercent(-1)
		defer debug.SetGCPercent(oldGC)

		soaMs := soa.Milliseconds()
		var onsetErrors, ioiVals []float64
		var prevActualMs int64
		streamStart := time.Now()

		for i := 0; i < nTones; i++ {
			targetOnsetMs := int64(i) * soaMs

			// ── Trigger + Play ────────────────────────────────────────────
			if !isNull {
				_ = trig.SetHigh(*fTriggerPin)
			}
			actualOnset := time.Since(streamStart)
			_ = tone.Play()

			// Pulse the trigger for triggerMs, then set low synchronously.
			if !isNull {
				time.Sleep(triggerDur)
				_ = trig.SetLow(*fTriggerPin)
			}

			// ── Wait remainder of on-phase ────────────────────────────────
			onDeadline := streamStart.Add(time.Duration(targetOnsetMs)*time.Millisecond + toneDur)
			for time.Now().Before(onDeadline) {
				time.Sleep(time.Millisecond)
				if exp.PollEvents(nil).QuitRequested {
					return control.EndLoop
				}
			}
			actualOffset := time.Since(streamStart)

			// ── Wait ISI ──────────────────────────────────────────────────
			offDeadline := onDeadline.Add(isiDur)
			for time.Now().Before(offDeadline) {
				time.Sleep(time.Millisecond)
				if exp.PollEvents(nil).QuitRequested {
					return control.EndLoop
				}
			}

			// ── Log ───────────────────────────────────────────────────────
			actualMs := actualOnset.Milliseconds()
			onsetErr := float64(actualMs - targetOnsetMs)

			var ioiMs, ioiErr float64
			if i > 0 {
				ioiMs = float64(actualMs - prevActualMs)
				ioiErr = ioiMs - float64(soaMs)
				ioiVals = append(ioiVals, ioiMs)
			}
			onsetErrors = append(onsetErrors, onsetErr)
			prevActualMs = actualMs

			exp.Data.Add(
				i,
				targetOnsetMs,
				actualMs,
				fmt.Sprintf("%.3f", onsetErr),
				actualOffset.Milliseconds(),
				fmt.Sprintf("%.3f", ioiMs),
				fmt.Sprintf("%.3f", ioiErr),
				!isNull,
			)
		}

		printStats("Onset error vs target (ms)", computeStats(onsetErrors, 0), 0)
		printStats("Inter-onset interval (ms)", computeStats(ioiVals, float64(soaMs)), float64(soaMs))
		return control.EndLoop
	})
}

// ── Test: rt ──────────────────────────────────────────────────────────────────

// runRT measures keyboard reaction time using SDL3 event timestamps.
//
// Each trial: a white flash appears for one frame; the participant presses any
// key as fast as possible. RT is computed as event.Timestamp − onset_ns, where
// onset_ns is the SDL nanosecond tick captured by Screen.FlipNS() immediately
// after SDL_RenderPresent returns.
//
// Because both timestamps come from the same SDL nanosecond clock (SDL_GetTicksNS),
// RT reflects the interval between actual display output and the hardware
// keyboard interrupt — without any polling latency on the response side.
//
// Use with a hardware response box connected as a USB keyboard for ground-truth
// RT validation. Compare against the photodiode onset (frames test) to obtain
// the full stimulus-onset → button-press chain in nanoseconds.
func runRT(exp *control.Experiment, trig triggers.Trigger) error {
	nTrials := *fCycles
	meanItiMs := *fItiMs
	fmt.Printf("rt: %d trials  mean ITI %.0f ms  press any key each flash\n", nTrials, meanItiMs)

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
			itiDur := time.Duration((meanItiMs+jitter)*float64(time.Millisecond))
			exp.Screen.Clear()
			exp.Screen.Update()
			time.Sleep(itiDur)

			// Optional trigger pulse just before the onset flip.
			_, isNull := trig.(triggers.NullTrigger)
			if !isNull {
				_ = trig.SetHigh(*fTriggerPin)
			}

			// Flash: draw white screen and flip, capturing SDL nanosecond onset.
			exp.Screen.Renderer.SetDrawColor(255, 255, 255, 255)
			exp.Screen.Renderer.Clear()
			onsetNS, _ := exp.Screen.FlipNS()

			if !isNull {
				go func() {
					time.Sleep(time.Duration(*fTriggerMs) * time.Millisecond)
					_ = trig.SetLow(*fTriggerPin)
				}()
			}

			// Wait for keypress — returns SDL event timestamp (nanoseconds).
			_, eventTS, err := exp.Keyboard.WaitKeysEventRT(nil, 5000)
			if control.IsEndLoop(err) {
				return control.EndLoop
			}

			rtNS := int64(eventTS - onsetNS)
			rtMs := float64(rtNS) / 1e6
			rtValues = append(rtValues, rtMs)

			exp.Data.Add(i, onsetNS, eventTS, rtNS, fmt.Sprintf("%.3f", rtMs))
			fmt.Printf("trial %3d  RT = %.1f ms\n", i, rtMs)
		}

		printStats("RT (ms, event-timestamp method)", computeStats(rtValues, 0), 0)
		return control.EndLoop
	})
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	// Parse flags early so we can act on -audio-frames before SDL opens the
	// audio device inside NewExperimentFromFlags. flag.Parse() is idempotent;
	// NewExperimentFromFlags will call it again harmlessly.
	flag.Parse()
	if *fAudioFrames > 0 {
		control.SetAudioSampleFrames(*fAudioFrames)
		fmt.Printf("audio: requesting %d sample frames hardware buffer\n", *fAudioFrames)
	}

	exp := control.NewExperimentFromFlags("Timing-Tests", control.Black, control.White, 24)
	defer exp.End()

	if *fTest == "" {
		log.Fatal("usage: go run main.go -test <frames|flash|av|jitter|square|sound> [flags]")
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
	case "frames":
		runErr = runFrames(exp, trig)
	case "flash":
		runErr = runFlash(exp, trig)
	case "av":
		runErr = runAV(exp, trig)
	case "jitter":
		runErr = runJitter(exp)
	case "square":
		runErr = runSquare(exp, trig)
	case "sound":
		runErr = runSound(exp, trig)
	case "rt":
		runErr = runRT(exp, trig)
	default:
		log.Fatalf("unknown test %q — choose from: frames flash av jitter square sound rt", *fTest)
	}

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("test error: %v", runErr)
	}
}
