// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/misc"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"
	"math"
)

const (
	NumPositions = 20
	Radius       = 200.0
	SquareSize   = 20.0
	MaskGap      = 2.0
)

func getCirclePoints(num int, radius float32) []sdl.FPoint {
	points := make([]sdl.FPoint, num)
	for i := 0; i < num; i++ {
		angle := float64(i) * 2.0 * math.Pi / float64(num)
		points[i] = sdl.FPoint{
			X: radius * float32(math.Cos(angle)),
			Y: radius * float32(math.Sin(angle)),
		}
	}
	return points
}

type TrialConfig struct {
	TargetPosIdx int     // 0-19, or -1 for blank
	Delay        float64 // in seconds
	HasDistractor bool
	DistractorPosIdx int
}

func runTrial(exp *control.Experiment, config TrialConfig, points []sdl.FPoint, fixation *stimuli.FixCross) (string, int, error) {
	targetColor := sdl.Color{R: 89, G: 89, B: 89, A: 255}
	
	// 1. Fixation (500ms)
	if err := fixation.Present(exp.Screen, true, true); err != nil { return "", 0, err }
	misc.Wait(500)

	// 2. Target (17ms)
	if config.TargetPosIdx >= 0 {
		p := points[config.TargetPosIdx]
		target := stimuli.NewRectangle(p.X, p.Y, SquareSize, SquareSize, targetColor)
		if err := exp.Screen.Clear(); err != nil { return "", 0, err }
		if err := fixation.Draw(exp.Screen); err != nil { return "", 0, err }
		if err := target.Draw(exp.Screen); err != nil { return "", 0, err }
		if err := exp.Screen.Update(); err != nil { return "", 0, err }
	} else {
		if err := fixation.Present(exp.Screen, true, true); err != nil { return "", 0, err }
	}
	misc.Wait(17)

	// 3. Post-target Fixation (17ms)
	if err := fixation.Present(exp.Screen, true, true); err != nil { return "", 0, err }
	misc.Wait(17)

	// 4. Mask (233ms)
	// Mask consists of 4 squares at EVERY possible target location
	if err := exp.Screen.Clear(); err != nil { return "", 0, err }
	if err := fixation.Draw(exp.Screen); err != nil { return "", 0, err }
	for _, p := range points {
		// 4 mask squares surrounding the location
		maskColor := control.White // Usually calibrated, but using white as placeholder
		offset := float32(SquareSize + MaskGap)
		m1 := stimuli.NewRectangle(p.X+offset, p.Y, SquareSize, SquareSize, maskColor)
		m1.Draw(exp.Screen)
		m2 := stimuli.NewRectangle(p.X-offset, p.Y, SquareSize, SquareSize, maskColor)
		m2.Draw(exp.Screen)
		m3 := stimuli.NewRectangle(p.X, p.Y+offset, SquareSize, SquareSize, maskColor)
		m3.Draw(exp.Screen)
		m4 := stimuli.NewRectangle(p.X, p.Y-offset, SquareSize, SquareSize, maskColor)
		m4.Draw(exp.Screen)
	}
	if err := exp.Screen.Update(); err != nil { return "", 0, err }
	misc.Wait(233)

	// 5. Delay Period (2.5, 3.0, 3.5, 4.0s)
	delayStart := misc.GetTime()
	distractorShown := false
	for {
		elapsed := float64(misc.GetTime()-delayStart) / 1000.0
		if elapsed >= config.Delay {
			break
		}

		if config.HasDistractor && !distractorShown && elapsed >= 1.5 {
			dp := points[config.DistractorPosIdx]
			distractor := stimuli.NewRectangle(dp.X, dp.Y, SquareSize, SquareSize, targetColor)
			if err := exp.Screen.Clear(); err != nil { return "", 0, err }
			if err := fixation.Draw(exp.Screen); err != nil { return "", 0, err }
			if err := distractor.Draw(exp.Screen); err != nil { return "", 0, err }
			if err := exp.Screen.Update(); err != nil { return "", 0, err }
			misc.Wait(17)
			distractorShown = true
		}

		if err := fixation.Present(exp.Screen, true, true); err != nil { return "", 0, err }
		
		// Poll for events to allow exiting
		if _, _, err := exp.HandleEvents(); err != nil {
			return "", 0, err
		}
		
		misc.Wait(10)
	}

	// 6. Response Screen (Letters at 20 positions) (2.5s)
	letters := []string{"a", "b", "c", "d", "f", "g", "h", "i", "k", "l", "m", "o", "q", "r", "s", "u", "w", "x", "y", "z"}
	design.ShuffleList(letters)
	
	if err := exp.Screen.Clear(); err != nil { return "", 0, err }
	if err := fixation.Draw(exp.Screen); err != nil { return "", 0, err }
	for i, p := range points {
		t := stimuli.NewTextLine(letters[i], p.X, p.Y, control.White)
		t.Draw(exp.Screen)
	}
	if err := exp.Screen.Update(); err != nil { return "", 0, err }
	
	misc.Wait(2500)

	// 7. Visibility Rating (PAS)
	vuText := stimuli.NewTextLine("Vu?", 0, 0, control.White)
	if err := vuText.Present(exp.Screen, true, true); err != nil { return "", 0, err }
	
	ratingKey, err := exp.Keyboard.WaitKeys([]sdl.Keycode{sdl.K_1, sdl.K_2, sdl.K_3, sdl.K_4, sdl.K_KP_1, sdl.K_KP_2, sdl.K_KP_3, sdl.K_KP_4}, 2500)
	if err != nil { return "", 0, err }
	
	rating := 0
	switch ratingKey {
	case sdl.K_1, sdl.K_KP_1: rating = 1
	case sdl.K_2, sdl.K_KP_2: rating = 2
	case sdl.K_3, sdl.K_KP_3: rating = 3
	case sdl.K_4, sdl.K_KP_4: rating = 4
	}

	// ITI (1s)
	if err := exp.Screen.Clear(); err != nil { return "", 0, err }
	exp.Screen.Update()
	misc.Wait(1000)

	return "n/a", rating, nil
}

