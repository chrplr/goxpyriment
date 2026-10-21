// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"math/rand/v2"
	"testing"
)

// testParams are the defaults at 60 Hz.
var testParams = scheduleParams{
	TargetFrames: 48, GapMinFrames: 180, GapMaxFrames: 480, RespFrames: 120,
}

// Every run, of any length, has exactly as many left as right arrows, spaced
// by the gap rules, with every response window inside the run.
func TestPlanArrowsBalancedAndSpaced(t *testing.T) {
	for seed := uint64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewPCG(seed, 7))
		total := 600 + rng.IntN(30000)
		arrows, err := planArrows(total, testParams, rng)
		if err != nil {
			t.Fatal(err)
		}
		var count [2]int
		for i, a := range arrows {
			count[a.Side]++
			if i == 0 {
				if a.OnsetFrame != testParams.GapMinFrames {
					t.Fatalf("seed %d: first onset at frame %d", seed, a.OnsetFrame)
				}
				continue
			}
			d := a.OnsetFrame - arrows[i-1].OnsetFrame
			if d < testParams.TargetFrames+testParams.GapMinFrames || d > testParams.TargetFrames+testParams.GapMaxFrames {
				t.Fatalf("seed %d: onsets %d frames apart", seed, d)
			}
		}
		if count[left] != count[right] {
			t.Fatalf("seed %d: %d left, %d right", seed, count[left], count[right])
		}
		if n := len(arrows); n > 0 && arrows[n-1].OnsetFrame+testParams.RespFrames > total {
			t.Fatalf("seed %d: last response window ends after the run", seed)
		}
	}
}

// A default 300 s run gets about the number of arrows the timing rules imply:
// one every 0.8 + 5.5 s on average.
func TestPlanArrowsRate(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 1))
	arrows, err := planArrows(300*60, testParams, rng)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(arrows); n < 40 || n > 56 {
		t.Fatalf("unexpected arrow count %d", n)
	}
}
