// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// test_naming is a vocal-response digit-naming task, structurally identical
// to examples/Naming-Stroop but without the colour/word conflict: the words
// "one" through "ten" are shown one at a time and the participant names each
// aloud. A voice key (apparatus.VoiceKey) detects speech onset from the
// microphone; reaction time = voice onset − word flip.
//
// Unlike Naming-Stroop, the voice-key amplitude threshold is not a fixed
// flag. Before the naming trials start, the participant is asked to stay
// silent for 10 seconds while a live VU meter shows the RMS input level,
// sampled every 100 ms. At the end of the recording, the experimenter can
// accept the sample or redo it (background noise, a cough, a chair creak,
// etc. can spoil a take). The threshold used for the naming trials is the
// 90th percentile of the recorded background-noise RMS distribution,
// multiplied by calibFactor — comfortably above ambient noise but still
// well below normal speech level; inspect the saved WAV files afterwards to
// see whether it worked.
//
// Usage:
//
//	go run main.go -w -s 1
//
// Flags: -w windowed mode, -d N display index, -s N subject ID.
package main

import (
	"fmt"
	"log"
	"math"
	"path/filepath"
	"sort"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Layout constants for the calibration VU meter.
const (
	meterWidth  = float32(140)
	meterHeight = float32(500)
	meterBottom = -meterHeight / 2
	// meterScale is the RMS amplitude that maps to a full-height bar.
	// Typical speech RMS peaks are well under this; raise it if the bar
	// pins at the top for normal speaking volume.
	meterScale = float32(0.3)
)

// Calibration constants: record calibDurationMs of silence, take the
// calibPercentile-th percentile of the observed RMS distribution as an
// estimate of the background-noise ceiling, and multiply by calibFactor to
// get a threshold comfortably above it.
const (
	calibDurationMs = 10_000
	calibPercentile = 0.90
	calibFactor     = 3.0
)

func main() {
	saveWav := true

	exp := control.NewExperimentFromFlags("Naming Digits", control.Black, control.White, 32)
	defer exp.End()

	if err := exp.OpenMicrophone(nil); err != nil {
		log.Fatalf("cannot open microphone: %v\n(is a recording device connected?)", err)
	}

	exp.AddDataVariableNames([]string{"trial", "digit", "rt_ms", "detected"})

	digits := []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}

	err := exp.Run(func() error {
		threshold, err := calibrateVoiceKeyThreshold(exp)
		if err != nil {
			return err
		}
		log.Printf("voice-key threshold set to %.4f (calibrated)", threshold)

		vk := apparatus.NewVoiceKey(exp.Microphone, threshold, 128)
		vk.OnOnset = func() { exp.Screen.ClearAndUpdate() }

		if ierr := exp.ShowInstructions(
			"Naming task\n\n" +
				"You will see a number word at the center of the screen.\n" +
				"Say it aloud, as quickly as possible.\n\n" +
				"Press SPACE to begin.",
		); ierr != nil {
			return ierr
		}

		for i, digit := range digits {
			trial := i + 1

			if berr := exp.Blank(500); berr != nil {
				return berr
			}

			stim := stimuli.NewTextLine(digit, 0, 0, control.White)
			stimuli.PreloadVisualOnScreen(exp.Screen, stim)

			// Arm the voice key just before the word flip so capture starts
			// as close as possible to stimulus onset.
			captureStartNS, armErr := vk.Arm()
			if armErr != nil {
				log.Printf("trial %d: microphone arm failed: %v", trial, armErr)
				exp.Data.Add(trial, digit, int64(-1), false)
				continue
			}

			// Show word: clear + draw + flip.
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := stim.Draw(exp.Screen); err != nil {
				return err
			}
			wordOnsetNS, _ := exp.Screen.FlipTS()

			// Wait for voice onset (3-second response window); record 1000 ms
			// after onset to capture the full vocal response.
			onsetNS, pcm, vkErr := vk.WaitOnset(captureStartNS, 3000, 1000)

			var rtMs int64
			detected := vkErr == nil
			if detected {
				rtMs = int64(onsetNS-wordOnsetNS) / 1_000_000
			} else {
				rtMs = -1
			}

			exp.Data.Add(trial, digit, rtMs, detected)

			if saveWav && len(pcm) > 0 {
				wavPath := filepath.Join(
					exp.OutputDirectory,
					fmt.Sprintf("sub-%03d_trial-%02d_%s.wav", exp.SubjectID, trial, digit),
				)
				var cues []apparatus.WAVCue
				if detected {
					onsetSample := uint32((onsetNS - captureStartNS) * uint64(exp.Microphone.Spec.Freq) / 1_000_000_000)
					cues = append(cues, apparatus.WAVCue{SampleOffset: onsetSample, Label: "onset"})
				}
				if werr := apparatus.WriteWAV(wavPath, exp.Microphone.Spec, pcm, cues...); werr != nil {
					log.Printf("trial %d: could not save WAV: %v", trial, werr)
				}
			}

			if detected {
				log.Printf("trial %02d  digit=%-6s  RT = %d ms", trial, digit, rtMs)
			} else {
				log.Printf("trial %02d  digit=%-6s  no response (timeout)", trial, digit)
			}

			// Inter-trial interval
			if berr := exp.Blank(500); berr != nil {
				return berr
			}
		}

		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}

// calibrateVoiceKeyThreshold records calibDurationMs of background silence
// (with a chance to redo the recording if it was spoiled), then sets the
// voice-key threshold to the calibPercentile-th percentile of the observed
// RMS distribution, times calibFactor.
func calibrateVoiceKeyThreshold(exp *control.Experiment) (float32, error) {
	for {
		samples, err := recordSilence(exp)
		if err != nil {
			return 0, err
		}

		p90 := percentile(samples, calibPercentile)
		threshold := p90 * calibFactor

		accept, err := confirmThreshold(exp, p90, threshold)
		if err != nil {
			return 0, err
		}
		if accept {
			return threshold, nil
		}
	}
}

// recordSilence shows a live VU meter for calibDurationMs while instructing
// the participant to stay silent, sampling RMS every 100 ms. Returns the
// recorded RMS samples.
func recordSilence(exp *control.Experiment) ([]float32, error) {
	mic := exp.Microphone
	if _, err := mic.StartCapture(); err != nil {
		return nil, fmt.Errorf("recordSilence: %w", err)
	}
	defer mic.StopCapture()

	frame := stimuli.NewRectangle(0, 0, meterWidth+8, meterHeight+8, control.White)
	track := stimuli.NewRectangle(0, 0, meterWidth, meterHeight, control.DarkGray)
	level := stimuli.NewRectangle(0, meterBottom, meterWidth-10, 0, control.Green)

	cursorHalfW := meterWidth/2 + 10
	maxLine := stimuli.NewLine(control.Point(-cursorHalfW, meterBottom), control.Point(cursorHalfW, meterBottom), control.Red, 3)

	title := stimuli.NewTextLine("Stay silent!", 0, meterHeight/2+60, control.Yellow)
	readout := stimuli.NewTextLine("listening...", 0, -meterHeight/2-50, control.White)

	stimuli.PreloadAllVisual(exp.Screen, []stimuli.VisualStimulus{
		frame, track, level, maxLine, title, readout,
	})

	var samples []float32
	var maxRMS, curRMS float32

	var windowBuf []byte
	windowStart := time.Now()
	readBuf := make([]byte, 4096)
	recordingStart := time.Now()
	duration := time.Duration(calibDurationMs) * time.Millisecond

	for time.Since(recordingStart) < duration {
		if _, _, err := exp.HandleEvents(); err != nil {
			return nil, err
		}

		n, rErr := mic.ReadSamples(readBuf)
		if rErr != nil {
			return nil, fmt.Errorf("recordSilence: %w", rErr)
		}
		if n > 0 {
			windowBuf = append(windowBuf, readBuf[:n]...)
		}

		if time.Since(windowStart) >= 100*time.Millisecond {
			if len(windowBuf) > 0 {
				curRMS = rmsLevel(windowBuf)
				samples = append(samples, curRMS)
				if curRMS > maxRMS {
					maxRMS = curRMS
				}
			}
			windowBuf = windowBuf[:0]
			windowStart = time.Now()

			remaining := calibDurationMs/1000 - int(time.Since(recordingStart).Seconds())
			readout.Text = fmt.Sprintf("level=%.4f   max=%.4f   %ds remaining", curRMS, maxRMS, remaining)
			// Force the texture to regenerate now that Text has changed.
			readout.Unload()
		}

		levelNorm := normalizeLevel(curRMS)
		level.Rect.H = meterHeight * levelNorm
		level.Position.Y = meterBottom + level.Rect.H/2

		maxY := meterBottom + meterHeight*normalizeLevel(maxRMS)
		maxLine.Start.Y, maxLine.End.Y = maxY, maxY

		if err := exp.Screen.Clear(); err != nil {
			return nil, err
		}
		for _, s := range []stimuli.VisualStimulus{frame, track, level, maxLine, title, readout} {
			if err := s.Draw(exp.Screen); err != nil {
				return nil, err
			}
		}
		if err := exp.Screen.Update(); err != nil {
			return nil, err
		}

		time.Sleep(15 * time.Millisecond)
	}

	return samples, nil
}

// confirmThreshold shows the computed threshold and asks whether to keep it
// or redo the silence recording. Returns true to accept, false to redo.
func confirmThreshold(exp *control.Experiment, p90, threshold float32) (bool, error) {
	text := fmt.Sprintf(
		"Calibration complete.\n\n"+
			"Background RMS: P%.0f = %.4f\n"+
			"Voice-key threshold (x %.1f) = %.4f\n\n"+
			"Press SPACE to keep this calibration,\nor R to redo the 10-second silent recording.",
		calibPercentile*100, p90, calibFactor, threshold,
	)
	tb := stimuli.NewTextBox(text, int32(float32(exp.Screen.Width)*0.80), sdl.FPoint{}, control.White)
	if err := exp.Screen.Clear(); err != nil {
		return false, err
	}
	if err := tb.Draw(exp.Screen); err != nil {
		return false, err
	}
	if err := exp.Screen.Update(); err != nil {
		return false, err
	}

	key, err := exp.Keyboard.WaitKeys([]sdl.Keycode{control.K_SPACE, control.K_R}, -1)
	if err != nil {
		return false, err
	}
	return key == control.K_SPACE, nil
}

// percentile returns the p-th percentile (0 <= p <= 1) of samples using the
// nearest-rank method. samples is not modified. Returns 0 for an empty slice.
func percentile(samples []float32, p float64) float32 {
	if len(samples) == 0 {
		return 0
	}
	sorted := make([]float32, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// normalizeLevel maps an RMS amplitude to a 0-1 bar height fraction, clamped.
func normalizeLevel(rms float32) float32 {
	v := rms / meterScale
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return v
}

// rmsLevel computes the RMS amplitude of a slice of F32LE PCM bytes.
func rmsLevel(buf []byte) float32 {
	n := len(buf) / 4
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		bits := uint32(buf[i*4]) |
			uint32(buf[i*4+1])<<8 |
			uint32(buf[i*4+2])<<16 |
			uint32(buf[i*4+3])<<24
		v := float64(math.Float32frombits(bits))
		sum += v * v
	}
	return float32(math.Sqrt(sum / float64(n)))
}
