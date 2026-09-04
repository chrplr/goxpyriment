package main

import (
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("Hello World", control.Black, control.White, 32)
	defer exp.End()

	hello := stimuli.NewTextBox("Hello, World!", 600, control.Point(0, 0), control.White)
	exp.Show(hello)
	exp.Keyboard.Wait()
}
