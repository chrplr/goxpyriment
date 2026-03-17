// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"log"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

//go:embed assets/bonjour.wav
var bonjourWav []byte

func main() {
	exp := control.NewExperimentFromFlags("My First Go Experiment", control.Black, control.White, 32)
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
		// Splash screen: logo + experiment name, up to 3 s or any key
		if err := stimuli.SplashScreen(exp.Screen, assets_embed.LogoPNG, "My First Go Experiment", 3); err != nil {
			return err
		}

		if err := stimuli.PlayPing(exp.AudioDevice); err != nil {
			log.Printf("Warning: failed to play ping: %v", err)
		}
		if err := exp.Show(instr); err != nil {
			return err
		}
		exp.Keyboard.Wait()
		sound.Play()

		if err := exp.Show(greetings); err != nil {
			return err
		}
		exp.Keyboard.Wait()

		if err := exp.Show(finish); err != nil {
			return err
		}
		exp.Keyboard.Wait()

		return control.EndLoop
	})

}
