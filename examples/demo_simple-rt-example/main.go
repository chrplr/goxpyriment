package main

import (
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("AI",
		control.Black, control.White, 32)
	defer exp.End()
	exp.AddDataVariableNames([]string{"key", "keyname", "rt"})
	s1 := stimuli.NewFixCross(20, 2, control.White)
	s2 := stimuli.NewTextLine("GO", 0, 0, control.White)

	exp.Run(func() error {
		for range 10 {
			exp.Keyboard.Clear() // discard any stale events from the previous trial
			onset, _ := exp.ShowTS(s1)
			exp.Wait(1000)
			exp.Show(s2)
			exp.Wait(1000)
			exp.Blank(0)
			key, ts, _ := exp.Keyboard.GetKeyEventTS(nil, -1)
			rt := (ts - onset) / 1_000_000
			exp.Data.Add(key, key.KeyName(), rt)
		}
		return control.EndLoop
	})
}
