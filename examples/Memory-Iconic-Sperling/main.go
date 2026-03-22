// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"log"
	"strings"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	StimulusDuration = 50  // ms
	CueDuration      = 200 // ms
	FixationDuration = 500 // ms
	GridSpacing      = 60  // pixels
)

// letterPool contains the consonants used to build the stimulus grids.
// Vowels are excluded following the standard Sperling (1960) practice.
var letterPool = strings.Split("BCDFGHJKLMNPQRSTVWXYZ", "")

// generateGrid creates a 3×3 array of unique consonants drawn from letterPool.
func generateGrid() [3][3]string {
	grid := [3][3]string{}
	used := make(map[string]bool)
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			for {
				l := letterPool[design.RandInt(0, len(letterPool)-1)]
				if !used[l] {
					grid[r][c] = l
					used[l] = true
					break
				}
			}
		}
	}
	return grid
}

func drawGrid(exp *control.Experiment, grid [3][3]string) error {
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			x := float32((c - 1) * GridSpacing)
			y := float32((1 - r) * GridSpacing) // row 0 → top
			txt := stimuli.NewTextLine(grid[r][c], x, y, control.White)
			if err := txt.Draw(exp.Screen); err != nil {
				return err
			}
		}
	}
	return nil
}

func showInstructions(exp *control.Experiment) error {
	text := "Sperling's Iconic Memory Experiment\n\n" +
		"A 3×3 grid of letters will flash very briefly.\n\n" +
		"PARTIAL REPORT:\n" +
		"After the flash, you will hear a TONE:\n" +
		" - HIGH tone: Recall TOP row\n" +
		" - MEDIUM tone: Recall MIDDLE row\n" +
		" - LOW tone: Recall BOTTOM row\n\n" +
		"Click the letters you remember (or press their keys).\n" +
		"BACKSPACE to undo the last choice.\n\n" +
		"WHOLE REPORT:\n" +
		"Recall as many letters as you can, then press ENTER.\n\n" +
		"Press any key to begin."

	instrBox := stimuli.NewTextBox(text, 650, control.FPoint{X: 0, Y: 0}, control.White)
	if err := exp.Show(instrBox); err != nil {
		return err
	}
	_, err := exp.Keyboard.Wait()
	return err
}

