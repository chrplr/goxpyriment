// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"flag"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
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
        
	exp := control.NewExperiment("My First Go Experiment", 1368, 1024, *fullscreen)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	if err := exp.LoadFontFromMemory(inconsolataFont, 32); err != nil {
		log.Printf("Warning: failed to load font: %v. Using fallback.", err)
	}

        greetings := stimuli.NewTextBox("Hello World !", 600, sdl.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
	instr := stimuli.NewTextBox("Press any key to start the experiment", 600, sdl.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
	finish := stimuli.NewTextBox("Experiment Finished!\n Press any key to exit.", 600, sdl.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
        
	sound := stimuli.NewSoundFromMemory(bonjourWav)
	if err := sound.PreloadDevice(exp.AudioDevice); err != nil {
		log.Printf("Warning: failed to load sound: %v", err)
	}

	// Run the experiment logic
	err := exp.Run(func() error {
		if err := instr.Present(exp.Screen, true, true); err != nil {
			return err
		}
		if _, err := exp.Keyboard.Wait(); err != nil {
			return err
		}

		if err := sound.Play(); err != nil {
			return err
		}

                greetings.Present(exp.Screen, true, true)
                exp.Keyboard.Wait()
                
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
