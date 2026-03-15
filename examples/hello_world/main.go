// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"flag"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

//go:embed assets/bonjour.wav
var bonjourWav []byte

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}
	exp := control.NewExperiment("My First Go Experiment", width, height, fullscreen, control.Black, control.White, 32)
	exp.SubjectID = *subject
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()


	greetings := stimuli.NewTextBox("Hello World !", 600, control.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
	instr := stimuli.NewTextBox("Press any key to start the experiment", 600, control.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
	finish := stimuli.NewTextBox("Experiment Finished!\n Press any key to exit.", 600, control.FPoint{X: 0, Y: 100}, control.DefaultTextColor)

	sound := stimuli.NewSoundFromMemory(bonjourWav)
	if err := sound.PreloadDevice(exp.AudioDevice); err != nil {
		log.Printf("Warning: failed to load sound: %v", err)
	}

	// Run the experiment logic
	exp.Run(func() error {
		if err := stimuli.PlayPing(exp.AudioDevice); err != nil {
			log.Printf("Warning: failed to play ping: %v", err)
		}
		instr.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()
		sound.Play()

		greetings.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()

		finish.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()

		return control.EndLoop
	})

}
