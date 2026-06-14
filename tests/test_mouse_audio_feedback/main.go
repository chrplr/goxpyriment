// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
package main

import (
	"log"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	// 1. Create and initialize the experiment
	exp := control.NewExperimentFromFlags("Mouse Audio Feedback", control.Black, control.White, 32)
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
			return err // Likely control.EndLoop for ESC or Quit
		}

		if key == control.K_ESCAPE {
			return control.EndLoop
		}

		if btn == uint32(control.BUTTON_LEFT) {
			feedbackText.Text = "Left button pressed! Playing ping sound."
			if err := stimuli.PlayPing(exp.AudioDevice); err != nil {
				log.Printf("Error playing ping sound: %v", err)
			}
		} else if btn == uint32(control.BUTTON_RIGHT) {
			feedbackText.Text = "Right button pressed! Playing buzzer sound."
			if err := stimuli.PlayBuzzer(exp.AudioDevice); err != nil {
				log.Printf("Error playing buzzer sound: %v", err)
			}
		}

		clock.Wait(10) // Small delay to prevent busy-waiting

		return nil
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
