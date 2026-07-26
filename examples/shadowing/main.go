// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// shadowing demonstrates a vocal shadowing latency task.
//
// Each trial plays an audio stimulus while the participant repeats it aloud
// as quickly as possible. The software measures the latency between audio onset
// and the participant's vocal onset via a microphone voice key.
//
// The audio stimulus and a text label are presented simultaneously using
// PlaySyncedWithFlip, which pauses the audio device, pre-fills the buffer,
// executes the VSYNC-locked screen flip, then immediately resumes — so the
// speaker starts within one audio callback period (≤ frames/sampleRate) of the
// visual onset. Use exp.SetAudioSampleFrames(128) before Initialize() to reduce
// this bound to ≤ 2.9 ms at 44100 Hz.
//
// Usage:
//
//	go run main.go -w -s 1
//
// Flags: -w windowed mode, -d N display index, -s N subject ID.
//
// Threshold calibration: raise the threshold if you get false triggers from
// the playback audio bleeding into the microphone (this happens with built-in
// laptop speakers — use headphones). Lower it if soft voices are missed.
//
// To use real speech files instead of generated tones, replace the NewTone
// calls with NewSound("path/to/word.wav") and call PreloadDevice(exp.AudioDevice).
package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// item pairs a text label with an audio stimulus and a visual label.
type item struct {
	label string
	tone  *stimuli.Tone
	text  *stimuli.TextLine
}

func main() {
	threshold := flag.Float64("threshold", 0.03, "Voice-key amplitude threshold (0–1, F32LE RMS)")
	saveWav := flag.Bool("save-wav", true, "Save per-trial WAV files for offline verification")
	nReps := flag.Int("reps", 2, "Number of repetitions of the stimulus list")

	exp := control.NewExperimentFromFlags(
		"Shadowing",
		control.Black, control.White, 42,
	)
	defer exp.End()

	if err := exp.OpenMicrophone(nil); err != nil {
		log.Fatalf("cannot open microphone: %v\n(is a recording device connected?)", err)
	}

	vk := apparatus.NewVoiceKey(exp.Microphone, float32(*threshold), 128)

	// -------------------------------------------------------------------------
	// Audio stimuli — generated tones as stand-ins for real speech.
	//
	// To use real WAV files instead:
	//
	//   snd := stimuli.NewSound("stimuli/word_apple.wav")
	//   snd.PreloadDevice(exp.AudioDevice)
	//   // then call snd.PlaySyncedWithFlip(exp.Screen) in the trial loop
	// -------------------------------------------------------------------------
	items := []item{
		newItem("low", 200, exp),
		newItem("mid", 440, exp),
		newItem("high", 880, exp),
		newItem("rising", 330, exp),
		newItem("falling", 660, exp),
	}

	fixCross := stimuli.NewFixCross(30, 3, control.White)
	stimuli.PreloadVisualOnScreen(exp.Screen, fixCross)
	for _, it := range items {
		stimuli.PreloadVisualOnScreen(exp.Screen, it.text)
	}

	exp.AddDataVariableNames([]string{"trial", "rep", "label", "shadowing_ms", "detected"})

	exp.ShowInstructions(
		"Shadowing Task\n\n" +
			"You will hear a tone and see its label.\n" +
			"Repeat the label aloud as quickly as possible.\n\n" +
			"Use headphones to avoid microphone feedback.\n\n" +
			"Press SPACE to begin.",
	)

	exp.Run(func() error {
		trial := 0
		for rep := range *nReps {
			for _, it := range items {
				trial++

				// Fixation cross (500 ms)
				exp.ShowTimed(fixCross, 500)

				// Blank (200 ms)
				exp.Blank(200)

				// Arm voice key: flush mic buffer and start capturing.
				// This happens before PlaySyncedWithFlip so the mic is already
				// running when the audio device is resumed.
				captureStartNS, armErr := vk.Arm()
				if armErr != nil {
					log.Printf("trial %d: microphone arm failed: %v", trial, armErr)
					exp.Data.Add(trial, rep+1, it.label, -1, false)
					continue
				}

				// Draw text label to the backbuffer, then call PlaySyncedWithFlip:
				// it pauses the audio device, pre-fills the tone, executes the
				// VSYNC flip (text becomes visible), then resumes audio playback.
				// stimulusNS is the VSYNC flip timestamp — both the screen update
				// and audio onset are anchored to this moment.
				exp.Screen.Clear()
				it.text.Draw(exp.Screen)
				stimulusNS, syncErr := it.tone.PlaySyncedWithFlip(exp.Screen)
				if syncErr != nil {
					log.Printf("trial %d: PlaySyncedWithFlip failed: %v", trial, syncErr)
					exp.Data.Add(trial, rep+1, it.label, -1, false)
					continue
				}

				// Wait for voice onset (3-second window).
				onsetNS, pcm, vkErr := vk.WaitOnset(captureStartNS, 3000, 1500)

				var shadowingMs int64
				detected := vkErr == nil
				if detected {
					shadowingMs = int64(onsetNS-stimulusNS) / 1_000_000
				}

				exp.Data.Add(trial, rep+1, it.label, shadowingMs, detected)

				if *saveWav && len(pcm) > 0 {
					wavPath := filepath.Join(
						exp.OutputDirectory,
						fmt.Sprintf("sub-%03d_trial-%03d_rep%d_%s.wav",
							exp.SubjectID, trial, rep+1, it.label),
					)
					if werr := apparatus.WriteWAV(wavPath, exp.Microphone.Spec, pcm); werr != nil {
						log.Printf("trial %d: could not save WAV: %v", trial, werr)
					}
				}

				if detected {
					log.Printf("trial %03d  rep%d  %-8s  shadowing latency = %d ms",
						trial, rep+1, it.label, shadowingMs)
				} else {
					log.Printf("trial %03d  rep%d  %-8s  no response (timeout)",
						trial, rep+1, it.label)
				}

				exp.Blank(500)
			}
		}
		return control.EndLoop
	})
}

// newItem constructs one shadowing item: a tone + centred text label.
func newItem(label string, freqHz float64, exp *control.Experiment) item {
	tone := stimuli.NewTone(freqHz, 400, 0.5) // 400 ms tone, 50% amplitude
	if err := tone.PreloadDevice(exp.AudioDevice); err != nil {
		log.Fatalf("cannot preload tone %q: %v", label, err)
	}
	return item{
		label: label,
		tone:  tone,
		text:  stimuli.NewTextLine(label, 0, 0, control.White),
	}
}
