// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
//
// test_audio_drift — measure the audio clock's rate error against the system
// clock, in parts per million.
//
// # What it measures
//
// A long tone (default 10 minutes) is synthesised in memory and played through
// the normal goxpyriment audio path. While it plays, the test repeatedly pairs
//
//	t_audio  = frames the device has consumed / nominal sample rate
//	t_system = clock.GetTimeNS() (Go monotonic clock)
//
// and regresses one on the other. The slope, expressed as
//
//	ppm = (slope - 1) x 1e6
//
// is how much faster (positive) or slower (negative) the audio hardware runs
// than its nominal rate. A DAC crystal spec'd at +/-50 ppm accumulates
// +/-30 ms of error over a 10-minute file; that is the effect this test is
// sized to resolve.
//
// # What the slope does and does not include
//
// The frame count comes from SDL_GetAudioStreamQueued, which sees only SDL's
// software queue. Frames that have left it may still sit in the hardware DMA
// buffer, unheard. That lag is *constant*, so it lands entirely in the
// regression intercept and does not bias the slope — which is the whole reason
// this test regresses rather than simply timing the file end to end. Timing the
// end of playback instead makes the file look one buffer period *short*
// (~23 ms on PipeWire at 44.1 kHz), an artefact easily mistaken for the DAC
// running fast.
//
// The audio thread drains that queue one callback period at a time, so
// individual samples are quantised into a staircase of the same ~23 ms. That is
// noise on each point, not bias, and averages down over the run.
//
// # Steady skew versus wander
//
// The per-segment table separates the two mechanisms that produce a duration
// mismatch:
//
//   - a fixed crystal offset gives the same ppm in every segment;
//   - an adaptive resampler tracking a second clock (a loopback, combined or
//     network sink, Bluetooth, an aggregate device) makes ppm wander between
//     segments while the overall mean stays near the true ratio.
//
// Read the per-segment spread before attributing a number to the crystal.
//
// # Reporting
//
// Every run records the sound server, device rate, and callback size in the CSV
// header. A ppm figure without them is not comparable across machines: routing
// the same DAC through a different server changes it.
//
// Flags:
//
//	-minutes float   Tone duration in minutes (default 10)
//	-rate int        WAV sample rate in Hz (default 44100)
//	-tone-hz float   Tone frequency in Hz (default 440)
//	-amplitude float Tone amplitude 0–1 (default 0.05)
//	-interval-ms int Sampling interval in ms (default 500)
//	-warmup-s float  Leading seconds discarded from the fit (default 5)
//	-segment-s float Per-segment report length in seconds (default 60)
//	-bracket-us int  Reject samples whose read bracket exceeds this (default 200)
//	-audio-frames N  Audio hardware buffer, sample frames (0 = SDL default)
//	-csv path        Output CSV (default audio_drift.csv)
//	-w               Windowed mode — recommended, the test draws nothing
//	-d int           Display index (-1 = primary)
//
// ESC or Q stops the run early and still writes the fit over whatever was
// collected.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"time"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/sysinfo"
)

var (
	fMinutes     = flag.Float64("minutes", 10, "Tone duration in minutes")
	fRate        = flag.Int("rate", 44100, "WAV sample rate in Hz")
	fToneHz      = flag.Float64("tone-hz", 440, "Tone frequency in Hz")
	fAmplitude   = flag.Float64("amplitude", 0.05, "Tone amplitude, 0–1")
	fIntervalMs  = flag.Int("interval-ms", 500, "Sampling interval in ms")
	fWarmupS     = flag.Float64("warmup-s", 5, "Leading seconds discarded from the fit")
	fCooldownS   = flag.Float64("cooldown-s", 5, "Trailing seconds discarded from the fit (see the tail note in the README)")
	fSegmentS    = flag.Float64("segment-s", 60, "Per-segment report length in seconds")
	fBracketUs   = flag.Int("bracket-us", 200, "Reject samples whose read bracket exceeds this many µs")
	fAudioFrames = flag.Int("audio-frames", 0, "Audio hardware buffer size in sample frames (0 = SDL default)")
	fCSV         = flag.String("csv", "audio_drift.csv", "Output CSV path")
	fWindowed    = flag.Bool("w", false, "Windowed mode (1024×768 window instead of fullscreen)")
	fDisplay     = flag.Int("d", -1, "Display index: monitor where the window/fullscreen will open (-1 = primary)")
)

