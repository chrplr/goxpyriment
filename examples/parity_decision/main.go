// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"flag"
	"fmt"
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"
	"math/rand"
	"time"

	"github.com/Zyko0/go-sdl3/ttf"
)

const (
	NTrialsPerTarget = 1
	EvenResponse     = control.K_F
	OddResponse      = control.K_J
)

var Targets = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	// 1. Create and initialize the experiment
	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}
	exp := control.NewExperiment("Parity Decision", width, height, fullscreen, control.Black, control.White, 32)
	exp.SubjectID = *subject
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// Set logical size for consistent centering
	if err := exp.SetLogicalSize(1368, 1024); err != nil {
		log.Printf("Warning: failed to set logical size: %v", err)
	}

	// Create a larger font specifically for the numbers (64pt)
	fontIO, _ := sdl.IOFromBytes(assets_embed.InconsolataFont)
	bigFont, err := ttf.OpenFontIO(fontIO, true, 64)
	if err != nil {
		log.Printf("Warning: failed to load big font: %v", err)
	} else {
		defer bigFont.Close()
	}

	exp.Data.AddVariableNames([]string{"number", "key", "rt", "correct"})

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
	rand.Shuffle(len(trials), func(i, j int) {
		trials[i], trials[j] = trials[j], trials[i]
	})

	cue := stimuli.NewFixCross(50, 4, control.DefaultTextColor)

	instrText := fmt.Sprintf("When you'll see a number, your task to decide, as quickly as possible, whether it is even or odd.\n\nif it is even, press 'F'\n\nif it is odd, press 'J'\n\nThere will be %d trials in total.\n\nPress the spacebar to start.", len(trials))
	// Use 1000px width for instructions to ensure they fit well
	instructions := stimuli.NewTextBox(instrText, 1000, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)

	// 3. Run the experiment logic
	err = exp.Run(func() error {
		// Instructions
		if err := instructions.Present(exp.Screen, true, true); err != nil {
			return err
		}
		var key sdl.Keycode
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

		// Loop through trials
		for i, t := range trials {
			// Blank screen
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := exp.Screen.Update(); err != nil {
				return err
			}
			clock.Wait(1000)

			// Cue
			if err := cue.Present(exp.Screen, true, true); err != nil {
				return err
			}
			clock.Wait(500)

			// Stimulus
			if err := t.stim.Present(exp.Screen, true, true); err != nil {
				return err
			}

			// Wait for response
			startTime := clock.GetTime()
			for {
				key, _, subErr = exp.HandleEvents()
				if subErr != nil {
					return subErr
				}
				if key == EvenResponse || key == OddResponse {
					rt := clock.GetTime() - startTime
					oddity := t.number % 2
					var responseOddity int
					if key == OddResponse {
						responseOddity = 1
					} else {
						responseOddity = 0
					}
					correct := oddity == responseOddity
					exp.Data.Add([]interface{}{t.number, key, rt, correct})
					fmt.Printf("Trial %d: Num=%d, Key=%d, RT=%d ms, Correct=%v\n", i, t.number, key, rt, correct)
					break
				}
				clock.Wait(1)
			}
			
			// Small pause between trials
			clock.Wait(500)
		}

		return control.EndLoop // Graceful exit
	})

	if err != nil && err != control.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
