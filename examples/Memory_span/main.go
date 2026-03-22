// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"fmt"
	"log"
	"math/rand"
	"strings"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

type StimType int

const (
	TypeDigit StimType = iota
	TypeLetter
	TypeWord
)

func (t StimType) String() string {
	switch t {
	case TypeDigit:
		return "Digit"
	case TypeLetter:
		return "Letter"
	case TypeWord:
		return "Word"
	default:
		return "Unknown"
	}
}

var Digits = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
var Letters = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}
var Words = []string{"CAT", "DOG", "SUN", "HAT", "BOX", "RED", "BIG", "FLY", "CUP", "BED", "PEN", "RUN", "EAT", "SIT", "TOP"}

type Button struct {
	Rect   *stimuli.Rectangle
	Text   *stimuli.TextLine
	Value  string
	Bounds sdl.FRect // in SDL coordinates
}

func NewButton(value string, x, y, w, h float32, screen *control.Experiment) *Button {
	rect := stimuli.NewRectangle(x, y, w, h, control.LightGray)
	text := stimuli.NewTextLine(value, x, y, control.Black)

	sdlX, sdlY := screen.Screen.CenterToSDL(x, y)
	bounds := sdl.FRect{
		X: sdlX - w/2,
		Y: sdlY - h/2,
		W: w,
		H: h,
	}

	return &Button{
		Rect:   rect,
		Text:   text,
		Value:  value,
		Bounds: bounds,
	}
}

func (b *Button) Draw(screen *control.Experiment) error {
	if err := b.Rect.Draw(screen.Screen); err != nil {
		return err
	}
	return b.Text.Draw(screen.Screen)
}

func (b *Button) IsClicked(x, y float32) bool {
	return x >= b.Bounds.X && x <= b.Bounds.X+b.Bounds.W &&
		y >= b.Bounds.Y && y <= b.Bounds.Y+b.Bounds.H
}

