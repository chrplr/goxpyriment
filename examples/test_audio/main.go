package main

import (
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/misc"
	"log"
)

func main() {
	exp := control.NewExperiment("Audio Test", 800, 600, false)
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()

	log.Println("Playing Buzzer...")
	if err := exp.PlayBuzzer(); err != nil {
		log.Printf("Error playing buzzer: %v", err)
	}
	misc.Wait(1000)

	log.Println("Playing Correct...")
	if err := exp.PlayCorrect(); err != nil {
		log.Printf("Error playing correct: %v", err)
	}
	misc.Wait(1000)
}
