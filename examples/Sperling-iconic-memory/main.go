// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"fmt"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"
	"strings"
)

const (
	StimulusDuration = 50   // ms
	CueDuration      = 200  // ms
	FixationDuration = 500  // ms
	GridSpacing      = 60   // pixels
)

// generateGrid creates a 3x3 array of random uppercase letters (excluding vowels for standard practice).
func generateGrid() [3][3]string {
	letters := "BCDFGHJKLMNPQRSTVWXYZ"
	grid := [3][3]string{}
	used := make(map[byte]bool)
	for r := 0; r < 3; r++ {
		for c := 0; r < 3 && c < 3; c++ {
			for {
				l := letters[design.RandInt(0, len(letters)-1)]
				if !used[l] {
					grid[r][c] = string(l)
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
			y := float32((1 - r) * GridSpacing) // Row 0 is Top
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
		"A 3x3 grid of letters will flash very briefly.\n\n" +
		"PARTIAL REPORT:\n" +
		"After the flash, you will hear a TONE:\n" +
		" - HIGH tone: Recall TOP row\n" +
		" - MEDIUM tone: Recall MIDDLE row\n" +
		" - LOW tone: Recall BOTTOM row\n\n" +
		"WHOLE REPORT:\n" +
		"Recall as many letters as you can.\n\n" +
		"Press any key to begin."

	instrBox := stimuli.NewTextBox(text, 650, control.FPoint{X: 0, Y: 0}, control.White)
	if err := instrBox.Present(exp.Screen, true, true); err != nil {
		return err
	}
	_, err := exp.Keyboard.Wait()
	return err
}

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 800x600)")
	subjectID := flag.Int("s", 1, "Subject ID")
	flag.Parse()

	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 800, 600, false
	}

	exp := control.NewExperiment("Sperling-Partial-Report", width, height, fullscreen, control.Black, control.White, 32)
	exp.SubjectID = *subjectID

	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	exp.Data.AddVariableNames([]string{"trial_idx", "condition", "cued_row", "target_letters", "response", "accuracy"})

	if err := showInstructions(exp); err != nil {
		if err == control.EndLoop { return }
		log.Fatalf("instruction error: %v", err)
	}

	// Tones
	highTone := stimuli.NewTone(1000, CueDuration, 0.5)
	medTone := stimuli.NewTone(500, CueDuration, 0.5)
	lowTone := stimuli.NewTone(250, CueDuration, 0.5)
	
	highTone.PreloadDevice(exp.AudioDevice)
	medTone.PreloadDevice(exp.AudioDevice)
	lowTone.PreloadDevice(exp.AudioDevice)

	// Trial configurations
	type TrialConfig struct {
		Condition string // "partial" or "whole"
		CuedRow   int    // 0, 1, 2
	}

	var trials []TrialConfig
	// 10 Whole report trials
	for i := 0; i < 10; i++ {
		trials = append(trials, TrialConfig{Condition: "whole", CuedRow: -1})
	}
	// 30 Partial report trials (10 per row)
	for row := 0; row < 3; row++ {
		for i := 0; i < 10; i++ {
			trials = append(trials, TrialConfig{Condition: "partial", CuedRow: row})
		}
	}
	design.ShuffleList(trials)

	// 8 training trials (mix of whole and partial report, not logged).
	var trainingTrials []TrialConfig
	for i := 0; i < 4; i++ {
		trainingTrials = append(trainingTrials, TrialConfig{Condition: "whole", CuedRow: -1})
	}
	for i := 0; i < 4; i++ {
		row := design.RandInt(0, 2)
		trainingTrials = append(trainingTrials, TrialConfig{Condition: "partial", CuedRow: row})
	}
	design.ShuffleList(trainingTrials)

	fixation := stimuli.NewFixCross(20, 2, control.White)

	// Helper to run a single trial and optionally provide error feedback.
	runOne := func(trialIdx int, config TrialConfig, giveFeedback bool, logData bool) error {
		grid := generateGrid()
		
		// 1. Fixation
		if err := fixation.Present(exp.Screen, true, true); err != nil { return err }
		clock.Wait(FixationDuration)

		// 2. Stimulus flash (50ms)
		if err := exp.Screen.Clear(); err != nil { return err }
		if err := drawGrid(exp, grid); err != nil { return err }
		if err := exp.Screen.Update(); err != nil { return err }
		clock.Wait(StimulusDuration)

		// 3. Offset (ISI) - can be varied, here 0ms
		if err := exp.Screen.Clear(); err != nil { return err }
		if err := exp.Screen.Update(); err != nil { return err }
		// clock.Wait(offset)

		// 4. Cue
		var targetLetters string
		if config.Condition == "partial" {
			switch config.CuedRow {
			case 0: highTone.Play(); targetLetters = strings.Join(grid[0][:], "")
			case 1: medTone.Play(); targetLetters = strings.Join(grid[1][:], "")
			case 2: lowTone.Play(); targetLetters = strings.Join(grid[2][:], "")
			}
		} else {
			// Whole report: no specific tone cue, or a neutral one
			targetLetters = strings.Join(grid[0][:], "") + strings.Join(grid[1][:], "") + strings.Join(grid[2][:], "")
		}

		// 5. Response
		prompt := "Enter the letters you remember:"
		if config.Condition == "partial" {
			rowNames := []string{"TOP", "MIDDLE", "BOTTOM"}
			prompt = fmt.Sprintf("Recall the %s row:", rowNames[config.CuedRow])
		}
		
		ti := stimuli.NewTextInput(prompt, control.FPoint{X: 0, Y: 0}, 300, control.Black, control.White, control.White)
		response, err := ti.Get(exp.Screen, exp.Keyboard)
		if err != nil {
			return err
		}
		
		response = strings.ToUpper(strings.TrimSpace(response))
		
		// Calculate accuracy (count how many target letters are in response)
		acc := 0
		for _, char := range targetLetters {
			if strings.Contains(response, string(char)) {
				acc++
			}
		}

		if giveFeedback {
			// Treat a trial as correct only if the full target row (or whole grid)
			// is exactly reproduced (order-sensitive). Otherwise, play an error buzzer.
			if response != strings.ToUpper(targetLetters) {
				_ = stimuli.PlayBuzzer(exp.AudioDevice)
			}
		}

		if logData {
			exp.Data.Add([]interface{}{
				trialIdx, config.Condition, config.CuedRow, targetLetters, response, acc,
			})
		}

		// ITI
		if err := exp.Screen.Clear(); err != nil { return err }
		exp.Screen.Update()
		clock.Wait(1000)

		return nil
	}

	// Training block (8 trials, feedback with buzzer on incorrect, no logging).
	for i, config := range trainingTrials {
		if err := runOne(i+1, config, true, false); err != nil {
			if err == control.EndLoop {
				return
			}
			log.Fatalf("training trial error: %v", err)
		}
	}

	// Training finished screen.
	trainDone := stimuli.NewTextBox(
		"Training finished.\n\nPress a key to go on to the main experiment.",
		650,
		control.FPoint{X: 0, Y: 0},
		control.White,
	)
	if err := trainDone.Present(exp.Screen, true, true); err != nil {
		log.Fatalf("training-finished screen error: %v", err)
	}
	if _, err := exp.Keyboard.Wait(); err != nil && err != control.EndLoop {
		log.Fatalf("training-finished wait error: %v", err)
	}

	// Main experimental trials (logged, no additional buzzer feedback).
	for i, config := range trials {
		if err := runOne(i+1, config, false, true); err != nil {
			if err == control.EndLoop {
				return
			}
			log.Fatalf("trial error: %v", err)
		}
	}
}