func main() {
	exp := control.NewExperimentFromFlags("Memory Span", control.Black, control.White, 32)
	defer exp.End()

	if err := exp.SetLogicalSize(1368, 1024); err != nil {
		log.Printf("Warning: failed to set logical size: %v", err)
	}

	exp.AddDataVariableNames([]string{"trial", "type", "length", "sequence", "response", "correct"})

	// Staircase lengths for each type
	lengths := map[StimType]int{
		TypeDigit:  3,
		TypeLetter: 3,
		TypeWord:   3,
	}

	// Prepare trials: 10 of each type
	var trialTypes []StimType
	for i := 0; i < 10; i++ {
		trialTypes = append(trialTypes, TypeDigit, TypeLetter, TypeWord)
	}
	rand.Shuffle(len(trialTypes), func(i, j int) {
		trialTypes[i], trialTypes[j] = trialTypes[j], trialTypes[i]
	})

	err := exp.Run(func() error {
		// Instructions
		instrText := "In this experiment, you will see sequences of digits, letters, or words.\n\n" +
			"Each item will appear for one second.\n" +
			"After the sequence, click on the buttons in the SAME ORDER as presented.\n\n" +
			"If you are correct, the sequence will get longer.\n" +
			"If you make a mistake, it will get shorter.\n\n" +
			"There will be 30 trials in total.\n\n" +
			"Press SPACEBAR to start."
		instructions := stimuli.NewTextBox(instrText, 1000, control.Point(0, 0), control.White)
		if err := exp.Show(instructions); err != nil {
			return err
		}

		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}

		for i, tType := range trialTypes {
			length := lengths[tType]

			// Select stimuli
			var pool []string
			switch tType {
			case TypeDigit:
				pool = Digits
			case TypeLetter:
				pool = Letters
			case TypeWord:
				pool = Words
			}

			sequence := make([]string, length)
			for j := 0; j < length; j++ {
				sequence[j] = pool[rand.Intn(len(pool))]
			}

			// Presentation
			for _, item := range sequence {
				if err := exp.Screen.Clear(); err != nil {
					return err
				}
				stim := stimuli.NewTextLine(item, 0, 0, control.White)
				if err := stim.Draw(exp.Screen); err != nil {
					return err
				}
				if err := exp.Screen.Update(); err != nil {
					return err
				}
				clock.Wait(1000)

				if err := exp.Blank(200); err != nil {
					return err
				}
			}

			// Response Phase
			buttons := createButtons(pool, exp)
			response := make([]string, 0, length)

			for len(response) < length {
				if err := drawButtons(exp, buttons, response); err != nil {
					return err
				}

				clickedValue, err := waitForClick(exp, buttons)
				if err != nil {
					return err
				}
				response = append(response, clickedValue)
			}

			// Feedback and Staircase
			correct := true
			for j := range sequence {
				if sequence[j] != response[j] {
					correct = false
					break
				}
			}

			feedbackDuration := 1500
			if correct {
				lengths[tType]++
				msg := stimuli.NewTextLine("CORRECT!", 0, 0, control.Green)
				msg.Present(exp.Screen, true, true)
			} else {
				if lengths[tType] > 1 {
					lengths[tType]--
				}
				msg := stimuli.NewTextLine(fmt.Sprintf("WRONG! Correct was: %s", strings.Join(sequence, " ")), 0, 0, control.Red)
				msg.Present(exp.Screen, true, true)
				if err := exp.Audio.PlayBuzzer(); err != nil {
					log.Printf("Warning: buzzer failed: %v", err)
				}
				feedbackDuration = 4000
			}
			clock.Wait(feedbackDuration)

			exp.Data.Add(i+1, tType.String(), length, strings.Join(sequence, " "), strings.Join(response, " "), correct)
		}

		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}

func createButtons(pool []string, exp *control.Experiment) []*Button {
	var buttons []*Button
	n := len(pool)
	cols := 5
	if n > 15 {
		cols = 7
	}
	rows := (n + cols - 1) / cols

	w, h := float32(120), float32(60)
	margin := float32(20)

	totalW := float32(cols)*w + float32(cols-1)*margin
	totalH := float32(rows)*h + float32(rows-1)*margin

	startX := -totalW/2 + w/2
	startY := totalH/2 - h/2

	for i, val := range pool {
		r := i / cols
		c := i % cols
		x := startX + float32(c)*(w+margin)
		y := startY - float32(r)*(h+margin)
		buttons = append(buttons, NewButton(val, x, y, w, h, exp))
	}
	return buttons
}

func drawButtons(exp *control.Experiment, buttons []*Button, response []string) error {
	if err := exp.Screen.Clear(); err != nil {
		return err
	}

	// Show current response sequence
	respText := "Your response: " + strings.Join(response, " ")
	respStim := stimuli.NewTextLine(respText, 0, 400, control.White)
	if err := respStim.Draw(exp.Screen); err != nil {
		return err
	}

	for _, b := range buttons {
		if err := b.Draw(exp); err != nil {
			return err
		}
	}
	return exp.Screen.Update()
}

func waitForClick(exp *control.Experiment, buttons []*Button) (string, error) {
	for {
		var clickedValue string
		found := false

		state := exp.PollEvents(func(ev sdl.Event) bool {
			if ev.Type == sdl.EVENT_MOUSE_BUTTON_DOWN {
				mev := ev.MouseButtonEvent()
				// mev.X/Y are in window-pixel space; convert to logical
				// renderer space so they match the button bounds computed
				// via CenterToSDL (which uses the logical presentation size).
				lx, ly, err := exp.Screen.Renderer.RenderCoordinatesFromWindow(mev.X, mev.Y)
				if err != nil {
					lx, ly = mev.X, mev.Y
				}
				for _, b := range buttons {
					if b.IsClicked(lx, ly) {
						clickedValue = b.Value
						found = true
						return true
					}
				}
			}
			return false
		})

		if state.QuitRequested {
			return "", sdl.EndLoop
		}
		if found {
			return clickedValue, nil
		}

		clock.Wait(10)
	}
}
