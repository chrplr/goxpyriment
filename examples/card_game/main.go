// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"flag"
	"fmt"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/misc"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"

	"github.com/Zyko0/go-sdl3/sdl"
)

// Assets embedded in the binary
//
//go:embed assets/cards_pics_small/QH.png
var qhImg []byte

//go:embed assets/cards_pics_small/AS.png
var asImg []byte

//go:embed assets/cards_pics_small/AD.png
var adImg []byte

//go:embed assets/cards_pics_small/red_back.png
var redBackImg []byte

//go:embed assets/cards_pics_small/blue_back.png
var blueBackImg []byte

const (
	BackDisplayDuration = 1000
	MaxResponseTime     = 5000
	InterTrialTime      = 2000
	BaseCardW, BaseCardH = 300, 458
	BaseGap              = 100
)

var (
	Instructions = `
 Three cards are going to be used throughout the experiment:

 * Queen of Hearts (red back)
 * Black Spade Ace (blue back)
 * Red Diamond Ace (blue back)

 Note that only the queen card has a red back while the two ace cards have blue backs.

 The experiment is a series of trials where the three cards are presented in a row.
 First their backs are presented, then one of the three cards is unmasked a second later.

 Your task is to identify the *middle* card as quickly as possible, when possible.
 You will indicate your response by pressing a key:

 Queen -> Press 'Q'
 Black Spade Ace -> Press 'S'
 Red Diamond Ace -> Press 'D'
 Dont know -> Press 'N'

 Now, rest your fingers on the keys 'Q', 'S', 'D' and 'N'.

 When you are ready, press the SPACE BAR to start.`
)

type cardTrial struct {
	Condition string
	Backs     [3]string
	TurnPos   int // 1, 2, 3
	FrontCard string
	Expected  int // 0:Q, 1:S, 2:D, -1:N
}

