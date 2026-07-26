// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_stream_callback exercises PresentStreamOfStimuliFunc's per-frame
// FrameCallback. It shows an RSVP word stream while a FrameCallback (a) draws a
// persistent overlay label on every frame — on top of each word AND over the
// blank ISI — and (b) runs real-time logic, recording the moment each word's
// first frame is shown and counting total frames.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("Stream Callback Test", control.Black, control.White, 48)
	defer exp.End()

	words := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	stims := make([]stimuli.Stimulus, len(words))
	for i, w := range words {
		stims[i] = stimuli.NewTextLine(w, 0, 0, control.White)
	}
	elements := stimuli.MakeRegularStream(stims, 400*time.Millisecond, 200*time.Millisecond)

	// Persistent overlay drawn every frame by the callback.
	overlay := stimuli.NewTextLine("[ overlay — visible on every frame ]", 0, -300, control.Yellow)
	if err := stimuli.PreloadVisualOnScreen(exp.Screen, overlay); err != nil {
		log.Fatalf("preloading overlay: %v", err)
	}

	var frameCount int
	var onsetMarks []uint64 // NowNS at each word's first on-frame (real-time logic demo)

	cb := func(ctx stimuli.FrameContext) error {
		// (a) persistent overlay: drawn on top of the stimulus, before the flip.
		if err := overlay.Draw(ctx.Screen); err != nil {
			return err
		}
		// (b) real-time logic: react to the onset frame of each element.
		frameCount++
		if ctx.OnPhase && ctx.FirstFrame {
			onsetMarks = append(onsetMarks, ctx.NowNS)
		}
		return nil
	}

	fmt.Printf("Presenting %d words with a per-frame overlay callback...\n", len(words))

	_, logs, err := stimuli.PresentStreamOfStimuliFunc(exp.Screen, elements, 0, 0, cb)
	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("stream failed: %v", err)
	}

	fmt.Printf("\ncallback ran on %d frames; recorded %d word-onset marks\n", frameCount, len(onsetMarks))
	var ref uint64
	if len(logs) > 0 {
		ref = logs[0].OnsetNS // stream start on the SDL clock; OnsetNS is the authoritative onset (see TimingLog)
	}
	for _, tl := range logs {
		fmt.Printf("word %d (%q): onset %dms, OnsetNS %d\n",
			tl.Index, words[tl.Index], int64(tl.OnsetNS-ref)/1_000_000, tl.OnsetNS)
	}
}
