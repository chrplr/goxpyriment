// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"fmt"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"

	"github.com/Zyko0/go-sdl3/sdl"
)

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	// 1. Create and initialize the experiment
	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}
	exp := control.NewExperiment("TextInput Demo", width, height, fullscreen, control.Black, control.White, 32)
	exp.SubjectID = *subject
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// 2. Prepare TextInput
	ti := stimuli.NewTextInput("Please enter your name:", 
		sdl.FPoint{X: 100, Y: 10}, 
		400, 
		sdl.Color{R: 50, G: 50, B: 50, A: 255}, 
		sdl.Color{R: 200, G: 200, B: 200, A: 255}, 
		control.White)

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		name, err := ti.Get(exp.Screen, exp.Keyboard)
		if err != nil {
			return err
		}

		fmt.Printf("User entered: %s\n", name)
		
		// Show result
		result := fmt.Sprintf("Hello, %s!", name)
		msg := stimuli.NewTextInput(result + "\nPress any key to exit.", 
			sdl.FPoint{X: 0, Y: 0}, 
			400, 
			sdl.Color{R: 50, G: 50, B: 50, A: 255}, 
			sdl.Color{R: 50, G: 50, B: 50, A: 255}, // Hide frame
			control.White)
		
		if _, err := msg.Get(exp.Screen, exp.Keyboard); err != nil {
			return err
		}

		return sdl.EndLoop
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
