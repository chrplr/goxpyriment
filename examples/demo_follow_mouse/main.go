// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("Follow Mouse", control.Black, control.White, 32)
	defer exp.End()

	dot := stimuli.NewCircle(10, control.White)

	exp.Run(func() error {
		x, y := exp.Screen.MousePosition()
		dot.SetPosition(sdl.FPoint{X: x, Y: y})
		exp.Show(dot)
		return nil
	})
}
