// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"log"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperiment("Audio Test", 800, 600, false, control.Black, control.White, 32)
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()

	log.Println("Playing Buzzer...")
	if err := stimuli.PlayBuzzer(exp.AudioDevice); err != nil {
		log.Printf("Error playing buzzer: %v", err)
	}
	clock.Wait(1000)

	log.Println("Playing Ping...")
	if err := stimuli.PlayPing(exp.AudioDevice); err != nil {
		log.Printf("Error playing ping: %v", err)
	}
	clock.Wait(1000)
}
