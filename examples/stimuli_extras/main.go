// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("Stimuli Extras Showcase", control.Black, control.White, 32)
	defer exp.End()

	// 2. Prepare stimuli

	// VisualMask
	mask := stimuli.NewVisualMask(400, 400, 10, 10,
		control.Gray,
		control.Black, 50)

	// GaborPatch
	gabor := stimuli.NewGaborPatch(40, 45, 20, 0, 0, 1,
		control.Gray, 300)

	// DotCloud
	cloud := stimuli.NewDotCloud(200,
		control.Color{R: 50, G: 50, B: 50, A: 255},
		control.White)
	cloud.Make(50, 5, 2)

	// StimulusCircle
	var dots []stimuli.VisualStimulus
	for i := 0; i < 12; i++ {
		dots = append(dots, stimuli.NewCircle(10, control.Green))
	}
	circle := stimuli.NewStimulusCircle(150, dots)
	circle.Make(true, true)

	// ThermometerDisplay
	therm := stimuli.NewThermometerDisplay(control.FPoint{X: 50, Y: 300}, 10, 65, 80)

	// Titles
	titleMask := stimuli.NewTextLine("Visual Mask (Press Space)", 0, 250, control.DefaultTextColor)
	titleGabor := stimuli.NewTextLine("Gabor Patch (Press Space)", 0, 250, control.DefaultTextColor)
	titleCloud := stimuli.NewTextLine("Dot Cloud (Press Space)", 0, 250, control.DefaultTextColor)
	titleCircle := stimuli.NewTextLine("Stimulus Circle (Press Space)", 0, 250, control.DefaultTextColor)
	titleTherm := stimuli.NewTextLine("Thermometer Display (Press Space)", 0, 250, control.DefaultTextColor)

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		show := func(title *stimuli.TextLine, stim stimuli.VisualStimulus) error {
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := title.Draw(exp.Screen); err != nil {
				return err
			}
			if err := stim.Draw(exp.Screen); err != nil {
				return err
			}
			if err := exp.Screen.Update(); err != nil {
				return err
			}
			_, err := exp.Keyboard.Wait()
			return err
		}

		if err := show(titleMask, mask); err != nil {
			return err
		}
		if err := show(titleGabor, gabor); err != nil {
			return err
		}
		if err := show(titleCloud, cloud); err != nil {
			return err
		}
		if err := show(titleCircle, circle); err != nil {
			return err
		}
		if err := show(titleTherm, therm); err != nil {
			return err
		}

		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
