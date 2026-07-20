// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// test_stream_mixed demonstrates PresentStreamOfStimuli: a single stream that
// freely mixes visual and audio stimulus types (text, picture, shape, tone,
// sound). Visual elements are redrawn each frame; audio elements are triggered
// once and hold the previous frame for their slot, so the picture placed just
// before a tone stays on screen while the tone plays.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	// NewExperimentFromFlags parses -w/-d/-s and fully initializes the
	// experiment (SDL, window, audio, font, data file) — do NOT call
	// Initialize() again afterwards.
	exp := control.NewExperimentFromFlags("Mixed Stream Test", control.Black, control.White, 48)
	defer exp.End()

	if err := exp.SetVSync(1); err != nil {
		log.Printf("Warning: could not enable VSync: %v", err)
	}

	// Build the audio stimuli and bind them to the audio device. Audio elements
	// MUST be preloaded on the device before the stream call.
	tone := stimuli.NewTone(440.0, 400, 0.5)
	if err := tone.PreloadDevice(exp.AudioDevice); err != nil {
		log.Fatalf("preloading tone: %v", err)
	}
	ping := stimuli.NewSoundFromMemory(assets_embed.CorrectWav)
	if err := ping.PreloadDevice(exp.AudioDevice); err != nil {
		log.Fatalf("preloading sound: %v", err)
	}

	// A heterogeneous sequence: text, picture (held during the tone), a shape,
	// then a WAV sound (held on the shape).
	elements := []stimuli.StreamElement{
		{Stimulus: stimuli.NewTextLine("Ready?", 0, 0, control.White), DurationOn: 600 * time.Millisecond, DurationOff: 200 * time.Millisecond},
		{Stimulus: stimuli.NewPictureFromMemory(assets_embed.LogoPNG, 0, 0), DurationOn: 800 * time.Millisecond, DurationOff: 0},
		{Stimulus: tone, DurationOn: 400 * time.Millisecond, DurationOff: 200 * time.Millisecond},
		{Stimulus: stimuli.NewCircle(80, control.Red), DurationOn: 800 * time.Millisecond, DurationOff: 0},
		{Stimulus: ping, DurationOn: 500 * time.Millisecond, DurationOff: 300 * time.Millisecond},
		{Stimulus: stimuli.NewTextLine("Done.", 0, 0, control.White), DurationOn: 600 * time.Millisecond, DurationOff: 0},
	}

	exp.AddDataVariableNames([]string{"index", "target_on_ms", "actual_onset_ms", "actual_offset_ms"})

	fmt.Printf("Presenting %d mixed elements...\n", len(elements))

	userEvents, timingLogs, err := stimuli.PresentStreamOfStimuli(exp.Screen, elements, 0, 0)
	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("Stream failed: %v", err)
	}

	fmt.Println("\n--- Presentation Report ---")
	var ref uint64
	if len(timingLogs) > 0 {
		ref = timingLogs[0].OnsetNS // stream start on the SDL clock; OnsetNS/OffsetNS are the authoritative timing (see TimingLog)
	}
	for _, tl := range timingLogs {
		onsetMS := int64(tl.OnsetNS-ref) / 1_000_000
		offsetMS := int64(tl.OffsetNS-ref) / 1_000_000
		fmt.Printf("Element %d: Target %dms | Onset: %dms | Offset: %dms | OnsetNS: %d\n",
			tl.Index, tl.TargetOn.Milliseconds(), onsetMS, offsetMS, tl.OnsetNS)
		exp.Data.Add(tl.Index, tl.TargetOn.Milliseconds(), onsetMS, offsetMS)
	}

	fmt.Println("\n--- User Input Captured ---")
	for _, ev := range userEvents {
		fmt.Printf("Event type %v at %v relative to start\n", ev.Event.Type, ev.Timestamp)
	}
}
