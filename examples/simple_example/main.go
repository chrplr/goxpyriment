// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"fmt"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"
)

//go:embed assets/bonjour.wav
var bonjourWav []byte

func main() {
	// 1. Create and initialize the experiment
	exp := control.NewExperimentFromFlags("My First Go Experiment", control.Black, control.White, 32)
	defer exp.End()

	// 2. Prepare design
	block := design.NewBlock("Main Block")
	for i := 0; i < 5; i++ {
		trial := design.NewTrial()
		trial.Factors["color"] = "white"
		block.AddTrial(trial, 1, false)
	}

		// 3. Prepare stimuli
	instr := stimuli.NewTextBox("Press any key to start the experiment", 600, control.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
	fixation := stimuli.NewTextLine("+", 0, 0, control.DefaultTextColor)
	rect := stimuli.NewRectangle(0, 0, 100, 100, control.Red)
	finish := stimuli.NewTextBox("Experiment Finished! Press any key to exit.", 600, control.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
	sound := stimuli.NewSoundFromMemory(bonjourWav)

	if err := sound.PreloadDevice(exp.AudioDevice); err != nil {
		log.Printf("Warning: failed to load sound: %v", err)
	}

	// 4. Run the experiment logic
	err := exp.Run(func() error {
		// Instructions
		if err := exp.Show(instr); err != nil {
			return err
		}
		if _, err := exp.Keyboard.Wait(); err != nil {
			return err
		}

		// Play sound at start
		if err := sound.Play(); err != nil {
			return err
		}


		// Loop through trials
		for _, trial := range block.Trials {
			fmt.Printf("Running trial %d\n", trial.ID)

			// Fixation cross
			if err := exp.Show(fixation); err != nil {
				return err
			}
			clock.Wait(500)

			// Target stimulus
			if err := exp.Show(rect); err != nil {
				return err
			}

			// Wait for response
			startTime := clock.GetTime()
			_, err := exp.Keyboard.Wait()
			if err != nil {
				return err
			}
			rt := clock.GetTime() - startTime
			fmt.Printf("Reaction Time: %d ms\n", rt)

			// Clear screen between trials
			if err := exp.Blank(500); err != nil {
				return err
			}
		}

		// Finish
		if err := exp.Show(finish); err != nil {
			return err
		}
		if _, err := exp.Keyboard.Wait(); err != nil {
			return err
		}

		return control.EndLoop // Graceful exit
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
