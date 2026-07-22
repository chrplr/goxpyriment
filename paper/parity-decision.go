package main

import (
	"strconv"  // to convert int -> string

	. "github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := NewExperimentFromFlags("Parity Decision",
		Black, White, 32)
	defer exp.End()

	Instructions := "When you'll see a number, your task to decide, " +
		"as quickly as possible, whether it is even or odd.\n\n" +
		"if it is even, press 'F'\n\nif it is odd, press 'J'"

	Targets := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	NTrialsPerTarget := 2
	trials := design.RepeatList(Targets, NTrialsPerTarget)
	design.ShuffleList(trials)

	EvenResponse := K_F
	OddResponse := K_J
	respKeys := []Keycode{EvenResponse, OddResponse}

	cross := stimuli.NewFixCross(25, 2, DefaultTextColor)

	// creates a map number -> image
	Image := make(map[int]*stimuli.TextLine)
	for _, num := range Targets {
		Image[num] = stimuli.NewTextLine(strconv.Itoa(num),
			0, 0, DefaultTextColor)
	}

	exp.AddDataVariableNames([]string{"number", "key", "rt",
		"correct"})

	exp.Run(func() error {
		exp.ShowInstructions(Instructions)

		for _, num := range trials {
			exp.Blank(1500)
			exp.ShowTimed(cross, 500)
			key, rt, _ := exp.ShowAndGetRT(Image[num], respKeys, -1)

			correct := (num%2 == 0) == (key == EvenResponse)
			exp.Data.Add(num, key, rt, correct)
			if !correct {
				exp.Audio.PlayBuzzer()
			}
		}
		return EndLoop
	})

	exp.ShowEndMessage("Experiment complete. Thank you!\n\n" +
		"Press any key to exit.")
}
