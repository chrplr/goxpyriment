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
	exp.AddDataVariableNames([]string{"trial", "word", "ink_color", "response", "rt", "correct", "congruent"})

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
	design.ShuffleList(trials)

	instrText := "Name the COLOR of the word as quickly as possible!\n\nUse keys R, G, B, Y for Red, Green, Blue, Yellow.\n\nPress SPACE to start."

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		// Instructions
		exp.ShowInstructions(instrText)

		// Loop through trials
		for i, t := range trials {
			// Blank screen
			exp.Blank(1000)

			// Stimulus
			stim := stimuli.NewTextLine(t.word, 0, 0, t.color)
			onsetNS, _ := exp.ShowNS(stim)

			// Wait for response
			responseKeys := []control.Keycode{control.K_R, control.K_G, control.K_B, control.K_Y}
			key, eventTS, _ := exp.Keyboard.WaitKeysEventRT(responseKeys, -1)
			rt := int64(eventTS-onsetNS) / 1_000_000

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
			correct := resp == t.name
			congruent := t.word == t.name
			exp.Data.Add(i, t.word, t.name, resp, rt, correct, congruent)
			fmt.Printf("Trial %d: Word=%s, Color=%s, Resp=%s, RT=%d ms, Correct=%v, Congruent=%v\n", i, t.word, t.name, resp, rt, correct, congruent)

			// Small pause between trials
			exp.Wait(500)
		}

		return control.EndLoop // Graceful exit
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
