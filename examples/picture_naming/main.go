// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// picture_naming demonstrates a vocal naming latency task.
//
// Each trial shows a picture and the participant names it aloud as quickly
// as possible. The software records audio via the microphone and detects when
// the participant starts speaking using an amplitude threshold (voice key).
// Reaction time = voice onset − image flip, measured on the SDL3 nanosecond clock.
//
// Audio from each trial is saved as a .wav file for offline verification — an
// essential step because voice keys can be triggered by breaths or room noise.
//
// Usage:
//
//	go run main.go -w -s 1
//
// Flags: -w windowed mode, -d N display index, -s N subject ID.
//
// Threshold calibration: if you get spuriously short RTs (< 100 ms), raise
// the threshold. If responses are missed, lower it. A value of 0.02–0.05 is
// a good starting point for a quiet room.
package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// trialItem pairs a label with the visual stimuli to display.
type trialItem struct {
	label string
	rect  *stimuli.Rectangle
	text  *stimuli.TextLine
}

func main() {
	// Register experiment-specific flags before calling NewExperimentFromFlags.
	threshold := flag.Float64("threshold", 0.03, "Voice-key amplitude threshold (0–1, F32LE RMS)")
	saveWav := flag.Bool("save-wav", true, "Save per-trial WAV files for offline verification")

	exp := control.NewExperimentFromFlags(
		"Picture Naming",
		control.Black, control.White, 36,
	)
	defer exp.End()

	// Open the default microphone with the standard recording spec.
	if err := exp.OpenMicrophone(nil); err != nil {
		log.Fatalf("cannot open microphone: %v\n(is a recording device connected?)", err)
	}

	vk := apparatus.NewVoiceKey(exp.Microphone, float32(*threshold), 128)
	vk.OnOnset = func() { exp.Screen.ClearAndUpdate() }

	// -------------------------------------------------------------------------
	// Build trial stimuli.
	//
	// This demo uses colored rectangles with text labels as placeholders.
	// In a real experiment, replace with:
	//
	//   pic := stimuli.NewPicture("stimuli/apple.png", 0, 0)
	//
	// or embed images in the binary:
	//
	//   //go:embed stimuli/*.png
	//   var stimuliFS embed.FS
	//   data, _ := stimuliFS.ReadFile("stimuli/apple.png")
	//   pic := stimuli.NewPictureFromMemory(data, 0, 0)
	// -------------------------------------------------------------------------
	items := []trialItem{
		makeItem("apple", control.Red),
		makeItem("house", control.Blue),
		makeItem("tree", control.Green),
		makeItem("car", control.Yellow),
		makeItem("dog", sdl.Color{R: 200, G: 100, B: 50, A: 255}),
	}

	// Preload all stimuli onto the GPU to avoid jitter on first draw.
	for _, it := range items {
		stimuli.PreloadVisualOnScreen(exp.Screen, it.rect)
		stimuli.PreloadVisualOnScreen(exp.Screen, it.text)
	}

	fixCross := stimuli.NewFixCross(30, 3, control.White)
	stimuli.PreloadVisualOnScreen(exp.Screen, fixCross)

	exp.AddDataVariableNames([]string{"trial", "label", "rt_ms", "detected"})

	exp.ShowInstructions(
		"Picture Naming Task\n\n" +
			"You will see colored squares with a word label.\n" +
			"Name each one aloud as quickly as possible.\n\n" +
			"Press SPACE to begin.",
	)

	exp.Run(func() error {
		for trial, item := range items {
			// Fixation cross (500 ms)
			exp.ShowTimed(fixCross, 500)

			// Blank (200 ms)
			exp.Blank(200)

			// Arm the voice key just before the image flip so capture starts
			// as close as possible to stimulus onset.
			captureStartNS, armErr := vk.Arm()
			if armErr != nil {
				log.Printf("trial %d: microphone arm failed: %v", trial+1, armErr)
				exp.Data.Add(trial+1, item.label, -1, false)
				continue
			}

			// Show picture: clear + draw rectangle + draw label + flip.
			exp.Screen.Clear()
			item.rect.Draw(exp.Screen)
			item.text.Draw(exp.Screen)
			imageOnsetNS, _ := exp.Screen.FlipTS()

			// Wait for voice onset (2-second response window); record 1500 ms
			// after onset to capture the full vocal response.
			onsetNS, pcm, vkErr := vk.WaitOnset(captureStartNS, 2000, 1500)

			var rtMs int64
			detected := vkErr == nil
			if detected {
				rtMs = int64(onsetNS-imageOnsetNS) / 1_000_000
			}

			exp.Data.Add(trial+1, item.label, rtMs, detected)

			// Save WAV for offline verification, with an "onset" cue marker at the
			// precise sample where the voice key triggered.
			if *saveWav && len(pcm) > 0 {
				wavPath := filepath.Join(
					exp.OutputDirectory,
					fmt.Sprintf("sub-%03d_trial-%02d_%s.wav",
						exp.SubjectID, trial+1, item.label),
				)
				var cues []apparatus.WAVCue
				if detected {
					onsetSample := uint32((onsetNS - captureStartNS) * uint64(exp.Microphone.Spec.Freq) / 1_000_000_000)
					cues = append(cues, apparatus.WAVCue{SampleOffset: onsetSample, Label: "onset"})
				}
				if werr := apparatus.WriteWAV(wavPath, exp.Microphone.Spec, pcm, cues...); werr != nil {
					log.Printf("trial %d: could not save WAV: %v", trial+1, werr)
				}
			}

			if detected {
				log.Printf("trial %02d  %-10s  RT = %d ms", trial+1, item.label, rtMs)
			} else {
				log.Printf("trial %02d  %-10s  no response (timeout)", trial+1, item.label)
			}

			// Inter-trial interval
			exp.Blank(500)
		}
		return control.EndLoop
	})
}

// makeItem creates a placeholder stimulus: a coloured square with a text label.
// Replace with stimuli.NewPicture in a real experiment.
func makeItem(label string, color sdl.Color) trialItem {
	return trialItem{
		label: label,
		rect:  stimuli.NewRectangle(0, 0, 300, 300, color),
		text:  stimuli.NewTextLine(label, 0, 0, control.White),
	}
}