func main() {
	develop := flag.Bool("d", false, "Develop mode (windowed display)")
	scaling := flag.Float64("scaling", 1.0, "Scaling factor for stimuli")
	fullscreenFlag := flag.Bool("F", false, "Force Fullscreen")
	flag.Parse()

	// Default is fullscreen unless develop mode is requested
	isFullscreen := !*develop
	if *fullscreenFlag {
		isFullscreen = true
	}

	var winW, winH int
	winW, winH = 1280, 1024

	// Scaled dimensions
	cardW := float32(BaseCardW) * float32(*scaling)
	cardH := float32(BaseCardH) * float32(*scaling)
	gap := float32(BaseGap) * float32(*scaling)
	shift := cardW + gap

	// 1. Initialize experiment
	exp := control.NewExperiment("Mental Logic Card Game", winW, winH, isFullscreen)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// Set logical size to ensure consistent centering and coordinates
	if err := exp.SetLogicalSize(1280, 1024); err != nil {
		log.Printf("Warning: failed to set logical size: %v", err)
	}

	// Wait for fullscreen transition to stabilize
	// if isFullscreen {
	//	misc.Wait(2000)
	// }

	exp.AddDataVariableNames([]string{"condition", "response", "rt", "correct"})

	// 2. Prepare assets from memory
	pics := map[string]*stimuli.Picture{
		"QH":        stimuli.NewPictureFromMemory(qhImg, 0, 0),
		"AS":        stimuli.NewPictureFromMemory(asImg, 0, 0),
		"AD":        stimuli.NewPictureFromMemory(adImg, 0, 0),
		"red_back":  stimuli.NewPictureFromMemory(redBackImg, 0, 0),
		"blue_back": stimuli.NewPictureFromMemory(blueBackImg, 0, 0),
	}
	// Stimuli will be preloaded automatically on first Draw
	for _, p := range pics {
		p.Width = float32(cardW)
		p.Height = float32(cardH)
	}

	// Fixation Cross
	fixation := stimuli.NewFixCross(50*float32(*scaling), 4*float32(*scaling), control.DefaultTextColor)

	// 3. Define trials (More complete set representing conditions)
	trials := []cardTrial{
		// Inference: Queen at 1, reveal Ace at 3 -> Infer middle Ace
		{"Inference", [3]string{"red_back", "blue_back", "blue_back"}, 3, "AS", 2},
		{"Inference", [3]string{"red_back", "blue_back", "blue_back"}, 3, "AD", 1},
		// NoInf: Queen at 2, reveal Ace at 1 -> Identify middle Queen
		{"NoInf", [3]string{"blue_back", "red_back", "blue_back"}, 1, "AS", 0},
		// LackOfInf: Queen at 1, reveal Queen at 1 -> Can't decide middle
		{"LackOfInf", [3]string{"red_back", "blue_back", "blue_back"}, 1, "QH", -1},
	}
	design.ShuffleList(trials)

	// Prepare reusable canvases
	canvas1 := stimuli.NewCanvas(3*cardW+2*gap, cardH, sdl.Color{A: 0})
	canvas2 := stimuli.NewCanvas(3*cardW+2*gap, cardH, sdl.Color{A: 0})

	// 4. Run Experiment
	err := exp.Run(func() error {
		// Wait for fullscreen transition to stabilize before showing instructions
		if isFullscreen {
			misc.Wait(2000)
		}

		// Instructions - Initialize here to ensure correct centering after fullscreen transition
		instr := stimuli.NewTextBox(Instructions, 1000, sdl.FPoint{X: 0, Y: 0}, control.DefaultTextColor)
		
		// Small extra wait to ensure renderer is ready
		misc.Wait(100)
		
		if err := instr.Present(exp.Screen, true, true); err != nil {
			return err
		}
		for {
			key, _, err := exp.HandleEvents()
			if err != nil {
				return err
			}
			if key == sdl.K_SPACE {
				break
			}
			misc.Wait(10)
		}

		for _, ct := range trials {
			// Update Canvases
			canvas1.Clear(exp.Screen)
			for i := 0; i < 3; i++ {
				p := pics[ct.Backs[i]]
				p.SetPosition(sdl.FPoint{X: -shift + float32(i)*shift, Y: 0})
				canvas1.Blit(p, exp.Screen)
			}

			canvas2.Clear(exp.Screen)
			for i := 0; i < 3; i++ {
				var p *stimuli.Picture
				if i == ct.TurnPos-1 {
					p = pics[ct.FrontCard]
				} else {
					p = pics[ct.Backs[i]]
				}
				p.SetPosition(sdl.FPoint{X: -shift + float32(i)*shift, Y: 0})
				canvas2.Blit(p, exp.Screen)
			}

			// Pre-trial Fixation
			if err := fixation.Present(exp.Screen, true, true); err != nil { return err }
			if err := waitInterruption(exp, 1000); err != nil { return err }

			// Show Backs
			if err := canvas1.Present(exp.Screen, true, true); err != nil { return err }
			
			resp, rt, err := waitResponse(exp, BackDisplayDuration)
			if err != nil && err != sdl.EndLoop { return err }

			// Show Turned Card
			if err := canvas2.Present(exp.Screen, true, true); err != nil { return err }
			
			if resp == 0 {
				resp, rt, err = waitResponse(exp, MaxResponseTime)
				if err != nil && err != sdl.EndLoop { return err }
				rt += int64(BackDisplayDuration)
			}

			// Check correctness
			correct := checkCorrect(resp, ct.Expected)
			exp.Data.Add([]interface{}{ct.Condition, resp, rt, correct})
			
			fmt.Printf("Trial: %s, Resp: %d, RT: %d, Correct: %v\n", ct.Condition, resp, rt, correct)

			// Inter-trial Fixation
			if err := fixation.Present(exp.Screen, true, true); err != nil { return err }
			if err := waitInterruption(exp, InterTrialTime); err != nil { return err }
		}

		return sdl.EndLoop
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}

func waitInterruption(exp *control.Experiment, timeout int) error {
	start := misc.GetTime()
	for {
		_, _, err := exp.HandleEvents()
		if err != nil {
			return err
		}
		if int(misc.GetTime()-start) >= timeout {
			return nil
		}
		misc.Wait(1)
	}
}

func waitResponse(exp *control.Experiment, timeout int) (sdl.Keycode, int64, error) {
	start := misc.GetTime()
	for {
		key, _, err := exp.HandleEvents()
		if err != nil {
			return 0, misc.GetTime() - start, err
		}
		if key == sdl.K_Q || key == sdl.K_S || key == sdl.K_D || key == sdl.K_N {
			return key, misc.GetTime() - start, nil
		}
		if timeout > 0 && int(misc.GetTime()-start) >= timeout {
			return 0, int64(timeout), nil
		}
		misc.Wait(1)
	}
}

func checkCorrect(key sdl.Keycode, expected int) bool {
	mapping := map[sdl.Keycode]int{
		sdl.K_Q: 0,
		sdl.K_S: 1,
		sdl.K_D: 2,
		sdl.K_N: -1,
	}
	val, ok := mapping[key]
	if !ok { return false }
	return val == expected
}
