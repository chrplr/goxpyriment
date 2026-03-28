// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"fmt"
	"math"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Octagon locations (centered)
func getOctagonPoints(radius float32) []control.FPoint {
	points := make([]control.FPoint, 8)
	for i := 0; i < 8; i++ {
		// 0 is top, clockwise. Indices in study are 1-8, we use 0-7.
		angle := math.Pi/2 - float64(i)*(2*math.Pi/8)
		points[i] = control.FPoint{
			X: radius * float32(math.Cos(angle)),
			Y: radius * float32(math.Sin(angle)),
		}
	}
	return points
}

type Sequence struct {
	Name    string
	Base    []int // 8 locations
	Indices []int // 16 locations
}

func NewSequence(name string, base []int) Sequence {
	full := make([]int, 16)
	for i := 0; i < 8; i++ {
		full[i] = base[i]
		full[i+8] = base[i]
	}
	return Sequence{Name: name, Base: base, Indices: full}
}

// drawEnvironment renders stimuli directly on the main thread (inside exp.Run).
func drawEnvironment(exp *control.Experiment, dots []*stimuli.Circle, fixation *stimuli.FixCross, target *stimuli.Circle, activeIdx int) error {
	if err := exp.Screen.Clear(); err != nil {
		return err
	}
	// Draw background dots
	for i := 0; i < 8; i++ {
		if err := dots[i].Draw(exp.Screen); err != nil {
			return err
		}
	}
	// Draw fixation
	if err := fixation.Draw(exp.Screen); err != nil {
		return err
	}
	// Draw target if activeIdx >= 0
	if activeIdx >= 0 {
		if err := target.Draw(exp.Screen); err != nil {
			return err
		}
	}
	return exp.Screen.Update()
}

func flashSequence(exp *control.Experiment, dots []*stimuli.Circle, fixation *stimuli.FixCross, target *stimuli.Circle, indices []int) error {
	for _, idx := range indices {
		if err := drawEnvironment(exp, dots, fixation, target, idx); err != nil {
			return err
		}
		exp.Wait(500)
		if err := drawEnvironment(exp, dots, fixation, target, -1); err != nil {
			return err
		}
		exp.Wait(100)
	}
	return nil
}

func getGuess(exp *control.Experiment, dots []*stimuli.Circle, fixation *stimuli.FixCross, octagonPoints []control.FPoint) (int, int64, error) {
	startTime := clock.GetTime()
	// Ensure screen is updated with dots and fixation
	if err := drawEnvironment(exp, dots, fixation, nil, -1); err != nil {
		return -1, 0, err
	}

	for {
		btn, err := exp.Mouse.WaitPress()
		if err != nil {
			return -1, 0, err
		}

		if btn == 1 { // Left click
			mx, my := exp.Screen.MousePosition()
			for i, p := range octagonPoints {
				dx := mx - p.X
				dy := my - p.Y
				if dx*dx+dy*dy < 40*40 { // 40px radius click zone
					return i, clock.GetTime() - startTime, nil
				}
			}
		}
	}
}

func showInstructions(exp *control.Experiment) error {
	text := "Welcome to the Geometry Experiment.\n\n" +
		"You will see sequences of dots flashing on an octagon.\n" +
		"Your task is to guess the NEXT location in the sequence\n" +
		"by clicking on it with the mouse.\n\n" +
		"If your guess is correct, you continue to the next one.\n" +
		"If you make a mistake, the sequence will restart from the\n" +
		"beginning to show you the correct locations.\n\n" +
		"Press any key to begin."

	return exp.ShowInstructions(text)
}

