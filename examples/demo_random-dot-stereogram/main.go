// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("Random Dot Stereogram", control.Gray, control.White, 32)
	defer exp.End()

	// Create RDS stimulus
	// Python defaults: imgsize=(80, 80), inner_size=(30, 30), shift=6, gap=10
	// We'll scale it up for better visibility
	rds := stimuli.NewRDS([2]int{80, 80}, [2]int{30, 30}, 6, 10, 4)

	err := exp.Run(func() error {
		if _, _, err := exp.HandleEvents(); err != nil {
			return err
		}

		if err := exp.Screen.Clear(); err != nil {
			return err
		}

		if err := rds.Draw(exp.Screen); err != nil {
			return err
		}

		return exp.Screen.Update()
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
