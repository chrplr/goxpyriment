package main

import (
	_ "embed"
	"flag"
	"fmt"
	"goxpyriment/control"
	"goxpyriment/design"
	"goxpyriment/misc"
	"goxpyriment/stimuli"
	"log"

	"github.com/Zyko0/go-sdl3/sdl"
)

//go:embed assets/Inconsolata.ttf
var inconsolataFont []byte

//go:embed assets/bonjour.wav
var bonjourWav []byte

func main() {
	fullscreen := flag.Bool("F", false, "Launch in fullscreen display mode")
	flag.Parse()
	// 1. Create and initialize the experiment
	exp := control.NewExperiment("My First Go Experiment", 1368, 1024, *fullscreen)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	if err := exp.LoadFontFromMemory(inconsolataFont, 32); err != nil {
		log.Printf("Warning: failed to load font: %v. Using fallback.", err)
	}

	// 2. Prepare design
	block := design.NewBlock("Main Block")
	for i := 0; i < 5; i++ {
		trial := design.NewTrial()
		trial.Factors["color"] = "white"
		block.AddTrial(trial, 1, false)
	}

	// 3. Prepare stimuli
	instr := stimuli.NewTextBox("Press any key to start the experiment", 600, sdl.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
	fixation := stimuli.NewTextLine("+", 0, 0, control.DefaultTextColor)
	rect := stimuli.NewRectangle(0, 0, 100, 100, sdl.Color{R: 255, G: 0, B: 0, A: 255})
	finish := stimuli.NewTextBox("Experiment Finished! Press any key to exit.", 600, sdl.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
	sound := stimuli.NewSoundFromMemory(bonjourWav)

	if err := sound.PreloadDevice(exp.AudioDevice); err != nil {
		log.Printf("Warning: failed to load sound: %v", err)
	}

	// 4. Run the experiment logic
	err := exp.Run(func() error {
		// Instructions
		if err := instr.Present(exp.Screen, true, true); err != nil {
			return err
		}
		if _, err := exp.Keyboard.Wait(); err != nil {
			return err
		}

		// Play sound at start
		if err := sound.Play(); err != nil {
			return err
		}


		// Loop through trials
		for _, trial := range block.Trials {
			fmt.Printf("Running trial %d\n", trial.ID)

			// Fixation cross
			if err := fixation.Present(exp.Screen, true, true); err != nil {
				return err
			}
			misc.Wait(500)

			// Target stimulus
			if err := rect.Present(exp.Screen, true, true); err != nil {
				return err
			}

			// Wait for response
			startTime := misc.GetTime()
			_, err := exp.Keyboard.Wait()
			if err != nil {
				return err
			}
			rt := misc.GetTime() - startTime
			fmt.Printf("Reaction Time: %d ms\n", rt)
			
			// Clear screen between trials
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := exp.Screen.Update(); err != nil {
				return err
			}
			misc.Wait(500)
		}

		// Finish
		if err := finish.Present(exp.Screen, true, true); err != nil {
			return err
		}
		if _, err := exp.Keyboard.Wait(); err != nil {
			return err
		}

		return sdl.EndLoop // Graceful exit
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
