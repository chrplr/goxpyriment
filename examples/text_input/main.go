package main

import (
	_ "embed"
	"fmt"
	"goxpyriment/control"
	"goxpyriment/stimuli"
	"log"

	"github.com/Zyko0/go-sdl3/sdl"
)

//go:embed assets/Roboto-Regular.ttf
var robotoFont []byte

func main() {
	// 1. Create and initialize the experiment
	exp := control.NewExperiment("TextInput Demo", 800, 600, false)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	if err := exp.LoadFontFromMemory(robotoFont, 32); err != nil {
		log.Printf("Warning: failed to load font: %v. Using fallback.", err)
	}

	// 2. Prepare TextInput
	ti := stimuli.NewTextInput("Please enter your name:", 
		sdl.FPoint{X: 0, Y: 0}, 
		400, 
		sdl.Color{R: 50, G: 50, B: 50, A: 255}, 
		sdl.Color{R: 200, G: 200, B: 200, A: 255}, 
		sdl.Color{R: 255, G: 255, B: 255, A: 255})

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
			sdl.Color{R: 255, G: 255, B: 255, A: 255})
		
		if _, err := msg.Get(exp.Screen, exp.Keyboard); err != nil {
			return err
		}

		return sdl.EndLoop
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
