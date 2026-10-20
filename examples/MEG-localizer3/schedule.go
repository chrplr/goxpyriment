// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"math/rand/v2"
)

// The task-event schedule is planned before the run, from the trial durations,
// so that the response side can be exactly balanced within each modality. Done
// online, a 50/50 draw only balances on average, and nothing could correct the
// imbalance because the number of events is not known until the run ends.

// Modalities and sides index the per-event tables.
const (
	visual = 0
	audio  = 1

	left  = 0
	right = 1
)

// scheduleParams are the schedule's timing rules, in frames.
type scheduleParams struct {
	TargetFrames   int // duration of a colour change
	GapMinFrames   int // gap from the end of one colour change to the next onset
	GapMaxFrames   int
	SoundMinFrames int // gap between successive sound onsets
	SoundMaxFrames int
	MinSepFrames   int // minimum gap between the onsets of any two events
}

// plannedEvent is a task event assigned to a trial.
type plannedEvent struct {
	Modality, Side int
}

// planEvents assigns task events to trials. trialFrames lists the duration of
// every trial of the run, habituation included, in presentation order; the
// result has the same length, nil where no event starts.
//
// Events start only on a trial onset. A kind of event falls due once the run
// reaches its next scheduled frame, and starts at the first trial onset at
// least MinSepFrames after the previous event of either kind; sounds take
// priority, being rarer. The visual gap is counted from the end of the colour
// change, the sound gap from onset to onset.
//
// Each modality then gets exactly as many left as right events, in random
// order. A modality with an odd count loses its last event, so the run has one
// event fewer than the schedule would allow rather than an unbalanced side.
func planEvents(trialFrames []int, p scheduleParams, rng *rand.Rand) ([]*plannedEvent, error) {
	if p.GapMaxFrames < p.GapMinFrames || p.SoundMaxFrames < p.SoundMinFrames ||
		p.GapMinFrames < 1 || p.SoundMinFrames < 1 || p.TargetFrames < 1 {
		return nil, fmt.Errorf("schedule: invalid timing parameters %+v", p)
	}
	draw := func(lo, hi int) int { return lo + rng.IntN(hi-lo+1) }

	plan := make([]*plannedEvent, len(trialFrames))
	var byModality [2][]int // trial indices
	nextVisual := p.GapMinFrames
	nextSound := draw(p.SoundMinFrames, p.SoundMaxFrames)
	lastEvent := -p.MinSepFrames
	frame := 0
	for t, n := range trialFrames {
		if frame-lastEvent >= p.MinSepFrames {
			m := -1
			switch {
			case frame >= nextSound:
				m = audio
				nextSound = frame + draw(p.SoundMinFrames, p.SoundMaxFrames)
			case frame >= nextVisual:
				m = visual
				nextVisual = frame + p.TargetFrames + draw(p.GapMinFrames, p.GapMaxFrames)
			}
			if m >= 0 {
				plan[t] = &plannedEvent{Modality: m}
				byModality[m] = append(byModality[m], t)
				lastEvent = frame
			}
		}
		frame += n
	}

	for _, idx := range byModality {
		if len(idx)%2 == 1 {
			plan[idx[len(idx)-1]] = nil
			idx = idx[:len(idx)-1]
		}
		sides := make([]int, len(idx))
		for i := len(idx) / 2; i < len(idx); i++ {
			sides[i] = right
		}
		rng.Shuffle(len(sides), func(i, j int) { sides[i], sides[j] = sides[j], sides[i] })
		for i, t := range idx {
			plan[t].Side = sides[i]
		}
	}
	return plan, nil
}
