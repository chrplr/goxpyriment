// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Naming-Stroop is a vocal-response version of the classic Stroop (1935)
// colour-naming task.
//
// Each trial shows a colour word rendered in a coloured font. The word and
// the ink colour are sometimes congruent (e.g. "RED" in red) and sometimes
// incongruent (e.g. "RED" in blue). The participant must name the ink
// colour aloud, as quickly as possible, ignoring the word itself.
//
// Unlike examples/Stroop_task (which records a key press), this version
// records the participant's vocal response through the microphone. A voice
// key (apparatus.VoiceKey) detects the onset of speech via an amplitude
// threshold; reaction time = voice onset − word flip, measured on the SDL3
// nanosecond clock. Audio for each trial is saved as a .wav file so onsets
// can be checked offline — an essential step, since voice keys can be
// triggered by breaths or room noise instead of the intended response.
//
// Usage:
//
//	go run main.go -w -s 1
//
// Flags: -w windowed mode, -d N display index, -s N subject ID.
//
// Threshold calibration: if you get spuriously short RTs (< 100 ms), raise
// the threshold. If responses are missed, lower it. A value of 0.02-0.05 is
// a good starting point for a quiet room.
package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// stroopTrial describes one trial: a word rendered in an ink colour.
type stroopTrial struct {
	word      string
	color     control.Color
	colorName string
}

func main() {
	threshold := flag.Float64("threshold", 0.03, "Voice-key amplitude threshold (0-1, F32LE RMS)")
	saveWav := flag.Bool("save-wav", true, "Save per-trial WAV files for offline verification")

	exp := control.NewExperimentFromFlags("Naming Stroop", control.Black, control.White, 48)
	defer exp.End()

	// Open the default microphone with the standard recording spec.
	if err := exp.OpenMicrophone(nil); err != nil {
		log.Fatalf("cannot open microphone: %v\n(is a recording device connected?)", err)
	}

	vk := apparatus.NewVoiceKey(exp.Microphone, float32(*threshold), 128)
	vk.OnOnset = func() { exp.Screen.ClearAndUpdate() }

	exp.AddDataVariableNames([]string{"trial", "word", "ink_color", "congruent", "rt_ms", "detected"})

	// Build the 4x4 word x ink-colour design (16 combinations).
	words := []string{"RED", "GREEN", "BLUE", "YELLOW"}
	colors := []control.Color{control.Red, control.Green, control.Blue, control.Yellow}
	colorNames := []string{"RED", "GREEN", "BLUE", "YELLOW"}

	var trials []stroopTrial
	for _, word := range words {
		for j, color := range colors {
			trials = append(trials, stroopTrial{word: word, color: color, colorName: colorNames[j]})
		}
	}
	design.ShuffleList(trials)

	fixCross := stimuli.NewFixCross(30, 3, control.White)
	stimuli.PreloadVisualOnScreen(exp.Screen, fixCross)

	exp.ShowInstructions(
		"Naming Stroop Task\n\n" +
			"You will see words written in colour.\n" +
			"Say the INK COLOUR aloud, as quickly as possible.\n" +
			"Ignore what the word says.\n\n" +
			"Press SPACE to begin.",
	)

	exp.Run(func() error {
		for i, t := range trials {
			trial := i + 1

			// Fixation cross (500 ms)
			exp.ShowTimed(fixCross, 500)

			// Blank (200 ms)
			exp.Blank(200)

			stim := stimuli.NewTextLine(t.word, 0, 0, t.color)
			stimuli.PreloadVisualOnScreen(exp.Screen, stim)

			// Arm the voice key just before the word flip so capture starts
			// as close as possible to stimulus onset.
			captureStartNS, armErr := vk.Arm()
			if armErr != nil {
				log.Printf("trial %d: microphone arm failed: %v", trial, armErr)
				congruent := t.word == t.colorName
				exp.Data.Add(trial, t.word, t.colorName, congruent, int64(-1), false)
				continue
			}

			// Show word: clear + draw + flip.
			exp.Screen.Clear()
			stim.Draw(exp.Screen)
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

			congruent := t.word == t.colorName
			exp.Data.Add(trial, t.word, t.colorName, congruent, rtMs, detected)

			// Save WAV for offline verification, with an "onset" cue marker at the
			// precise sample where the voice key triggered.
			if *saveWav && len(pcm) > 0 {
				wavPath := filepath.Join(
					exp.OutputDirectory,
					fmt.Sprintf("sub-%03d_trial-%02d_%s-%s.wav",
						exp.SubjectID, trial, t.word, t.colorName),
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
				log.Printf("trial %02d  word=%-6s ink=%-6s congruent=%-5v  RT = %d ms", trial, t.word, t.colorName, congruent, rtMs)
			} else {
				log.Printf("trial %02d  word=%-6s ink=%-6s congruent=%-5v  no response (timeout)", trial, t.word, t.colorName, congruent)
			}

			// Inter-trial interval
			exp.Blank(500)
		}

		return control.EndLoop
	})
}