func showInstructions(exp *control.Experiment) error {
	text := "Unconscious Working Memory Experiment\n\n" +
		"1. Fixate on the central cross throughout the trial.\n" +
		"2. A very faint target square will be flashed briefly.\n" +
		"3. After a delay, letters will appear at all possible locations.\n" +
		"4. Note the letter at the location where the target appeared.\n" +
		"5. Finally, rate how well you saw the target (1-4):\n" +
		"   1: No experience (unseen)\n" +
		"   2: Brief glimpse\n" +
		"   3: Almost clear experience\n" +
		"   4: Clear experience\n\n" +
		"Press any key to begin."

	instrBox := stimuli.NewTextBox(text, 650, sdl.FPoint{X: 0, Y: 0}, control.White)
	if err := instrBox.Present(exp.Screen, true, true); err != nil {
		return err
	}
	_, err := exp.Keyboard.Wait()
	return err
}

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 800x600)")
	flag.Parse()

	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 800, 600, false
	}

	exp := control.NewExperiment("Unconscious-Working-Memory", width, height, fullscreen)
	exp.BackgroundColor = control.Black

	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// Show instructions
	if err := showInstructions(exp); err != nil {
		if err == sdl.EndLoop { return }
		log.Fatalf("instruction error: %v", err)
	}

	exp.Data.AddVariableNames([]string{"trial", "target_idx", "delay", "distractor", "rating"})

	points := getCirclePoints(NumPositions, Radius)
	fixation := stimuli.NewFixCross(20, 2, control.White)

	var trialConfigs []TrialConfig
	for loc := 0; loc < NumPositions; loc++ {
		for rep := 0; rep < 8; rep++ {
			trialConfigs = append(trialConfigs, TrialConfig{
				TargetPosIdx: loc,
				Delay:        []float64{2.5, 3.0, 3.5, 4.0}[design.RandInt(0, 3)],
				HasDistractor: design.CoinFlip(0.5),
				DistractorPosIdx: design.RandInt(0, NumPositions-1),
			})
		}
	}
	for i := 0; i < 40; i++ {
		trialConfigs = append(trialConfigs, TrialConfig{
			TargetPosIdx: -1,
			Delay:        []float64{2.5, 3.0, 3.5, 4.0}[design.RandInt(0, 3)],
			HasDistractor: design.CoinFlip(0.5),
			DistractorPosIdx: design.RandInt(0, NumPositions-1),
		})
	}
	design.ShuffleList(trialConfigs)

	for i, config := range trialConfigs {
		_, rating, err := runTrial(exp, config, points, fixation)
		if err != nil {
			if err == sdl.EndLoop { break }
			log.Fatalf("trial error: %v", err)
		}
		
		exp.Data.Add([]interface{}{
			i + 1, config.TargetPosIdx, config.Delay, config.HasDistractor, rating,
		})
	}
}
