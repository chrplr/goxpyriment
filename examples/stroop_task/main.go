// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/misc"
	"github.com/chrplr/goxpyriment/stimuli"

	"github.com/Zyko0/go-sdl3/sdl"
)

type stroopTrial struct {
	word  string
	color sdl.Color
	name  string
}

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	flag.Parse()

	// 1. Create and initialize the experiment
	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}
	exp := control.NewExperiment("Stroop Task", width, height, fullscreen)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
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
	colors := []sdl.Color{control.Red, control.Green, control.Blue, control.Yellow}
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
		instructions := stimuli.NewTextBox(instrText, 800, sdl.FPoint{X: 0, Y: 0}, control.DefaultTextColor)
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
			if key == sdl.K_SPACE {
				break
			}
			misc.Wait(10)
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

			// Stimulus
			stim := stimuli.NewTextLine(t.word, 0, 0, t.color)
			if err := stim.Present(exp.Screen, true, true); err != nil {
				return err
			}

			// Wait for response
			startTime := misc.GetTime()
			for {
				key, _, subErr = exp.HandleEvents()
				if subErr != nil {
					return subErr
				}

				var resp string
				switch key {
				case sdl.K_R:
					resp = "RED"
				case sdl.K_G:
					resp = "GREEN"
				case sdl.K_B:
					resp = "BLUE"
				case sdl.K_Y:
					resp = "YELLOW"
				}

				if resp != "" {
					rt := misc.GetTime() - startTime
					correct := resp == t.name
					congruent := t.word == t.name

					// Log to data file
					exp.Data.Add([]interface{}{
						i,
						t.word,
						t.name,
						resp,
						rt,
						correct,
						congruent,
					})

					fmt.Printf("Trial %d: Word=%s, Color=%s, Resp=%s, RT=%d ms, Correct=%v, Congruent=%v\n", i, t.word, t.name, resp, rt, correct, congruent)
					break
				}
				misc.Wait(1)
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
