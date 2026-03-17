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
	gvPath := flag.String("f", "wedges.gv", "Path to .gv video file")
	exp := control.NewExperimentFromFlags("GV Video Test", control.Black, control.White, 32)
	defer exp.End()

	events, err := stimuli.PlayGv(exp.Screen, *gvPath, 0, 0)
	if err != nil {
		log.Fatalf("PlayGv: %v", err)
	}
	log.Printf("playback complete, %d user events recorded", len(events))
}