func main() {
	fmt.Println("Main started")
	exp := control.NewExperimentFromFlags("LoT-Geometry-Task", control.Black, control.White, 32)
	defer exp.End()

	fmt.Println("Experiment loaded. Starting Run...")

	exp.Run(func() error {
		fmt.Println("Inside Logic Thread: about to show instructions")
		// Show instructions before starting
		if err := showInstructions(exp); err != nil {
			fmt.Printf("showInstructions error: %v\n", err)
			return err
		}
		fmt.Println("Instructions dismissed!")

		exp.AddDataVariableNames([]string{"trial_idx", "seq_name", "step", "target_idx", "click_idx", "is_correct", "rt"})

		octagonPoints := getOctagonPoints(300)
		dots := make([]*stimuli.Circle, 8)
		for i := 0; i < 8; i++ {
			dots[i] = stimuli.NewCircle(15, control.Color{R: 80, G: 80, B: 80, A: 255})
			dots[i].SetPosition(octagonPoints[i])
		}
		target := stimuli.NewCircle(25, control.White)
		fixation := stimuli.NewFixCross(20, 3, control.White)

		// Define sequences (base 8 items, will be repeated to 16)
		// Indices 0-7 correspond to 1-8 in the study.
		allSequences := []Sequence{
			NewSequence("Repeat CW", []int{0, 1, 2, 3, 4, 5, 6, 7}),
			NewSequence("Repeat CCW", []int{0, 7, 6, 5, 4, 3, 2, 1}),
			NewSequence("Alternate CW", []int{0, 2, 1, 3, 2, 4, 3, 5}),  // +2, -1
			NewSequence("Alternate CCW", []int{0, 6, 7, 5, 6, 4, 5, 3}), // -2, +1
			NewSequence("2squares CW", []int{0, 2, 4, 6, 1, 3, 5, 7}),
			NewSequence("2squares CCW", []int{0, 6, 4, 2, 7, 5, 3, 1}),
			NewSequence("2arcs CW", []int{4, 5, 6, 7, 4, 3, 2, 1}),
			NewSequence("2arcs CCW", []int{0, 7, 6, 5, 0, 1, 2, 3}),
			NewSequence("4segments H", []int{1, 7, 2, 6, 3, 5, 0, 4}),
			NewSequence("4segments V", []int{1, 3, 0, 4, 7, 5, 2, 6}),
			NewSequence("4segments A", []int{0, 2, 7, 3, 6, 4, 5, 1}),
			NewSequence("4segments B", []int{0, 6, 1, 5, 2, 4, 7, 3}),
			NewSequence("4diagonals", []int{0, 4, 1, 5, 2, 6, 3, 7}),
			NewSequence("2rectangles", []int{1, 7, 5, 3, 0, 2, 4, 6}),
			NewSequence("2crosses", []int{0, 4, 2, 6, 1, 5, 3, 7}),
			NewSequence("Irregular 1", []int{0, 3, 5, 1, 7, 6, 2, 4}),
			NewSequence("Irregular 2", []int{0, 5, 2, 7, 4, 1, 6, 3}),
		}

		// Randomized order: first 2 are always Repeat CW and CCW (randomized between them)
		firstTwo := []Sequence{allSequences[0], allSequences[1]}
		design.ShuffleList(firstTwo)

		rest := allSequences[2:]
		design.ShuffleList(rest)

		orderedSequences := append(firstTwo, rest...)

		// Main Experiment Loop
		for trialIdx, seq := range orderedSequences {
			fmt.Printf("Starting trial %d: %s\n", trialIdx+1, seq.Name)

			// Starting point randomization (0-7)
			startOffset := design.RandInt(0, 7)
			indices := make([]int, 16)
			for i := 0; i < 16; i++ {
				indices[i] = (seq.Indices[i] + startOffset) % 8
			}

			currentKnownCount := 2
			needsFlash := true // flash before the very first guess and after every error
			for step := 2; step < 16; step++ {
				// A. Flash sequence only at trial start or after an error.
				// On a correct streak the subject continues directly to the next guess.
				if needsFlash {
					flashSequence(exp, dots, fixation, target, indices[:currentKnownCount])
				}

				// B. Wait for guess
				targetIdx := indices[step]
				clickIdx, rt, _ := getGuess(exp, dots, fixation, octagonPoints)

				isCorrect := (clickIdx == targetIdx)

				// C. Record data
				exp.Data.Add(
					trialIdx+1, seq.Name, step+1, targetIdx, clickIdx, isCorrect, rt,
				)

				// D. Feedback / advance logic
				if isCorrect {
					// Correct: brief feedback, then ask for the next location immediately
					// (no re-flash — subject continues in the streak).
					stimuli.PlayPing(exp.AudioDevice)
					drawEnvironment(exp, dots, fixation, target, targetIdx)
					exp.Wait(300)
					currentKnownCount++
					needsFlash = false
				} else {
					// Error: show the correct location, then re-flash the full sequence
					// (including the just-corrected item) before the next guess.
					stimuli.PlayBuzzer(exp.AudioDevice)
					drawEnvironment(exp, dots, fixation, target, targetIdx)
					exp.Wait(500)
					currentKnownCount = step + 1
					needsFlash = true
				}

				drawEnvironment(exp, dots, fixation, target, -1)
				exp.Wait(500)
			}

			// Inter-trial interval
			exp.Blank(1000)
		}
		return control.EndLoop
	})
}