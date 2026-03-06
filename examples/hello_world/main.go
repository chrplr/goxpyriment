// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"flag"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"

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
	exp.Run(func() error {
		instr.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()
		sound.Play()

		greetings.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()

		finish.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()

		return sdl.EndLoop
	})

}
