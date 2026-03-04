package main

import (
	_ "embed"
	"goxpyriment/control"
	"goxpyriment/stimuli"
	"log"

	"github.com/Zyko0/go-sdl3/sdl"
)

//go:embed assets/Roboto-Regular.ttf
var robotoFont []byte

func main() {
	// 1. Create and initialize the experiment
	exp := control.NewExperiment("Stimuli Extras Showcase", 800, 600, false)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	if err := exp.LoadFontFromMemory(robotoFont, 24); err != nil {
		log.Printf("Warning: failed to load font: %v. Using fallback.", err)
	}

	// 2. Prepare stimuli
	
	// VisualMask
	mask := stimuli.NewVisualMask(400, 400, 10, 10, 
		sdl.Color{R: 127, G: 127, B: 127, A: 255}, 
		sdl.Color{R: 0, G: 0, B: 0, A: 255}, 50)
	
	// GaborPatch
	gabor := stimuli.NewGaborPatch(40, 45, 20, 0, 0, 1, 
		sdl.Color{R: 127, G: 127, B: 127, A: 255}, 300)
	
	// DotCloud
	cloud := stimuli.NewDotCloud(200, 
		sdl.Color{R: 50, G: 50, B: 50, A: 255}, 
		sdl.Color{R: 255, G: 255, B: 255, A: 255})
	cloud.Make(50, 5, 2)
	
	// StimulusCircle
	var dots []stimuli.VisualStimulus
	for i := 0; i < 12; i++ {
		dots = append(dots, stimuli.NewCircle(10, sdl.Color{R: 0, G: 255, B: 0, A: 255}))
	}
	circle := stimuli.NewStimulusCircle(150, dots)
	circle.Make(true, true)
	
	// ThermometerDisplay
	therm := stimuli.NewThermometerDisplay(sdl.FPoint{X: 50, Y: 300}, 10, 65, 80)

	// Titles
	titleMask := stimuli.NewTextLine("Visual Mask (Press Space)", 0, 250, control.DefaultTextColor)
	titleGabor := stimuli.NewTextLine("Gabor Patch (Press Space)", 0, 250, control.DefaultTextColor)
	titleCloud := stimuli.NewTextLine("Dot Cloud (Press Space)", 0, 250, control.DefaultTextColor)
	titleCircle := stimuli.NewTextLine("Stimulus Circle (Press Space)", 0, 250, control.DefaultTextColor)
	titleTherm := stimuli.NewTextLine("Thermometer Display (Press Space)", 0, 250, control.DefaultTextColor)

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		show := func(title *stimuli.TextLine, stim stimuli.VisualStimulus) error {
			if err := exp.Screen.Clear(); err != nil { return err }
			if err := title.Draw(exp.Screen); err != nil { return err }
			if err := stim.Draw(exp.Screen); err != nil { return err }
			if err := exp.Screen.Update(); err != nil { return err }
			_, err := exp.Keyboard.Wait()
			return err
		}

		if err := show(titleMask, mask); err != nil { return err }
		if err := show(titleGabor, gabor); err != nil { return err }
		if err := show(titleCloud, cloud); err != nil { return err }
		if err := show(titleCircle, circle); err != nil { return err }
		if err := show(titleTherm, therm); err != nil { return err }

		return sdl.EndLoop
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
