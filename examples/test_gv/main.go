package main

import (
	"flag"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed)")
	gvPath := flag.String("f", "test.gv", "Path to .gv video file")
	flag.Parse()

	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1280, 720, false
	}

	exp := control.NewExperiment("GV Video Test", width, height, fullscreen, control.Black, control.White, 32)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	events, err := stimuli.PlayGv(exp.Screen, *gvPath, 0, 0)
	if err != nil {
		log.Fatalf("PlayGv: %v", err)
	}
	log.Printf("playback complete, %d user events recorded", len(events))
}
