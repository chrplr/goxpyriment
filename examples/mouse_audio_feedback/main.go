// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
package main

import (
	"flag"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"

	"github.com/Zyko0/go-sdl3/sdl"
)

func main() {
	fullscreen := flag.Bool("F", false, "Launch in fullscreen display mode")
	flag.Parse()

	// 1. Create and initialize the experiment
	exp := control.NewExperiment("Mouse Audio Feedback", 800, 600, *fullscreen)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// 2. Prepare stimuli
	feedbackText := stimuli.NewTextLine("Click mouse buttons!", 0, 0, control.White)

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		// Update and present the feedback text
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := feedbackText.Draw(exp.Screen); err != nil {
			return err
		}
		if err := exp.Screen.Update(); err != nil {
			return err
		}

		// Handle events
		key, btn, err := exp.HandleEvents()
		if err != nil {
			return err // Likely sdl.EndLoop for ESC or Quit
		}

		if key == sdl.K_ESCAPE {
			return sdl.EndLoop
		}

		if btn == uint32(sdl.BUTTON_LEFT) {
			feedbackText.Text = "Left button pressed! Playing ping sound."
			if err := stimuli.PlayPing(exp.AudioDevice); err != nil {
				log.Printf("Error playing ping sound: %v", err)
			}
		} else if btn == uint32(sdl.BUTTON_RIGHT) {
			feedbackText.Text = "Right button pressed! Playing buzzer sound."
			if err := stimuli.PlayBuzzer(exp.AudioDevice); err != nil {
				log.Printf("Error playing buzzer sound: %v", err)
			}
		}

		sdl.Delay(10) // Small delay to prevent busy-waiting

		return nil
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}

