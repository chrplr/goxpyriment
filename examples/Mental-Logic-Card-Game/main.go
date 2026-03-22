// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
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
	BackDisplayDuration  = 1000
	MaxResponseTime      = 5000
	InterTrialTime       = 2000
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
	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	scaling := flag.Float64("scaling", 0.6, "Scaling factor for stimuli")
	fullscreenFlag := flag.Bool("F", false, "Force Fullscreen")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	// Scaled dimensions
	cardW := float32(BaseCardW) * float32(*scaling)
	cardH := float32(BaseCardH) * float32(*scaling)
	gap := float32(BaseGap) * float32(*scaling)
	shift := cardW + gap

	// 1. Initialize experiment
	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}
	if *fullscreenFlag {
		fullscreen = true
	}
	exp := control.NewExperiment("Mental Logic Card Game", width, height, fullscreen, control.Black, control.White, 22)
	exp.SubjectID = *subject
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// Prepare event log header and write it as comments in the data file.
	// We log condition, response key, RT, and correctness.
	evLog := exp.CollectEventLog()
	evLog.SetSubjectID(fmt.Sprintf("%d", exp.SubjectID))
	evLog.SetCSVHeader([]string{"condition", "response", "rt", "correct"})
	exp.Data.WriteComment("--EVENT LOG")
	exp.Data.WriteComment(evLog.String())
	exp.Data.WriteComment("--TRIAL DATA")

	// Set logical size to ensure consistent centering and coordinates
	if err := exp.SetLogicalSize(1280, 1024); err != nil {
		log.Printf("Warning: failed to set logical size: %v", err)
	}

	// Wait for fullscreen transition to stabilize
	// if fullscreen {
	//	clock.Wait(2000)
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
	canvas1 := stimuli.NewCanvas(3*cardW+2*gap, cardH, control.Color{A: 0})
	canvas2 := stimuli.NewCanvas(3*cardW+2*gap, cardH, control.Color{A: 0})

	// 4. Run Experiment
	err := exp.Run(func() error {
		// Wait for fullscreen transition to stabilize before showing instructions
		if fullscreen {
			clock.Wait(2000)
		}

		// Instructions - Initialize here to ensure correct centering after fullscreen transition
		instr := stimuli.NewTextBox(Instructions, 1000, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)

		// Small extra wait to ensure renderer is ready
		clock.Wait(100)

		if err := exp.Show(instr); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}

		for _, ct := range trials {
			// Update Canvases
			canvas1.Clear(exp.Screen)
			for i := 0; i < 3; i++ {
				p := pics[ct.Backs[i]]
				p.SetPosition(control.FPoint{X: -shift + float32(i)*shift, Y: 0})
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
				p.SetPosition(control.FPoint{X: -shift + float32(i)*shift, Y: 0})
				canvas2.Blit(p, exp.Screen)
			}

			// Pre-trial Fixation
			if err := exp.Show(fixation); err != nil {
				return err
			}
			if err := waitInterruption(exp, 1000); err != nil {
				return err
			}

			// Show Backs
			if err := exp.Show(canvas1); err != nil {
				return err
			}

			resp, rt, err := waitResponse(exp, BackDisplayDuration)
			if err != nil && !control.IsEndLoop(err) {
				return err
			}

			// Show Turned Card
			if err := exp.Show(canvas2); err != nil {
				return err
			}

			if resp == 0 {
				resp, rt, err = waitResponse(exp, MaxResponseTime)
				if err != nil && !control.IsEndLoop(err) {
					return err
				}
				rt += int64(BackDisplayDuration)
			}

			// Check correctness
			correct := checkCorrect(resp, ct.Expected)
			exp.Data.Add(ct.Condition, resp, rt, correct)

			fmt.Printf("Trial: %s, Resp: %d, RT: %d, Correct: %v\n", ct.Condition, resp, rt, correct)

			// Inter-trial Fixation
			if err := exp.Show(fixation); err != nil {
				return err
			}
			if err := waitInterruption(exp, InterTrialTime); err != nil {
				return err
			}
		}

		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}

func waitInterruption(exp *control.Experiment, timeout int) error {
	clock.Wait(timeout)
	return nil
}

func waitResponse(exp *control.Experiment, timeout int) (control.Keycode, int64, error) {
	responseKeys := []control.Keycode{control.K_Q, control.K_S, control.K_D, control.K_N}
	key, rt, err := exp.Keyboard.WaitKeysRT(responseKeys, timeout)
	return key, rt, err
}

func checkCorrect(key control.Keycode, expected int) bool {
	mapping := map[control.Keycode]int{
		control.K_Q: 0,
		control.K_S: 1,
		control.K_D: 2,
		control.K_N: -1,
	}
	val, ok := mapping[key]
	if !ok {
		return false
	}
	return val == expected
}
