// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"math/rand/v2"
)

// The arrow schedule is planned before the run, from its duration, so that the
// response side can be exactly balanced. Done online, a 50/50 draw only
// balances on average, and nothing could correct the imbalance because the
// number of arrows is not known until the run ends.

// Sides index the per-side tables.
const (
	left  = 0
	right = 1
)

// scheduleParams are the schedule's timing rules, in frames.
type scheduleParams struct {
	TargetFrames int // duration of an arrow
	GapMinFrames int // gap from the end of one arrow to the next onset
	GapMaxFrames int
	RespFrames   int // response window, from an arrow's onset
}

// arrow is one planned '<' or '>'.
type arrow struct {
	OnsetFrame, Side int
}

// planArrows schedules the arrows of a run of totalFrames frames, in onset
// order. The first arrow starts GapMinFrames into the run; each next one
// starts a random gap of GapMinFrames-GapMaxFrames after the previous one
// ends. An arrow is kept only if its response window closes inside the run,
// so every arrow is fully scored.
//
// The run then gets exactly as many left as right arrows, in random order. An
// odd count loses its last arrow, so the run has one arrow fewer than the
// schedule would allow rather than an unbalanced side.
func planArrows(totalFrames int, p scheduleParams, rng *rand.Rand) ([]arrow, error) {
	if p.GapMaxFrames < p.GapMinFrames || p.GapMinFrames < 1 || p.TargetFrames < 1 ||
		p.RespFrames < 1 || p.RespFrames > p.TargetFrames+p.GapMinFrames {
		return nil, fmt.Errorf("schedule: invalid timing parameters %+v", p)
	}
	draw := func(lo, hi int) int { return lo + rng.IntN(hi-lo+1) }

	var arrows []arrow
	for onset := p.GapMinFrames; onset+p.RespFrames <= totalFrames; onset += p.TargetFrames + draw(p.GapMinFrames, p.GapMaxFrames) {
		arrows = append(arrows, arrow{OnsetFrame: onset})
	}
	if len(arrows)%2 == 1 {
		arrows = arrows[:len(arrows)-1]
	}
	sides := make([]int, len(arrows))
	for i := len(arrows) / 2; i < len(arrows); i++ {
		sides[i] = right
	}
	rng.Shuffle(len(sides), func(i, j int) { sides[i], sides[j] = sides[j], sides[i] })
	for i := range arrows {
		arrows[i].Side = sides[i]
	}
	return arrows, nil
}