func main() {
	exp := control.NewExperimentFromFlags("Sperling-Partial-Report", control.Black, control.White, 28)
	defer exp.End()

	exp.AddDataVariableNames([]string{"trial_idx", "condition", "cued_row", "target_letters", "response", "accuracy"})

	if err := showInstructions(exp); err != nil {
		if control.IsEndLoop(err) {
			return
		}
		log.Fatalf("instruction error: %v", err)
	}

	// Tones
	highTone := stimuli.NewTone(1000, CueDuration, 0.5)
	medTone := stimuli.NewTone(500, CueDuration, 0.5)
	lowTone := stimuli.NewTone(250, CueDuration, 0.5)

	highTone.PreloadDevice(exp.AudioDevice)
	medTone.PreloadDevice(exp.AudioDevice)
	lowTone.PreloadDevice(exp.AudioDevice)

	// Trial configurations.
	type TrialConfig struct {
		Condition string // "partial" or "whole"
		CuedRow   int    // 0=top, 1=middle, 2=bottom; -1 for whole
	}

	var trials []TrialConfig
	for i := 0; i < 10; i++ {
		trials = append(trials, TrialConfig{Condition: "whole", CuedRow: -1})
	}
	for row := 0; row < 3; row++ {
		for i := 0; i < 10; i++ {
			trials = append(trials, TrialConfig{Condition: "partial", CuedRow: row})
		}
	}
	design.ShuffleList(trials)

	// 8 training trials (4 whole + 4 partial), not logged.
	var trainingTrials []TrialConfig
	for i := 0; i < 4; i++ {
		trainingTrials = append(trainingTrials, TrialConfig{Condition: "whole", CuedRow: -1})
	}
	for i := 0; i < 4; i++ {
		trainingTrials = append(trainingTrials, TrialConfig{Condition: "partial", CuedRow: design.RandInt(0, 2)})
	}
	design.ShuffleList(trainingTrials)

	fixation := stimuli.NewFixCross(20, 2, control.White)

	// runOne executes a single trial.
	// giveFeedback: play a buzzer on incorrect responses (training only).
	// logData: write a row to the data file (main block only).
	runOne := func(trialIdx int, config TrialConfig, giveFeedback bool, logData bool) error {
		grid := generateGrid()

		// 1. Fixation
		if err := exp.Show(fixation); err != nil {
			return err
		}
		clock.Wait(FixationDuration)

		// 2. Stimulus flash (50 ms)
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := drawGrid(exp, grid); err != nil {
			return err
		}
		if err := exp.Screen.Update(); err != nil {
			return err
		}
		clock.Wait(StimulusDuration)

		// 3. Blank offset (ISI = 0 ms here; extend as needed)
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := exp.Screen.Update(); err != nil {
			return err
		}

		// 4. Cue tone + build target string
		var targetLetters string
		rowNames := []string{"TOP", "MIDDLE", "BOTTOM"}
		var prompt string

		if config.Condition == "partial" {
			switch config.CuedRow {
			case 0:
				highTone.Play()
				targetLetters = strings.Join(grid[0][:], "")
			case 1:
				medTone.Play()
				targetLetters = strings.Join(grid[1][:], "")
			case 2:
				lowTone.Play()
				targetLetters = strings.Join(grid[2][:], "")
			}
			prompt = "Recall the " + rowNames[config.CuedRow] + " row  (3 letters):"
		} else {
			targetLetters = strings.Join(grid[0][:], "") +
				strings.Join(grid[1][:], "") +
				strings.Join(grid[2][:], "")
			prompt = "Recall all letters you saw  (press ENTER when done):"
		}

		// 5. Response via ChoiceGrid
		var maxSel int
		if config.Condition == "partial" {
			maxSel = 3 // auto-submit after 3 selections
		}
		// maxSel == 0 for whole report → explicit ENTER/SPACE to submit

		cg := stimuli.NewChoiceGrid(letterPool, maxSel, prompt)
		cg.Cols = 7 // 21 consonants × 7 columns = 3 rows
		selections, err := cg.Get(exp.Screen, exp.Keyboard)
		if err != nil {
			return err
		}
		response := strings.Join(selections, "")

		// 6. Accuracy: count target letters present in the response
		acc := 0
		for _, ch := range targetLetters {
			if strings.Contains(response, string(ch)) {
				acc++
			}
		}

		if giveFeedback && response != strings.ToUpper(targetLetters) {
			_ = stimuli.PlayBuzzer(exp.AudioDevice)
		}

		if logData {
			exp.Data.Add(
				trialIdx, config.Condition, config.CuedRow, targetLetters, response, acc,
			)
		}

		// ITI
		if err := exp.Blank(1000); err != nil {
			return err
		}

		return nil
	}

	// Training block (feedback, no logging).
	for i, config := range trainingTrials {
		if err := runOne(i+1, config, true, false); err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatalf("training trial error: %v", err)
		}
	}

	trainDone := stimuli.NewTextBox(
		"Training finished.\n\nPress a key to start the main experiment.",
		650, control.FPoint{}, control.White,
	)
	if err := exp.Show(trainDone); err != nil {
		log.Fatalf("training-finished screen error: %v", err)
	}
	if _, err := exp.Keyboard.Wait(); err != nil && !control.IsEndLoop(err) {
		log.Fatalf("training-finished wait error: %v", err)
	}

	// Main block (logged, no buzzer feedback).
	for i, config := range trials {
		if err := runOne(i+1, config, false, true); err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatalf("trial error: %v", err)
		}
	}
}
