package main
import (
	_ "embed"
	"flag"
	"goxpyriment/control"
	"goxpyriment/stimuli"
	"log"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

//go:embed assets/Inconsolata.ttf
var inconsolataFont []byte

func main() {
	fullscreen := flag.Bool("F", false, "Launch in fullscreen display mode")
	flag.Parse()

	// 1. Create and initialize the experiment
	exp := control.NewExperiment("Canvas Demo", 1368, 1024, *fullscreen)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	if err := exp.LoadFontFromMemory(inconsolataFont, 24); err != nil {
		log.Printf("Warning: failed to load font: %v. Using fallback.", err)
	}

	// 2. Prepare Canvas
	canvas := stimuli.NewCanvas(400, 400, sdl.Color{R: 50, G: 50, B: 50, A: 255})
	
	// Prepare sub-stimuli to draw on canvas
	// Coordinates are relative to canvas center (0,0)
	rect := stimuli.NewRectangle(0, 0, 100, 100, sdl.Color{R: 200, G: 0, B: 0, A: 255})
	text := stimuli.NewTextLine("Inside Canvas", 0, -80, sdl.Color{R: 255, G: 255, B: 255, A: 255})
	
	// Title
	title := stimuli.NewTextLine("Canvas Demo (Press Space)", 0, 250, control.DefaultTextColor)

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		// Blit things onto the canvas
		if err := canvas.Blit(rect, exp.Screen); err != nil {
			return err
		}
		if err := canvas.Blit(text, exp.Screen); err != nil {
			return err
		}

		// Present the canvas on the screen
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := title.Draw(exp.Screen); err != nil {
			return err
		}
		if err := canvas.Draw(exp.Screen); err != nil {
			return err
		}
		if err := exp.Screen.Update(); err != nil {
			return err
		}

		_, err := exp.Keyboard.Wait()
		if err != nil {
			return err
		}

		return sdl.EndLoop
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
