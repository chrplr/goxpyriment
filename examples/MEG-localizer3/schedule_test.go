// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"math/rand/v2"
	"testing"
)

// testParams are the defaults at 60 Hz.
var testParams = scheduleParams{
	TargetFrames: 48, GapMinFrames: 180, GapMaxFrames: 480,
	SoundMinFrames: 480, SoundMaxFrames: 720, MinSepFrames: 120,
}

func randomTrials(rng *rand.Rand, n int) []int {
	frames := make([]int, n)
	for i := range frames {
		frames[i] = 13 + rng.IntN(7) // 217-317 ms at 60 Hz
	}
	return frames
}

// Every run, of any length, has exactly as many left as right events in each
// modality, and no two events closer than the minimum separation.
func TestPlanEventsBalancedAndSeparated(t *testing.T) {
	for seed := uint64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewPCG(seed, 7))
		frames := randomTrials(rng, 50+rng.IntN(500))
		plan, err := planEvents(frames, testParams, rng)
		if err != nil {
			t.Fatal(err)
		}
		var count [2][2]int
		frame, last := 0, -1
		for i, ev := range plan {
			if ev != nil {
				count[ev.Modality][ev.Side]++
				if last >= 0 && frame-last < testParams.MinSepFrames {
					t.Fatalf("seed %d: events %d frames apart, minimum %d",
						seed, frame-last, testParams.MinSepFrames)
				}
				last = frame
			}
			frame += frames[i]
		}
		for m := range count {
			if count[m][left] != count[m][right] {
				t.Fatalf("seed %d: modality %d has %d left, %d right",
					seed, m, count[m][left], count[m][right])
			}
		}
	}
}

// A full default run (20 habituation + 467 trials, ~2.2 min) gets about the
// number of events the timing rules imply, so balancing drops at most one per
// modality rather than distorting the schedule.
func TestPlanEventsRate(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 1))
	frames := randomTrials(rng, 487)
	plan, err := planEvents(frames, testParams, rng)
	if err != nil {
		t.Fatal(err)
	}
	var n [2]int
	for _, ev := range plan {
		if ev != nil {
			n[ev.Modality]++
		}
	}
	// ~130 s: sounds every ~10 s, colour changes every ~6.3 s minus collisions.
	if n[audio] < 10 || n[audio] > 16 || n[visual] < 12 || n[visual] > 24 {
		t.Fatalf("unexpected event counts: visual %d, audio %d", n[visual], n[audio])
	}
}
