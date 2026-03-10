// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
package main

import (
	"flag"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"

	"github.com/Zyko0/go-sdl3/sdl"
)

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	// 1. Create and initialize the experiment
	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}
	exp := control.NewExperiment("Mouse Audio Feedback", width, height, fullscreen)
	exp.SubjectID = *subject
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

