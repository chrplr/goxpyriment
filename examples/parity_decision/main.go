package main

import (
	_ "embed"
	"fmt"
	"goxpyriment/control"
	"goxpyriment/misc"
	"goxpyriment/stimuli"
	"log"
	"math/rand"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

//go:embed assets/Roboto-Regular.ttf
var robotoFont []byte

const (
	NTrialsPerTarget = 1
	EvenResponse     = sdl.K_F
	OddResponse      = sdl.K_J
	MaxResponseDelay = 2000
)

var Targets = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// 1. Create and initialize the experiment
	exp := control.NewExperiment("Parity Decision", 800, 600, false)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	if err := exp.LoadFontFromMemory(robotoFont, 32); err != nil {
		log.Printf("Warning: failed to load font: %v. Using fallback.", err)
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
			trials = append(trials, trialData{number: num, stim: stim})
		}
	}
	// Shuffle trials
	rand.Shuffle(len(trials), func(i, j int) {
		trials[i], trials[j] = trials[j], trials[i]
	})

	cue := stimuli.NewFixCross(50, 4, control.DefaultTextColor)

	instrText := fmt.Sprintf("When you'll see a number, your task to decide, as quickly as possible, whether it is even or odd.\n\nif it is even, press 'F'\n\nif it is odd, press 'J'\n\nThere will be %d trials in total.\n\nPress the spacebar to start.", len(trials))
	instructions := stimuli.NewTextBox(instrText, 600, sdl.FPoint{X: 0, Y: 100}, control.DefaultTextColor)

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		// Instructions
		if err := instructions.Present(exp.Screen, true, true); err != nil {
			return err
		}
		for {
			key, err := exp.Keyboard.Wait()
			if err != nil {
				return err
			}
			if key == sdl.K_SPACE {
				break
			}
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
			misc.Wait(1000)

			// Cue
			if err := cue.Present(exp.Screen, true, true); err != nil {
				return err
			}
			misc.Wait(500)

			// Stimulus
			if err := t.stim.Present(exp.Screen, true, true); err != nil {
				return err
			}

			// Wait for response
			startTime := misc.GetTime()
			for {
				key, err := exp.Keyboard.Wait()
				if err != nil {
					return err
				}
				if key == EvenResponse || key == OddResponse {
					rt := misc.GetTime() - startTime
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
			}
			
			// Small pause between trials
			misc.Wait(500)
		}

		return sdl.EndLoop // Graceful exit
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
