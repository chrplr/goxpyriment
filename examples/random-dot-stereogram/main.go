// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 800x600)")
	flag.Parse()

	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 800, 600, false
	}

	exp := control.NewExperiment("Random Dot Stereogram", width, height, fullscreen, control.Gray, control.White, 32)

	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
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

	if err != nil && err != control.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
