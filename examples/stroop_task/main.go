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

var (
	Red    = sdl.Color{R: 255, G: 0, B: 0, A: 255}
	Green  = sdl.Color{R: 0, G: 255, B: 0, A: 255}
	Blue   = sdl.Color{R: 0, G: 0, B: 255, A: 255}
	Yellow = sdl.Color{R: 255, G: 255, B: 0, A: 255}
)

type stroopTrial struct {
	word  string
	color sdl.Color
	name  string
}

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// 1. Create and initialize the experiment
	exp := control.NewExperiment("Stroop Task", 800, 600, false)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	if err := exp.LoadFontFromMemory(robotoFont, 32); err != nil {
		log.Printf("Warning: failed to load font: %v. Using fallback.", err)
	}

	// 2. Prepare design and stimuli
	words := []string{"RED", "GREEN", "BLUE", "YELLOW"}
	colors := []sdl.Color{Red, Green, Blue, Yellow}
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

			// Stimulus
			stim := stimuli.NewTextLine(t.word, 0, 0, t.color)
			if err := stim.Present(exp.Screen, true, true); err != nil {
				return err
			}

			// Wait for response
			startTime := misc.GetTime()
			for {
				key, err := exp.Keyboard.Wait()
				if err != nil {
					return err
				}
				
				var resp string
				switch key {
				case sdl.K_R: resp = "RED"
				case sdl.K_G: resp = "GREEN"
				case sdl.K_B: resp = "BLUE"
				case sdl.K_Y: resp = "YELLOW"
				}

				if resp != "" {
					rt := misc.GetTime() - startTime
					correct := resp == t.name
					congruent := t.word == t.name
					fmt.Printf("Trial %d: Word=%s, Color=%s, Resp=%s, RT=%d ms, Correct=%v, Congruent=%v\n", i, t.word, t.name, resp, rt, correct, congruent)
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
