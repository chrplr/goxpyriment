// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const sentence = "Welcome to Goxpy! I hope you like to vibe-code experiments!"

func main() {
	// 1. Initialize experiment
	exp := control.NewExperiment("RSVP Text Test", 800, 600, false, control.Black, control.White, 48)
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()

	if err := exp.SetVSync(1); err != nil {
		log.Printf("Warning: could not enable VSync: %v", err)
	}

	words := strings.Fields(sentence)

	exp.AddDataVariableNames([]string{"word_index", "word", "target_on_ms", "actual_onset_ms", "actual_offset_ms"})

	fmt.Printf("Presenting %d words...\n", len(words))

	// 2. Present the stream
	userEvents, timingLogs, err := stimuli.PresentStreamOfText(
		exp.Screen,
		words,
		300*time.Millisecond,
		100*time.Millisecond,
		0, 0,
		control.White,
	)
	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("Stream failed: %v", err)
	}

	// 3. Save and print timing results
	fmt.Println("\n--- Presentation Report ---")
	for _, tl := range timingLogs {
		onsetMS := tl.ActualOnset.Milliseconds()
		offsetMS := tl.ActualOffset.Milliseconds()
		targetMS := tl.TargetOn.Milliseconds()
		fmt.Printf("Word %d (%q): Target %dms | Onset: %dms | Offset: %dms\n",
			tl.Index, words[tl.Index], targetMS, onsetMS, offsetMS)
		exp.Data.Add(tl.Index, words[tl.Index], targetMS, onsetMS, offsetMS)
	}

	fmt.Println("\n--- User Input Captured ---")
	for _, ev := range userEvents {
		fmt.Printf("Event type %v at %v relative to start\n", ev.Event.Type, ev.Timestamp)
	}
}
