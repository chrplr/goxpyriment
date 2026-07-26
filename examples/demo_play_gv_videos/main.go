// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
package main

import (
	"flag"
	"log"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	gvPath := flag.String("f", "wedges.gv", "Path to .gv video file")
	exp := control.NewExperimentFromFlags("GV Video Test", control.Black, control.White, 32)
	defer exp.End()

	events, logs, err := stimuli.PlayGv(exp.Screen, *gvPath, 0, 0)
	if err != nil {
		log.Fatalf("PlayGv: %v", err)
	}
	totalSkipped := 0
	for _, l := range logs {
		totalSkipped += l.SkippedFrames
	}
	var duration time.Duration
	if len(logs) >= 2 {
		duration = time.Duration(logs[len(logs)-1].FlipNS - logs[0].FlipNS)
	}
	log.Printf("playback complete: %d frames, %d skipped, %d user events, duration %.3f s",
		len(logs), totalSkipped, len(events), duration.Seconds())
}
