// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
package main

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	// 1. Initialize experiment (loads SDL binaries, calls sdl.Init, creates window)
	exp := control.NewExperiment("RSVP Test", 800, 600, false, control.Black, control.White, 32)
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()

	if err := exp.SetVSync(1); err != nil {
		log.Printf("Warning: could not enable VSync: %v", err)
	}

	// 2. Prepare the list of images from the assets folder
	assetFiles, err := filepath.Glob(filepath.Join("assets", "*.png"))
	if err != nil || len(assetFiles) == 0 {
		log.Fatal("no PNG files found in assets/")
	}

	pics := make([]stimuli.VisualStimulus, len(assetFiles))
	for i, path := range assetFiles {
		pics[i] = stimuli.NewPicture(path, 0, 0)
	}
	elements := stimuli.MakeRegularVisualStream(pics, 100*time.Millisecond, 50*time.Millisecond)

	exp.AddDataVariableNames([]string{"image_index", "filename", "target_on_ms", "actual_onset_ms", "actual_offset_ms"})

	fmt.Println("Starting stream... Press keys to test logging.")

	// 3. Run the presentation, centered at screen center (0, 0 in center-based coords)
	userEvents, timingLogs, err := stimuli.PresentStreamOfImages(exp.Screen, elements, 0, 0)
	if err != nil {
		log.Fatalf("Stream failed: %v", err)
	}

	// 4. Save and print timing results
	fmt.Println("\n--- Presentation Report ---")
	for _, tl := range timingLogs {
		onsetMS := tl.ActualOnset.Milliseconds()
		offsetMS := tl.ActualOffset.Milliseconds()
		targetMS := tl.TargetOn.Milliseconds()
		fmt.Printf("Image %d: Target %dms | Actual Onset: %dms | Actual Offset: %dms\n",
			tl.Index, targetMS, onsetMS, offsetMS)
		exp.Data.Add(tl.Index, assetFiles[tl.Index], targetMS, onsetMS, offsetMS)
	}

	fmt.Println("\n--- User Input Captured ---")
	for _, ev := range userEvents {
		fmt.Printf("Event type %v at %v relative to start\n", ev.Event.Type, ev.Timestamp)
	}
}
