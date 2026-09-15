// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"math"
	"testing"

	"github.com/chrplr/goxpyriment/control"
)

func dist(a, b control.FPoint) float64 {
	return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y))
}

// The transcription of Table S1 is checked against the table's own derived
// column: every shape has a mean pairwise vertex distance of 1.434.
func TestReferenceShapesAreSizeMatched(t *testing.T) {
	for _, s := range referenceShapes {
		f, err := referenceFigure(s.name)
		if err != nil {
			t.Fatal(err)
		}
		p := f.parts[0].points
		var sum float64
		for i := 0; i < 4; i++ {
			for j := i + 1; j < 4; j++ {
				sum += dist(p[i], p[j])
			}
		}
		if avg := sum / 6; math.Abs(avg-avgPairwiseDistance) > 0.002 {
			t.Errorf("%s: mean pairwise distance %.4f, want %.3f", s.name, avg, avgPairwiseDistance)
		}
	}
}

// Every deviant moves only BR, by exactly d, and stays convex (the renderer
// fills a triangle fan, which is only correct for convex polygons).
func TestDeviants(t *testing.T) {
	d := deviantFraction * avgPairwiseDistance
	for _, s := range referenceShapes {
		ref, _ := referenceFigure(s.name)
		rp := ref.parts[0].points
		if !isConvex(rp) {
			t.Errorf("%s: reference is not convex", s.name)
		}
		for _, name := range deviantNames {
			dev, err := deviantFigure(s.name, name)
			if err != nil {
				t.Fatal(err)
			}
			dp := dev.parts[0].points
			for i := 0; i < 3; i++ {
				if dp[i] != rp[i] {
					t.Errorf("%s/%s: vertex %d moved", s.name, name, i)
				}
			}
			if got := dist(dp[3], rp[3]); math.Abs(got-d) > 1e-3 {
				t.Errorf("%s/%s: BR moved by %.4f, want %.4f", s.name, name, got, d)
			}
			if name == "rot_up" || name == "rot_down" {
				if got := dist(dp[3], dp[0]); math.Abs(got-float64(rp[3].X)) > 1e-3 {
					t.Errorf("%s/%s: bottom edge length %.4f changed from %.4f", s.name, name, got, rp[3].X)
				}
			}
			if !isConvex(dp) {
				t.Errorf("%s/%s: deviant is not convex", s.name, name)
			}
		}
	}
}

// Training figures must be made of convex parts only.
func TestTrainingFiguresAreConvex(t *testing.T) {
	for _, pairs := range [][]pair{trainingPicturePairs(), trainingPolygonPairs()} {
		for _, p := range pairs {
			for _, f := range []figure{p.a, p.b} {
				for i, pt := range f.parts {
					if len(pt.points) < 3 || !isConvex(pt.points) {
						t.Errorf("%s: part %d is not a convex polygon", f.name, i)
					}
				}
			}
		}
	}
}

func TestTransformCentresAndRotates(t *testing.T) {
	f, _ := referenceFigure("rectangle")
	parts := f.transform(100, 1, 90)
	c := polygonCentroid(parts[0].points)
	if math.Hypot(float64(c.X), float64(c.Y)) > 1e-3 {
		t.Errorf("centroid after transform = %v, want origin", c)
	}
	// Rotating the 150 × 100 rectangle by 90° makes it 100 wide and 150 tall.
	minX, maxX := parts[0].points[0].X, parts[0].points[0].X
	for _, p := range parts[0].points {
		minX, maxX = min(minX, p.X), max(maxX, p.X)
	}
	if math.Abs(float64(maxX-minX)-100) > 1e-2 {
		t.Errorf("width after 90° rotation = %.2f, want 100", maxX-minX)
	}
	if !pointInPolygon(0, 0, parts[0].points) || pointInPolygon(60, 0, parts[0].points) {
		t.Error("pointInPolygon disagrees with the rotated rectangle")
	}
}

func TestTrialCounts(t *testing.T) {
	for _, tc := range []struct {
		exp, n  int
		swapped bool
	}{{1, 44, false}, {2, 88, true}} {
		trials, err := buildTestTrials(tc.exp)
		if err != nil {
			t.Fatal(err)
		}
		if len(trials) != tc.n {
			t.Errorf("exp %d: %d trials, want %d", tc.exp, len(trials), tc.n)
		}
		slotCount := map[int]int{}
		hasSwapped := false
		for _, tr := range trials {
			slotCount[tr.outlierSlot]++
			hasSwapped = hasSwapped || tr.presentation == "swapped"
		}
		if hasSwapped != tc.swapped {
			t.Errorf("exp %d: swapped trials present = %v, want %v", tc.exp, hasSwapped, tc.swapped)
		}
		for s := 0; s < nSlots; s++ {
			if c := slotCount[s]; c < tc.n/nSlots || c > tc.n/nSlots+1 {
				t.Errorf("exp %d: slot %d is the outlier %d times, want %d or %d", tc.exp, s, c, tc.n/nSlots, tc.n/nSlots+1)
			}
		}
	}
	if n := len(buildTrainingExp1()); n != 2 {
		t.Errorf("exp 1 training: %d trials, want 2", n)
	}
}
