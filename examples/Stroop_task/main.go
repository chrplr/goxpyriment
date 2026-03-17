// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

type stroopTrial struct {
	word  string
	color control.Color
	name  string
}

func main() {
	exp := control.NewExperimentFromFlags("Stroop Task", control.Black, control.White, 32)
	defer exp.End()

	// Prepare event log header and write it as comments in the data file.
	// We will log word, ink color, response, RT, correctness and congruency.
	evLog := exp.CollectEventLog()
	evLog.SetSubjectID(fmt.Sprintf("%d", exp.SubjectID))
	evLog.SetCSVHeader([]string{"trial", "word", "ink_color", "response", "rt", "correct", "congruent"})
	exp.Data.WriteComment("--EVENT LOG")
	exp.Data.WriteComment(evLog.String())
	exp.Data.WriteComment("--TRIAL DATA")
	exp.Data.AddVariableNames([]string{"trial", "word", "ink_color", "response", "rt", "correct", "congruent"})

	// Set logical size for consistent centering
	//if err := exp.SetLogicalSize(int32(winW), int32(winH)); err != nil {
	//	log.Printf("Warning: failed to set logical size: %v", err)
	//}

	// Wait for fullscreen transition to stabilize
	// if isFullscreen {
	//	misc.Wait(2000)
	// }

	// 2. Prepare design and stimuli
	words := []string{"RED", "GREEN", "BLUE", "YELLOW"}
	colors := []control.Color{control.Red, control.Green, control.Blue, control.Yellow}
	colorNames := []string{"RED", "GREEN", "BLUE", "YELLOW"}

	var trials []stroopTrial
	for _, word := range words {
		for j, color := range colors {
			trials = append(trials, stroopTrial{word: word, color: color, name: colorNames[j]})
		}
	}
	// Shuffle trials
	rand.Shuffle(len(trials), func(i, j int) {
		trials[i], trials[j] = trials[j], trials[i]
	})

	instrText := "Name the COLOR of the word as quickly as possible!\n\nUse keys R, G, B, Y for Red, Green, Blue, Yellow.\n\nPress SPACE to start."

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		// Instructions
		instructions := stimuli.NewTextBox(instrText, 800, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)
		if err := exp.Show(instructions); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}

		// Loop through trials
		for i, t := range trials {
			// Blank screen
			if err := exp.Blank(1000); err != nil {
				return err
			}

			// Stimulus
			stim := stimuli.NewTextLine(t.word, 0, 0, t.color)
			if err := exp.Show(stim); err != nil {
				return err
			}

			// Wait for response
			startTime := clock.GetTime()
			var key control.Keycode
			var subErr error
			for {
				key, _, subErr = exp.HandleEvents()
				if subErr != nil {
					return subErr
				}

				var resp string
				switch key {
				case control.K_R:
					resp = "RED"
				case control.K_G:
					resp = "GREEN"
				case control.K_B:
					resp = "BLUE"
				case control.K_Y:
					resp = "YELLOW"
				}

				if resp != "" {
					rt := clock.GetTime() - startTime
					correct := resp == t.name
					congruent := t.word == t.name

					// Log to data file
					exp.Data.Add(
						i,
						t.word,
						t.name,
						resp,
						rt,
						correct,
						congruent,
					)

					fmt.Printf("Trial %d: Word=%s, Color=%s, Resp=%s, RT=%d ms, Correct=%v, Congruent=%v\n", i, t.word, t.name, resp, rt, correct, congruent)
					break
				}
				clock.Wait(1)
			}

			// Small pause between trials
			clock.Wait(500)
		}

		return control.EndLoop // Graceful exit
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
