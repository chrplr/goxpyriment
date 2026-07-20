// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// A C-major-ish ascending sequence of pure tones (Hz).
var frequencies = []float64{262, 294, 330, 349, 392, 440, 494, 523}

const (
	toneDurationMs = 200 // how long each tone is "on"
	isiMs          = 100 // silence between tones
	amplitude      = 0.5
)

func main() {
	// 1. Initialize experiment. A window is created so the stream can catch
	//    ESC / window-close, but the test is purely auditory.
	exp := control.NewExperiment("RSVP Tones Test", 800, 600, false, control.Black, control.White, 48)
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()

	// 2. Build the tones and bind each to the audio device (required before Play).
	sounds := make([]stimuli.AudioPlayable, len(frequencies))
	for i, freq := range frequencies {
		tone := stimuli.NewTone(freq, toneDurationMs, amplitude)
		if err := tone.PreloadDevice(exp.AudioDevice); err != nil {
			log.Fatalf("preloading tone %d (%.0f Hz): %v", i, freq, err)
		}
		sounds[i] = tone
	}

	exp.AddDataVariableNames([]string{"tone_index", "frequency_hz", "target_on_ms", "actual_onset_ms", "actual_offset_ms"})

	fmt.Printf("Playing %d tones (%d ms on, %d ms ISI)...\n", len(sounds), toneDurationMs, isiMs)

	// 3. Build and play the regular sound stream.
	elements := stimuli.MakeRegularSoundStream(
		sounds,
		toneDurationMs*time.Millisecond,
		isiMs*time.Millisecond,
	)

	userEvents, timingLogs, err := stimuli.PlayStreamOfSounds(elements)
	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("Stream failed: %v", err)
	}

	// 4. Save and print timing results.
	fmt.Println("\n--- Presentation Report ---")
	for _, tl := range timingLogs {
		onsetMS := tl.ActualOnset.Milliseconds()
		offsetMS := tl.ActualOffset.Milliseconds()
		targetMS := tl.TargetOn.Milliseconds()
		freq := frequencies[tl.Index]
		fmt.Printf("Tone %d (%.0f Hz): Target %dms | Onset: %dms | Offset: %dms\n",
			tl.Index, freq, targetMS, onsetMS, offsetMS)
		exp.Data.Add(tl.Index, freq, targetMS, onsetMS, offsetMS)
	}

	fmt.Println("\n--- User Input Captured ---")
	for _, ev := range userEvents {
		fmt.Printf("Event type %v at %v relative to start\n", ev.Event.Type, ev.Timestamp)
	}
}
