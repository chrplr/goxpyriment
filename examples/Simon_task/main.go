// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"fmt"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"
	"math/rand"
	"time"
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
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	develop := flag.Bool("d", false, "Developer mode (windowed 1024x768)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	// 1. Create and initialize the experiment
	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 768, false
	}
	exp := control.NewExperiment("Simon Task", width, height, fullscreen, control.Black, control.White, 32)
	exp.SubjectID = *subject
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
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
	rand.Shuffle(len(trials), func(i, j int) {
		trials[i], trials[j] = trials[j], trials[i]
	})

	instrText := fmt.Sprintf("In this experiment, you will see red or green squares appearing to the left or right of the center.\n\nYour task is to identify the COLOR of the square as quickly as possible:\n\n- If the square is RED, press 'F' (left index finger)\n- If the square is GREEN, press 'J' (right index finger)\n\nA fixation cross will remain in the center of the screen.\nIf you make a mistake, the trial will be repeated later.\n\nPress the spacebar to start.")
	instructions := stimuli.NewTextBox(instrText, 1000, control.Point(0, 0), control.DefaultTextColor)

	// 4. Run the experiment logic
	err := exp.Run(func() error {
		// Instructions
		if err := instructions.Present(exp.Screen, true, true); err != nil {
			return err
		}
		var key control.Keycode
		var subErr error
		for {
			key, _, subErr = exp.HandleEvents()
			if subErr != nil {
				return subErr
			}
			if key == control.K_SPACE {
				break
			}
			clock.Wait(10)
		}

		trialCount := 0
		successfulCount := 0

		for successfulCount < NTrials && len(trials) > 0 {
			t := trials[0]
			trials = trials[1:]
			trialCount++

			// Fixation (stays on screen)
			if err := fixation.Present(exp.Screen, true, true); err != nil {
				return err
			}
			// Random delay (fixation cross remains)
			delay := 500 + rand.Intn(1000) // 500 to 1500 ms
			clock.Wait(delay)

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
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := fixation.Draw(exp.Screen); err != nil {
				return err
			}
			if err := stim.Draw(exp.Screen); err != nil {
				return err
			}
			if err := exp.Screen.Update(); err != nil {
				return err
			}

			// Wait for response
			startTime := clock.GetTime()
			responded := false
			var rt int64
			var responseKey control.Keycode
			var correct bool

			for !responded {
				responseKey, _, subErr = exp.HandleEvents()
				if subErr != nil {
					return subErr
				}

				if responseKey == RedKey || responseKey == GreenKey {
					rt = clock.GetTime() - startTime
					responded = true

					if t.color == "red" && responseKey == RedKey {
						correct = true
					} else if t.color == "green" && responseKey == GreenKey {
						correct = true
					} else {
						correct = false
					}
				}
				clock.Wait(1)
			}

			// Congruency:
			// Red('F'=left) on Left OR Green('J'=right) on Right -> Congruent
			congruency := "incongruent"
			if (t.color == "red" && t.position == "left") || (t.color == "green" && t.position == "right") {
				congruency = "congruent"
			}

			exp.Data.Add([]interface{}{trialCount, t.color, t.position, responseKey, rt, correct, congruency})
			fmt.Printf("Subject %d, Trial %d: Color=%s, Pos=%s, Key=%d, RT=%d, Correct=%v, Congruency=%s\n", exp.SubjectID, trialCount, t.color, t.position, responseKey, rt, correct, congruency)

			if !correct {
				if err := exp.Audio.PlayBuzzer(); err != nil {
					log.Printf("Warning: buzzer playback failed: %v", err)
				}
				// Repeat trial: add back to trials slice at a random position
				insertPos := rand.Intn(len(trials) + 1)
				trials = append(trials[:insertPos], append([]trialDef{t}, trials[insertPos:]...)...)
				
				// Optional: Show error feedback
				errorStim := stimuli.NewTextLine("WRONG!", 0, 0, control.White)
				errorStim.Present(exp.Screen, true, true)
				clock.Wait(1000)
			} else {
				successfulCount++
			}

			// Inter-trial interval (fixation cross remains)
			if err := fixation.Present(exp.Screen, true, true); err != nil {
				return err
			}
			clock.Wait(500)
		}

		// Explicitly save results after the loop
		if err := exp.Data.Save(); err != nil {
			log.Printf("Warning: failed to save data file: %v", err)
		}

		// Final message
		finishText := "Experiment complete!\n\nThank you for your participation.\n\nPress space to exit."
		finishStim := stimuli.NewTextBox(finishText, 800, control.Point(0, 0), control.DefaultTextColor)
		finishStim.Present(exp.Screen, true, true)
		for {
			key, _, subErr = exp.HandleEvents()
			if subErr != nil {
				return subErr
			}
			if key == control.K_SPACE {
				break
			}
			clock.Wait(10)
		}

		return control.EndLoop
	})

	if err != nil && err != control.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
