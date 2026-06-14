// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"fmt"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("TextInput Demo", control.Black, control.White, 32)
	defer exp.End()

	// 2. Prepare TextInput
	ti := stimuli.NewTextInput("Please enter your name:",
		control.FPoint{X: 100, Y: 10},
		400,
		control.Color{R: 50, G: 50, B: 50, A: 255},
		control.Color{R: 200, G: 200, B: 200, A: 255},
		control.White)

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		name, err := ti.Get(exp.Screen, exp.Keyboard)
		if err != nil {
			return err
		}

		fmt.Printf("User entered: %s\n", name)

		// Show result
		result := fmt.Sprintf("Hello, %s!\n\nPress any key to exit.", name)
		msg := stimuli.NewTextBox(result, 600, control.FPoint{X: 0, Y: 0}, control.White)

		if err := exp.Show(msg); err != nil {
			return err
		}

		if _, err := exp.Keyboard.Wait(); err != nil {
			return err
		}

		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