// sample is one paired reading of the audio and system clocks.
type sample struct {
	tSystem float64 // seconds since the first reading
	tAudio  float64 // seconds of audio consumed by the device
	frames  int64   // frames consumed, for the record
	queued  int32   // bytes still in SDL's software queue
	bracket int64   // ns spent inside the paired read

	// The exclusions are tracked apart because they mean different things:
	// warm-up and cool-down samples are discarded by design, a wide bracket
	// means the reading itself is suspect. Reporting them as one "rejected"
	// count makes a normal run look like it collected bad data.
	inWarmup    bool // before -warmup-s elapsed
	inCooldown  bool // within -cooldown-s of the last reading
	drained     bool // software queue empty: frames consumed is clamped
	wideBracket bool // paired read took longer than -bracket-us
}

// used reports whether this sample contributes to the fit.
func (s sample) used() bool {
	return !s.inWarmup && !s.inCooldown && !s.drained && !s.wideBracket
}

func main() {
	// Parse early so -audio-frames can be applied before SDL opens the device.
	// This is why the test defines -w and -d itself and calls NewExperiment
	// rather than NewExperimentFromFlags, which parses only after registering
	// its own flags.
	flag.Parse()

	if *fAmplitude <= 0 || *fAmplitude > 1 {
		log.Fatalf("-amplitude must be in (0, 1], got %g", *fAmplitude)
	}
	if *fMinutes <= 0 {
		log.Fatalf("-minutes must be positive, got %g", *fMinutes)
	}
	if *fIntervalMs <= 0 {
		log.Fatalf("-interval-ms must be positive, got %d", *fIntervalMs)
	}
	if *fAudioFrames > 0 {
		control.SetAudioSampleFrames(*fAudioFrames)
		fmt.Printf("audio: requesting a %d sample-frame hardware buffer\n", *fAudioFrames)
	}

	width, height, fullscreen := 0, 0, true
	if *fWindowed {
		width, height, fullscreen = 1024, 768, false
	}
	exp := control.NewExperiment("Audio Drift Test", width, height, fullscreen,
		control.Black, control.White, 24)
	if *fDisplay >= 0 {
		exp.ScreenNumber = *fDisplay
	}
	if err := exp.Initialize(); err != nil {
		exp.End()
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// The device spec is read once here rather than per sample: querying it
	// inside the measurement loop would widen the read bracket for no gain,
	// since the values are fixed for the lifetime of the device.
	var devRate, devChannels, devFrames int
	if spec, frames, err := exp.AudioDevice.Format(); err == nil && spec != nil {
		devRate, devChannels, devFrames = int(spec.Freq), int(spec.Channels), int(frames)
	} else if err != nil {
		log.Printf("warning: could not read audio device format: %v", err)
	}

	audio := sysinfo.Collect().Audio
	server := audio.Server
	if server == "" {
		server = "unknown"
	}

	totalFrames := int64(*fMinutes * 60 * float64(*fRate))
	wav := synthWAV(*fRate, totalFrames, *fToneHz, *fAmplitude)
	nominalDur := float64(totalFrames) / float64(*fRate)

	fmt.Printf("\n─── configuration ───────────────────────────────────────────\n")
	fmt.Printf("sound server     : %s %s\n", server, audio.SrvVer)
	fmt.Printf("WAV              : %d Hz mono 16-bit, %d frames, %.1f s nominal\n",
		*fRate, totalFrames, nominalDur)
	if devRate > 0 {
		callbackMs := float64(devFrames) / float64(devRate) * 1000
		fmt.Printf("device           : %d Hz, %d ch, %d frames/callback (%.1f ms)\n",
			devRate, devChannels, devFrames, callbackMs)
		if devRate != *fRate {
			fmt.Printf("  NOTE: device rate differs from the WAV rate — SDL is resampling at a\n")
			fmt.Printf("  fixed ratio. The measured ppm still refers to the device clock, but\n")
			fmt.Printf("  rerun with -rate %d to take the resampler out of the path.\n", devRate)
		}
	}
	fmt.Printf("sampling         : every %d ms, discarding the first %.0f s\n",
		*fIntervalMs, *fWarmupS)
	fmt.Printf("─────────────────────────────────────────────────────────────\n\n")

	snd := stimuli.NewSoundFromMemory(wav)
	if err := snd.PreloadDevice(exp.AudioDevice); err != nil {
		log.Fatalf("preload sound: %v", err)
	}
	defer snd.Unload()

	samples, stoppedEarly := run(exp, snd, totalFrames)

	if len(samples) < 3 {
		log.Fatalf("only %d samples collected — nothing to fit", len(samples))
	}

	meta := []string{
		fmt.Sprintf("sound_server: %s %s", server, audio.SrvVer),
		fmt.Sprintf("alsa_version: %s", audio.ALSA),
		fmt.Sprintf("wav_rate_hz: %d", *fRate),
		fmt.Sprintf("tone_hz: %g", *fToneHz),
		fmt.Sprintf("tone_amplitude: %g", *fAmplitude),
		fmt.Sprintf("device_rate_hz: %d", devRate),
		fmt.Sprintf("device_channels: %d", devChannels),
		fmt.Sprintf("device_callback_frames: %d", devFrames),
		fmt.Sprintf("nominal_duration_s: %.3f", nominalDur),
		fmt.Sprintf("sampling_interval_ms: %d", *fIntervalMs),
		fmt.Sprintf("warmup_s: %.1f", *fWarmupS),
		fmt.Sprintf("cooldown_s: %.1f", *fCooldownS),
		fmt.Sprintf("bracket_reject_us: %d", *fBracketUs),
		fmt.Sprintf("stopped_early: %t", stoppedEarly),
	}
	if err := writeCSV(*fCSV, meta, samples); err != nil {
		log.Printf("warning: writing %s: %v", *fCSV, err)
	} else {
		fmt.Printf("\nper-sample data written to %s\n", *fCSV)
	}

	report(samples, devFrames, devRate, stoppedEarly)
}

// run plays the sound and collects paired clock readings until the stream
// drains, the user presses ESC/Q, or the window closes.
//
// Each reading brackets the queue query between two system-clock reads. If the
// goroutine is descheduled (or the GC pauses it) mid-read, the bracket widens
// and the sample is flagged rather than silently contributing a mistimed point.
// The midpoint of a narrow bracket is a better estimate of when the queue held
// that value than either endpoint.
func run(exp *control.Experiment, snd *stimuli.Sound, totalFrames int64) ([]sample, bool) {
	const bytesPerFrame = 2 // mono 16-bit, as synthWAV writes it

	if err := snd.Play(); err != nil {
		log.Fatalf("play: %v", err)
	}

	var (
		samples      []sample
		originNS     int64
		haveOrigin   bool
		stoppedEarly bool
		interval     = time.Duration(*fIntervalMs) * time.Millisecond
		bracketLimit = int64(*fBracketUs) * 1000
		nextTick     = time.Now()
		lastPrint    time.Time
	)

	for {
		nextTick = nextTick.Add(interval)
		if d := time.Until(nextTick); d > 0 {
			time.Sleep(d)
		}

		if key, err := exp.Keyboard.Check(); err == nil &&
			(key == control.K_ESCAPE || key == control.K_Q) {
			stoppedEarly = true
			break
		}

		t0 := clock.GetTimeNS()
		queued, err := snd.Stream.Queued()
		t1 := clock.GetTimeNS()
		if err != nil {
			log.Printf("warning: reading queue: %v", err)
			continue
		}
		if queued < 0 {
			queued = 0
		}

		if !haveOrigin {
			originNS, haveOrigin = t0, true
		}
		consumed := totalFrames - int64(queued)/bytesPerFrame
		s := sample{
			tSystem: float64((t0+t1)/2-originNS) / 1e9,
			tAudio:  float64(consumed) / float64(*fRate),
			frames:  consumed,
			queued:  queued,
			bracket: t1 - t0,
		}
		s.wideBracket = t1-t0 > bracketLimit
		s.inWarmup = s.tSystem < *fWarmupS
		// A drained queue clamps frames-consumed at the file length, so the
		// sample reports less audio elapsed than the interval that produced it.
		// Left in, this single point drags the slope hard on a short run.
		s.drained = queued == 0
		samples = append(samples, s)

		if time.Since(lastPrint) >= 10*time.Second {
			lastPrint = time.Now()
			drift := (s.tAudio - s.tSystem) * 1000
			fmt.Printf("\r  t=%7.1f s  audio-system = %+8.1f ms  ", s.tSystem, drift)
			os.Stdout.Sync()
		}

		if queued <= 0 {
			break
		}
	}
	fmt.Println()

	// Cool-down can only be marked once the end time is known.
	//
	// The tail is contaminated for roughly as long as the audio buffered
	// downstream of SDL's queue: over that final stretch the downstream buffer
	// is draining rather than staying full, so "frames that left the software
	// queue" stops tracking "frames played" at the steady-state offset the fit
	// assumes. Measured on this rig, dropping the last 5 s took a 42 s run's
	// residual SD from 17.3 ms to 9.0 ms and its slope from -356 to +44 ppm.
	if len(samples) > 0 && *fCooldownS > 0 {
		end := samples[len(samples)-1].tSystem
		for i := range samples {
			if samples[i].tSystem > end-*fCooldownS {
				samples[i].inCooldown = true
			}
		}
	}
	return samples, stoppedEarly
}

// report prints the overall fit and the per-segment breakdown.
func report(samples []sample, devFrames, devRate int, stoppedEarly bool) {
	used := make([]sample, 0, len(samples))
	var nWarmup, nCooldown, nDrained, nWide int
	for _, s := range samples {
		switch {
		case s.used():
			used = append(used, s)
		case s.inWarmup:
			nWarmup++
		case s.drained:
			nDrained++
		case s.inCooldown:
			nCooldown++
		default:
			nWide++
		}
	}
	if len(used) < 3 {
		fmt.Printf("\nonly %d usable samples after warm-up and bracket rejection — no fit\n", len(used))
		return
	}

	fit, ok := regress(used)
	if !ok {
		fmt.Printf("\nsystem times do not vary — no fit\n")
		return
	}
	span := used[len(used)-1].tSystem - used[0].tSystem

	fmt.Printf("\n─── result ──────────────────────────────────────────────────\n")
	fmt.Printf("samples          : %d used of %d (%d warm-up, %d cool-down, %d drained, %d wide-bracket)\n",
		len(used), len(samples), nWarmup, nCooldown, nDrained, nWide)
	if nWide > len(samples)/20 {
		fmt.Printf("  NOTE: over 5%% of readings exceeded the %d µs bracket — this host was\n", *fBracketUs)
		fmt.Printf("  busy. The fit survives it, but rerun on an idle machine before quoting.\n")
	}
	fmt.Printf("fitted span      : %.1f s\n", span)
	if stoppedEarly {
		fmt.Printf("                   (stopped early — span is shorter than requested)\n")
	}
	fmt.Printf("\nrate error       : %+.2f ppm   (95%% CI %+.2f … %+.2f)\n",
		fit.ppm, fit.ppm-1.96*fit.ppmSE, fit.ppm+1.96*fit.ppmSE)
	fmt.Printf("  audio clock runs %s than nominal\n", fasterSlower(fit.ppm))
	fmt.Printf("accumulated drift: %+.1f ms over the fitted %.1f s\n",
		fit.slope1*span*1000, span)
	fmt.Printf("  extrapolated   : %+.1f ms per 10 minutes\n", fit.slope1*600*1000)
	// The residual distribution, not the slope, is what says whether the fit
	// may be believed. A clean run is symmetric about zero and bounded by
	// roughly the callback period; anything with a long tail means some samples
	// are structurally different from the rest, and on a short run a handful of
	// those set the answer.
	res := fit.residuals
	sorted := append([]float64(nil), res...)
	sort.Float64s(sorted)
	quant := func(p float64) float64 { return sorted[int(p*float64(len(sorted)-1))] * 1000 }
	maxAbs := math.Max(math.Abs(sorted[0]), math.Abs(sorted[len(sorted)-1])) * 1000

	fmt.Printf("\nresiduals (ms)   : SD %.1f, median %+.1f, IQR %+.1f … %+.1f, range %+.1f … %+.1f\n",
		fit.residSD*1000, quant(0.5), quant(0.25), quant(0.75),
		sorted[0]*1000, sorted[len(sorted)-1]*1000)
	fmt.Printf("  skew %+.2f, kurtosis %.2f (uniform 1.8, normal 3.0), lag-1 autocorr %+.3f\n",
		skewness(res), kurtosis(res), autocorr(res, 1))
	if devFrames > 0 && devRate > 0 {
		q := float64(devFrames) / float64(devRate) * 1000
		fmt.Printf("  the queue drains one %.1f ms callback at a time, so quantisation alone\n", q)
		fmt.Printf("  predicts SD ≈ %.1f ms (period/sqrt(12)) and |residual| ≲ %.1f ms.\n",
			q/math.Sqrt(12), q)
		if maxAbs > 3*q {
			fmt.Printf("  WARNING: largest residual is %.0f ms, over 3 callback periods. Some\n", maxAbs)
			fmt.Printf("  samples are structurally unlike the rest — inspect the CSV tail before\n")
			fmt.Printf("  believing the slope, and consider raising -cooldown-s.\n")
		}
	}
	if a := math.Abs(autocorr(res, 1)); a > 0.5 {
		fmt.Printf("  NOTE: lag-1 autocorrelation is %+.2f. The printed CI assumes independent\n", autocorr(res, 1))
		fmt.Printf("  residuals and is therefore too narrow here.\n")
	}
	fmt.Printf("intercept        : %+.1f ms — the audio already buffered ahead at t=0\n",
		fit.intercept*1000)
	fmt.Printf("                   (SDL's internal buffering plus the hardware buffer, and\n")
	fmt.Printf("                   typically far more than one callback period). Constant,\n")
	fmt.Printf("                   so it biases the slope not at all — and is not a drift\n")
	fmt.Printf("                   measurement in itself.\n")

	fmt.Printf("\n─── per segment (%.0f s each) ─────────────────────────────────\n", *fSegmentS)
	fmt.Printf("  %-18s %6s %10s %10s\n", "window (s)", "n", "ppm", "+/-SE")
	var segPPM, segSE []float64
	for start := used[0].tSystem; start < used[len(used)-1].tSystem; start += *fSegmentS {
		end := start + *fSegmentS
		seg := make([]sample, 0, 64)
		for _, s := range used {
			if s.tSystem >= start && s.tSystem < end {
				seg = append(seg, s)
			}
		}
		if len(seg) < 3 {
			continue
		}
		sf, segOK := regress(seg)
		if !segOK {
			continue
		}
		segPPM = append(segPPM, sf.ppm)
		segSE = append(segSE, sf.ppmSE)
		fmt.Printf("  %7.0f – %-8.0f %6d %+10.2f %10.2f\n", start, end, len(seg), sf.ppm, sf.ppmSE)
	}

	if len(segPPM) >= 2 {
		mean, sd := meanSD(segPPM)
		// A short segment estimates its own slope badly, so the segments will
		// scatter even when the true rate is perfectly constant. The question
		// is not whether they scatter but whether they scatter by MORE than
		// their own standard errors predict. Comparing the observed SD against
		// the mean SE is what separates a wandering rate from a noisy fit;
		// comparing it against the mean ppm (the obvious thing) instead reports
		// "wandering" for every run that is merely too short.
		expected := rms(segSE)
		fmt.Printf("\nsegment ppm      : mean %+.2f, SD %.2f, range %+.2f … %+.2f\n",
			mean, sd, minOf(segPPM), maxOf(segPPM))
		fmt.Printf("  scatter expected from the fits' own noise: %.2f ppm\n", expected)
		fmt.Printf("\ninterpretation:\n")
		// Wander and resolution are separate questions and the answers are
		// independent: segments can agree with each other (no wander) while
		// none of them individually resolves the effect. Reporting only
		// "too noisy" when both hold throws away the finding that nothing is
		// wandering, which is the question this table exists to answer.
		if sd <= 1.5*expected {
			fmt.Printf("  Segment scatter (%.1f ppm) is no more than the fits' own noise (%.1f ppm).\n", sd, expected)
			fmt.Printf("  Consistent with a FIXED clock ratio (crystal tolerance) — no sign of an\n")
			fmt.Printf("  adaptive resampler chasing a second clock.\n")
		} else {
			fmt.Printf("  Segment scatter (%.1f ppm) exceeds the fits' own noise (%.1f ppm): the\n", sd, expected)
			fmt.Printf("  rate is WANDERING, which is what a rate-matching resampler does. Check\n")
			fmt.Printf("  for a loopback, combined/network sink, Bluetooth, or aggregate device.\n")
		}
		if expected > math.Abs(mean) {
			fmt.Printf("\n  Individually the segments cannot resolve an offset this small (SE %.1f ppm\n", expected)
			fmt.Printf("  vs mean %+.2f ppm); the overall fit above, which uses the full span, can.\n", mean)
			fmt.Printf("  Raise -segment-s for a per-segment verdict of comparable precision.\n")
		}
		if sd < expected/2 {
			fmt.Printf("\n  The segments agree with each other markedly better than their own SEs\n")
			fmt.Printf("  predict. Those SEs are inflated by low-frequency wander in the residual,\n")
			fmt.Printf("  which shifts a segment's points together rather than about its own line.\n")
		}
	}

	// Earlier versions asserted here that the residuals are serially correlated
	// and the CI therefore too narrow. Measured, the lag-1 autocorrelation is
	// small and its sign varies between runs, so the assertion was not
	// supported by the data the tool itself collects. Report the number instead
	// of the assumption.
	a1 := autocorr(res, 1)
	fmt.Printf("\ncaveat: the CI assumes independent residuals; the measured lag-1\n")
	fmt.Printf("autocorrelation is %+.3f. Structure at longer lags can still widen the true\n", a1)
	fmt.Printf("interval, so repeat the run before quoting a figure — agreement between two\n")
	fmt.Printf("runs is worth more than either CI.\n")
	fmt.Printf("─────────────────────────────────────────────────────────────\n")
}

// fitResult holds an ordinary least-squares fit of audio time on system time.
type fitResult struct {
	slope1    float64 // slope - 1, i.e. fractional rate error
	intercept float64
	ppm       float64
	ppmSE     float64
	residSD   float64
	residuals []float64 // seconds, in sample order
}

// regress fits tAudio = intercept + slope*tSystem by ordinary least squares.
// It reports false when the system times are degenerate.
func regress(s []sample) (fitResult, bool) {
	n := float64(len(s))
	var sumX, sumY float64
	for _, p := range s {
		sumX += p.tSystem
		sumY += p.tAudio
	}
	meanX, meanY := sumX/n, sumY/n

	var sxx, sxy float64
	for _, p := range s {
		dx := p.tSystem - meanX
		sxx += dx * dx
		sxy += dx * (p.tAudio - meanY)
	}
	if sxx == 0 {
		return fitResult{}, false
	}
	slope := sxy / sxx
	intercept := meanY - slope*meanX

	var ssres float64
	residuals := make([]float64, len(s))
	for i, p := range s {
		r := p.tAudio - (intercept + slope*p.tSystem)
		residuals[i] = r
		ssres += r * r
	}
	// n > 2 is guaranteed by the callers' len(s) >= 3 checks.
	sigma2 := ssres / (n - 2)
	slopeSE := math.Sqrt(sigma2 / sxx)

	return fitResult{
		slope1:    slope - 1,
		intercept: intercept,
		ppm:       (slope - 1) * 1e6,
		ppmSE:     slopeSE * 1e6,
		residSD:   math.Sqrt(sigma2),
		residuals: residuals,
	}, true
}

// skewness and kurtosis describe the residual distribution's shape. Pure
// callback quantisation is uniform (kurtosis 1.8) and symmetric; a long tail or
// a marked skew means some samples were produced by a different process than
// the rest, which is the case worth catching.
func skewness(v []float64) float64 { return standardisedMoment(v, 3) }
func kurtosis(v []float64) float64 { return standardisedMoment(v, 4) }

func standardisedMoment(v []float64, k int) float64 {
	n := float64(len(v))
	if n < 2 {
		return 0
	}
	var mean float64
	for _, x := range v {
		mean += x
	}
	mean /= n
	var m2, mk float64
	for _, x := range v {
		d := x - mean
		m2 += d * d
		mk += math.Pow(d, float64(k))
	}
	m2 /= n
	mk /= n
	if m2 == 0 {
		return 0
	}
	return mk / math.Pow(m2, float64(k)/2)
}

// autocorr is the lag-k autocorrelation of the residuals. The printed
// confidence interval assumes independence; a large value says it is too
// narrow, which is worth knowing before quoting it.
func autocorr(v []float64, k int) float64 {
	if len(v) <= k {
		return 0
	}
	var num, den float64
	for i := 0; i < len(v)-k; i++ {
		num += v[i] * v[i+k]
	}
	for _, x := range v {
		den += x * x
	}
	if den == 0 {
		return 0
	}
	return num / den
}

// synthWAV builds a mono 16-bit PCM WAV in memory: a sine at toneHz, with a
// 50 ms raised-cosine ramp at each end so the start and end do not click.
//
// The signal content is irrelevant to the measurement — only the frame count
// is — but an audible tone makes it obvious the run is alive, and a quiet one
// makes ten minutes bearable.
func synthWAV(rate int, frames int64, toneHz, amplitude float64) []byte {
	const (
		headerBytes   = 44
		bytesPerFrame = 2
	)
	dataBytes := frames * bytesPerFrame
	buf := bytes.NewBuffer(make([]byte, 0, headerBytes+dataBytes))

	w32 := func(v uint32) { binary.Write(buf, binary.LittleEndian, v) }
	w16 := func(v uint16) { binary.Write(buf, binary.LittleEndian, v) }

	buf.WriteString("RIFF")
	w32(uint32(36 + dataBytes))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	w32(16)                           // PCM fmt chunk size
	w16(1)                            // PCM
	w16(1)                            // mono
	w32(uint32(rate))                 // sample rate
	w32(uint32(rate * bytesPerFrame)) // byte rate
	w16(uint16(bytesPerFrame))        // block align
	w16(16)                           // bits per sample
	buf.WriteString("data")
	w32(uint32(dataBytes))

	ramp := int64(0.050 * float64(rate))
	if ramp*2 > frames {
		ramp = frames / 2
	}
	omega := 2 * math.Pi * toneHz / float64(rate)
	sample := make([]byte, 2)
	for i := int64(0); i < frames; i++ {
		env := 1.0
		if ramp > 0 {
			if i < ramp {
				env = 0.5 * (1 - math.Cos(math.Pi*float64(i)/float64(ramp)))
			} else if rem := frames - 1 - i; rem < ramp {
				env = 0.5 * (1 - math.Cos(math.Pi*float64(rem)/float64(ramp)))
			}
		}
		v := int16(amplitude * env * math.Sin(omega*float64(i)) * 32767)
		binary.LittleEndian.PutUint16(sample, uint16(v))
		buf.Write(sample)
	}
	return buf.Bytes()
}

// writeCSV saves every sample, rejected ones included, with the run conditions
// as #-prefixed metadata in the goxpyriment results convention. Rejected rows
// are kept so a reanalysis can apply a different bracket threshold.
func writeCSV(path string, meta []string, samples []sample) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, m := range meta {
		if _, err := fmt.Fprintf(f, "# %s\n", m); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(f,
		"index,t_system_s,t_audio_s,frames_consumed,queued_bytes,bracket_us,in_warmup,in_cooldown,drained,wide_bracket,used"); err != nil {
		return err
	}
	for i, s := range samples {
		if _, err := fmt.Fprintf(f, "%d,%.9f,%.9f,%d,%d,%.3f,%t,%t,%t,%t,%t\n",
			i, s.tSystem, s.tAudio, s.frames, s.queued, float64(s.bracket)/1000,
			s.inWarmup, s.inCooldown, s.drained, s.wideBracket, s.used()); err != nil {
			return err
		}
	}
	return f.Sync()
}

func fasterSlower(ppm float64) string {
	switch {
	case ppm > 0:
		return "faster"
	case ppm < 0:
		return "slower"
	default:
		return "exactly at nominal"
	}
}

func meanSD(v []float64) (float64, float64) {
	var sum float64
	for _, x := range v {
		sum += x
	}
	mean := sum / float64(len(v))
	if len(v) < 2 {
		return mean, 0
	}
	var ss float64
	for _, x := range v {
		ss += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(ss / float64(len(v)-1))
}

// rms is the root mean square, used to pool the per-segment standard errors
// into the scatter those segments would show even at a perfectly constant rate.
func rms(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var ss float64
	for _, x := range v {
		ss += x * x
	}
	return math.Sqrt(ss / float64(len(v)))
}

func minOf(v []float64) float64 {
	m := v[0]
	for _, x := range v {
		if x < m {
			m = x
		}
	}
	return m
}

func maxOf(v []float64) float64 {
	m := v[0]
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}
