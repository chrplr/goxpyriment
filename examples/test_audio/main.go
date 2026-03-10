package main

import (
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/misc"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperiment("Audio Test", 800, 600, false)
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()

	log.Println("Playing Buzzer...")
	if err := stimuli.PlayBuzzer(exp.AudioDevice); err != nil {
		log.Printf("Error playing buzzer: %v", err)
	}
	misc.Wait(1000)

	log.Println("Playing Ping...")
	if err := stimuli.PlayPing(exp.AudioDevice); err != nil {
		log.Printf("Error playing ping: %v", err)
	}
	misc.Wait(1000)
}
