package main

import (
	"slices"
	"strconv"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("Parity Decision",
		control.Black, control.White, 32)
	defer exp.End()

	Targets := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	NTrialsPerTarget := 2
	EvenResponse := control.K_F
	OddResponse := control.K_J
	Instructions := "When you'll see a number, your task to decide, " +
		"as quickly as possible, whether it is even or odd.\n\n" +
		"if it is even, press 'F'\n\nif it is odd, press 'J'"

	cue := stimuli.NewFixCross(25, 2, control.DefaultTextColor)
	// creates a map number -> image
	Image := make(map[int]*stimuli.TextLine)
	for _, num := range Targets {
		Image[num] = stimuli.NewTextLine(strconv.Itoa(num),
			0, 0, control.DefaultTextColor)
	}

	trials := slices.Repeat(Targets, NTrialsPerTarget)
	design.ShuffleList(trials)

	exp.AddDataVariableNames([]string{"number", "key", "rt",
		"correct"})

	exp.Run(func() error {
		exp.ShowInstructions(Instructions)

		for _, t := range trials {
			exp.Blank(1000)
			exp.ShowTimed(cue, 500)
			key, rt, _ := exp.ShowAndGetRT(Image[t],
				[]control.Keycode{EvenResponse, OddResponse},
				-1)
			correct := (t%2 == 0) == (key == EvenResponse)
			exp.Data.Add(t, key, rt, correct)
			if !correct {
				exp.Audio.PlayBuzzer()
			}
			exp.Wait(500)
		}

		return control.EndLoop
	})

	exp.ShowEndMessage("Experiment complete. Thank you!\n\n" +
		"Press any key to exit.")
}
