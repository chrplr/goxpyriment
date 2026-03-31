// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"log"
	"math"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// getAsymmetricShape returns points for an asymmetric "L-like" shape.
func getAsymmetricShape() []control.FPoint {
	return []control.FPoint{
		{X: -40, Y: -60},
		{X: 40, Y: -60},
		{X: 40, Y: -20},
		{X: -10, Y: -20},
		{X: -10, Y: 60},
		{X: -40, Y: 60},
	}
}

// rotatePoints rotates a set of points by an angle in degrees.
func rotatePoints(points []control.FPoint, angle float64) []control.FPoint {
	rad := angle * math.Pi / 180.0
	res := make([]control.FPoint, len(points))
	cosA := float32(math.Cos(rad))
	sinA := float32(math.Sin(rad))
	for i, p := range points {
		res[i] = control.FPoint{
			X: p.X*cosA - p.Y*sinA,
			Y: p.X*sinA + p.Y*cosA,
		}
	}
	return res
}

// mirrorPoints mirrors a set of points across the Y axis.
func mirrorPoints(points []control.FPoint) []control.FPoint {
	res := make([]control.FPoint, len(points))
	for i, p := range points {
		res[i] = control.FPoint{X: -p.X, Y: p.Y}
	}
	return res
}

func showInstructions(exp *control.Experiment) error {
	text := "Mental Rotation Task\n\n" +
		"Two shapes will appear on the screen.\n" +
		"Determine if they are the SAME shape (just rotated)\n" +
		"or if they are MIRROR images of each other.\n\n" +
		"Press 'S' if they are the SAME.\n" +
		"Press 'D' if they are DIFFERENT (mirrored).\n\n" +
		"Try to be as fast and accurate as possible.\n\n" +
		"Press any key to begin."

	instrBox := stimuli.NewTextBox(text, 600, control.FPoint{X: 0, Y: 0}, control.White)
	if err := exp.Show(instrBox); err != nil {
		return err
	}
	_, err := exp.Keyboard.Wait()
	return err
}

func main() {
	exp := control.NewExperimentFromFlags("Mental-Rotation-2D", control.Black, control.White, 32)
	defer exp.End()

	exp.AddDataVariableNames([]string{"trial_idx", "angle", "condition", "response", "is_correct", "rt"})

	// Show instructions
	if err := showInstructions(exp); err != nil {
		if control.IsEndLoop(err) {
			return
		}
		log.Fatalf("instruction error: %v", err)
	}

	// 1. Create Design
	block := design.NewBlock("Main Block")
	angles := []int{0, 40, 80, 120, 160}
	conditions := []string{"same", "mirrored"}

	for _, angle := range angles {
		for _, cond := range conditions {
			trial := design.NewTrial()
			trial.SetFactor("angle", angle)
			trial.SetFactor("condition", cond)
			block.AddTrial(trial, 4, true) // 4 repetitions of each combination
		}
	}
	block.ShuffleTrials()

	// 2. Main Loop
	basePoints := getAsymmetricShape()
	fixation := stimuli.NewFixCross(20, 3, control.White)

	for i, trial := range block.Trials {
		angle := float64(trial.GetFactor("angle").(int))
		condition := trial.GetFactor("condition").(string)

		// Prepare stimuli
		leftShape := stimuli.NewShape(basePoints, control.White)
		leftShape.SetPosition(control.FPoint{X: -150, Y: 0})

		var rightPoints []control.FPoint
		if condition == "same" {
			rightPoints = rotatePoints(basePoints, angle)
		} else {
			rightPoints = rotatePoints(mirrorPoints(basePoints), angle)
		}
		rightShape := stimuli.NewShape(rightPoints, control.White)
		rightShape.SetPosition(control.FPoint{X: 150, Y: 0})

		// Fixation period
		if err := exp.Screen.Clear(); err != nil {
			log.Fatal(err)
		}
		if err := fixation.Draw(exp.Screen); err != nil {
			log.Fatal(err)
		}
		if err := exp.Screen.Update(); err != nil {
			log.Fatal(err)
		}
		clock.Wait(500)

		// Show stimulus
		if err := exp.Screen.Clear(); err != nil {
			log.Fatal(err)
		}
		if err := fixation.Draw(exp.Screen); err != nil {
			log.Fatal(err)
		}
		if err := leftShape.Draw(exp.Screen); err != nil {
			log.Fatal(err)
		}
		if err := rightShape.Draw(exp.Screen); err != nil {
			log.Fatal(err)
		}
		if err := exp.Screen.Update(); err != nil {
			log.Fatal(err)
		}

		startTime := clock.GetTime()

		// Collect response
		var key control.Keycode
		var err error
		for {
			key, err = exp.Keyboard.WaitKeys([]control.Keycode{control.K_S, control.K_D, control.K_ESCAPE}, -1)
			if err != nil {
				if control.IsEndLoop(err) {
					return
				}
				log.Fatalf("keyboard error: %v", err)
			}
			if key != 0 {
				break
			}
		}

		rt := clock.GetTime() - startTime

		response := ""
		isCorrect := false
		if key == control.K_S {
			response = "same"
			isCorrect = (condition == "same")
		} else if key == control.K_D {
			response = "mirrored"
			isCorrect = (condition == "mirrored")
		} else if key == control.K_ESCAPE {
			return
		}

		// Auditory feedback: only negative feedback
		if !isCorrect {
			stimuli.PlayBuzzer(exp.AudioDevice)
		}

		// Log data
		exp.Data.Add(
			i+1, angle, condition, response, isCorrect, rt,
		)

		// Blank screen (with fixation cross) between trials
		if err := exp.Screen.Clear(); err != nil {
			log.Fatal(err)
		}
		if err := fixation.Draw(exp.Screen); err != nil {
			log.Fatal(err)
		}
		exp.Screen.Update()
		clock.Wait(500)
	}
}
