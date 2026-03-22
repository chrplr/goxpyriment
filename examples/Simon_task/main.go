// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"fmt"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	NTrials        = 100
	RedKey         = control.K_F
	GreenKey       = control.K_J
	SquareSize     = 100
	SquareOffset   = 300 // distance from center
	FixationRadius = 5
)

type trialDef struct {
	color    string // "red" or "green"
	position string // "left" or "right"
}

func main() {
	exp := control.NewExperimentFromFlags("Simon Task", control.Black, control.White, 32)
	defer exp.End()

	// Set logical size for consistent centering
	if err := exp.SetLogicalSize(1368, 1024); err != nil {
		log.Printf("Warning: failed to set logical size: %v", err)
	}

	exp.AddDataVariableNames([]string{"trial", "color", "position", "key", "rt", "correct", "congruency"})

	// 2. Prepare stimuli
	fixation := stimuli.NewFixCross(25, 2, control.White)

	// We'll create the square stimulus on the fly during the trial loop
	// or pre-create them for efficiency.
	stimRedLeft := stimuli.NewRectangle(-SquareOffset, 0, SquareSize, SquareSize, control.Red)
	stimRedRight := stimuli.NewRectangle(SquareOffset, 0, SquareSize, SquareSize, control.Red)
	stimGreenLeft := stimuli.NewRectangle(-SquareOffset, 0, SquareSize, SquareSize, control.Green)
	stimGreenRight := stimuli.NewRectangle(SquareOffset, 0, SquareSize, SquareSize, control.Green)

	// 3. Prepare design
	// We want 100 trials. We'll start with 100 balanced trials.
	var trials []trialDef
	for i := 0; i < NTrials/4; i++ {
		trials = append(trials, trialDef{"red", "left"})
		trials = append(trials, trialDef{"red", "right"})
		trials = append(trials, trialDef{"green", "left"})
		trials = append(trials, trialDef{"green", "right"})
	}
	// Shuffle initial trials
	design.ShuffleList(trials)

	instrText := fmt.Sprintf("In this experiment, you will see red or green squares appearing to the left or right of the center.\n\nYour task is to identify the COLOR of the square as quickly as possible:\n\n- If the square is RED, press 'F' (left index finger)\n- If the square is GREEN, press 'J' (right index finger)\n\nA fixation cross will remain in the center of the screen.\nIf you make a mistake, the trial will be repeated later.\n\nPress the spacebar to start.")

	// 4. Run the experiment logic
	err := exp.Run(func() error {
		// Instructions
		exp.ShowInstructions(instrText)

		trialCount := 0
		successfulCount := 0

		for successfulCount < NTrials && len(trials) > 0 {
			t := trials[0]
			trials = trials[1:]
			trialCount++

			// Fixation (stays on screen)
			exp.Show(fixation)
			// Random delay (fixation cross remains)
			delay := design.RandInt(500, 1499) // 500 to 1499 ms
			exp.Wait(delay)

			// Stimulus selection
			var stim *stimuli.Rectangle
			if t.color == "red" {
				if t.position == "left" {
					stim = stimRedLeft
				} else {
					stim = stimRedRight
				}
			} else {
				if t.position == "left" {
					stim = stimGreenLeft
				} else {
					stim = stimGreenRight
				}
			}

			// Draw BOTH fixation and stimulus
			_ = exp.Screen.Clear()
			_ = fixation.Draw(exp.Screen)
			_ = stim.Draw(exp.Screen)
			_ = exp.Screen.Update()

			// Wait for response
			var responseKey control.Keycode
			var rt int64
			var correct bool
			responseKey, rt, _ = exp.Keyboard.WaitKeysRT([]control.Keycode{RedKey, GreenKey}, -1)

			if t.color == "red" && responseKey == RedKey {
				correct = true
			} else if t.color == "green" && responseKey == GreenKey {
				correct = true
			}

			// Congruency:
			// Red('F'=left) on Left OR Green('J'=right) on Right -> Congruent
			congruency := "incongruent"
			if (t.color == "red" && t.position == "left") || (t.color == "green" && t.position == "right") {
				congruency = "congruent"
			}

			exp.Data.Add(trialCount, t.color, t.position, responseKey, rt, correct, congruency)
			fmt.Printf("Subject %d, Trial %d: Color=%s, Pos=%s, Key=%d, RT=%d, Correct=%v, Congruency=%s\n", exp.SubjectID, trialCount, t.color, t.position, responseKey, rt, correct, congruency)

			if !correct {
				_ = exp.Audio.PlayBuzzer()
				// Repeat trial: add back to trials slice at a random position
				insertPos := design.RandInt(0, len(trials))
				trials = append(trials[:insertPos], append([]trialDef{t}, trials[insertPos:]...)...)

				// Optional: Show error feedback
				errorStim := stimuli.NewTextLine("WRONG!", 0, 0, control.White)
				exp.Show(errorStim)
				exp.Wait(1000)
			} else {
				successfulCount++
			}

			// Inter-trial interval (fixation cross remains)
			exp.Show(fixation)
			exp.Wait(500)
		}

		// Explicitly save results after the loop
		_ = exp.Data.Save()

		// Final message
		finishText := "Experiment complete!\n\nThank you for your participation.\n\nPress SPACE to exit."
		exp.ShowInstructions(finishText)

		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
