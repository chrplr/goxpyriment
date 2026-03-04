package main

import (
	_ "embed"
	"fmt"
	"goxpyriment/control"
	"goxpyriment/design"
	"goxpyriment/misc"
	"goxpyriment/stimuli"
	"log"

	"github.com/Zyko0/go-sdl3/sdl"
)

// Assets embedded in the binary
//
//go:embed assets/Roboto-Regular.ttf
var robotoFont []byte

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
	CardW, CardH        = 300, 458
	Gap                 = 100
	Shift               = CardW + Gap
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
	// 1. Initialize experiment
	exp := control.NewExperiment("Mental Logic Card Game", 1200, 800, false)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	if err := exp.LoadFontFromMemory(robotoFont, 24); err != nil {
		log.Printf("Warning: failed to load font: %v", err)
	}

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
	canvas1 := stimuli.NewCanvas(float32(3*CardW+2*Gap), float32(CardH), sdl.Color{A: 0})
	canvas2 := stimuli.NewCanvas(float32(3*CardW+2*Gap), float32(CardH), sdl.Color{A: 0})

	// 4. Run Experiment
	err := exp.Run(func() error {
		// Instructions
		instr := stimuli.NewTextBox(Instructions, 700, sdl.FPoint{X: 0, Y: 0}, control.DefaultTextColor)
		if err := instr.Present(exp.Screen, true, true); err != nil {
			return err
		}
		if _, err := exp.Keyboard.WaitKeys([]sdl.Keycode{sdl.K_SPACE}, -1); err != nil {
			return err
		}

		for _, ct := range trials {
			// Update Canvases
			canvas1.Clear(exp.Screen)
			for i := 0; i < 3; i++ {
				p := pics[ct.Backs[i]]
				p.SetPosition(sdl.FPoint{X: float32(-Shift + i*Shift), Y: 0})
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
				p.SetPosition(sdl.FPoint{X: float32(-Shift + i*Shift), Y: 0})
				canvas2.Blit(p, exp.Screen)
			}

			// Trial Sequence
			if err := exp.Screen.Clear(); err != nil { return err }
			if err := exp.Screen.Update(); err != nil { return err }
			misc.Wait(1000)

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

			if err := exp.Screen.Clear(); err != nil { return err }
			if err := exp.Screen.Update(); err != nil { return err }
			misc.Wait(InterTrialTime)
		}

		return sdl.EndLoop
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}

func waitResponse(exp *control.Experiment, timeout int) (sdl.Keycode, int64, error) {
	start := misc.GetTime()
	key, err := exp.Keyboard.WaitKeys([]sdl.Keycode{sdl.K_Q, sdl.K_S, sdl.K_D, sdl.K_N}, timeout)
	return key, misc.GetTime() - start, err
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
