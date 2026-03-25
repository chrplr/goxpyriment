// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"fmt"
	"log"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	NTrialsPerTarget = 1
	EvenResponse     = control.K_F
	OddResponse      = control.K_J
)

var Targets = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

func main() {
	exp := control.NewExperimentFromFlags("Parity Decision", control.Black, control.White, 32)
	defer exp.End()

	// Set logical size for consistent centering
	if err := exp.SetLogicalSize(1368, 1024); err != nil {
		log.Printf("Warning: failed to set logical size: %v", err)
	}

	// Create a larger font specifically for the numbers (64pt)
	bigFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 64)
	if err != nil {
		log.Printf("Warning: failed to load big font: %v", err)
	} else {
		defer bigFont.Close()
	}

	exp.AddDataVariableNames([]string{"number", "key", "rt", "correct"})

	// 2. Prepare design and stimuli
	type trialData struct {
		number int
		stim   *stimuli.TextLine
	}
	var trials []trialData
	for i := 0; i < NTrialsPerTarget; i++ {
		for _, num := range Targets {
			stim := stimuli.NewTextLine(fmt.Sprintf("%d", num), 0, 0, control.DefaultTextColor)
			// Apply the larger font to the stimulus number
			if bigFont != nil {
				stim.Font = bigFont
			}
			trials = append(trials, trialData{number: num, stim: stim})
		}
	}
	// Shuffle trials
	design.ShuffleList(trials)

	cue := stimuli.NewFixCross(50, 4, control.DefaultTextColor)

	instrText := fmt.Sprintf("When you'll see a number, your task to decide, as quickly as possible, whether it is even or odd.\n\nif it is even, press 'F'\n\nif it is odd, press 'J'\n\nThere will be %d trials in total.\n\nPress the spacebar to start.", len(trials))

	// 3. Run the experiment logic
	err = exp.Run(func() error {
		exp.ShowInstructions(instrText)

		for i, t := range trials {
			exp.Blank(1000)
			exp.Show(cue)
			exp.Wait(500)
			onsetNS, _ := exp.ShowNS(t.stim)

			key, eventTS, _ := exp.Keyboard.WaitKeysEventRT([]control.Keycode{EvenResponse, OddResponse}, -1)
			rt := int64(eventTS-onsetNS) / 1_000_000
			correct := (t.number%2 == 0) == (key == EvenResponse)
			exp.Data.Add(t.number, key, rt, correct)
			fmt.Printf("Trial %d: Num=%d, RT=%d ms, Correct=%v\n", i, t.number, rt, correct)
			if !correct {
				exp.Audio.PlayBuzzer()
			}
			exp.Wait(500)
		}

		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
