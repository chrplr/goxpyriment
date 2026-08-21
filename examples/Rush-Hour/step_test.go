// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import "testing"

// CanStep is the mask the agent environment publishes, so it has to agree with
// what actually happens on the board — everywhere, not just on the easy cases.
// The reference is Step itself, applied to a clone.
func TestCanStepAgreesWithStep(t *testing.T) {
	puzzles, err := DefaultPuzzles()
	if err != nil {
		t.Fatalf("DefaultPuzzles: %v", err)
	}

	for _, p := range puzzles {
		b := p.Fresh()
		// Walk a deterministic pseudo-random path so the check sees positions
		// other than the starting one, where vehicles cluster at the walls.
		for iter := 0; iter < 60; iter++ {
			for _, car := range b.Cars {
				for _, dir := range []int{Left, Right} {
					probe := b.Clone()
					want := probe.Step(probe.Cars[car.ID], dir)
					if got := b.CanStep(car, dir); got != want {
						t.Fatalf("%s iter %d: CanStep(%s, %+d) = %v, Step reports %v\n%s",
							p.Name, iter, string(car.Label), dir, got, want, b.String())
					}
				}
			}
			if b.Solved() {
				break
			}
			// Advance: first legal step of the (iter mod n)-th car.
			car := b.Cars[iter%len(b.Cars)]
			if !b.Step(car, Right) {
				b.Step(car, Left)
			}
		}
	}
}

// A non-direction never steps, whatever the board looks like.
func TestCanStepRejectsNonDirections(t *testing.T) {
	b, err := ParseBoard(classic)
	if err != nil {
		t.Fatalf("ParseBoard: %v", err)
	}
	for _, dir := range []int{0, 2, -2} {
		if b.CanStep(b.Target(), dir) {
			t.Errorf("CanStep with dir %d reported legal", dir)
		}
	}
}

// Step must be all-or-nothing, unlike TryMove which parks against a blocker.
func TestStepIsOneCellOrNothing(t *testing.T) {
	b, err := ParseBoard(classic)
	if err != nil {
		t.Fatalf("ParseBoard: %v", err)
	}
	target := b.Target() // row 2, cols 1-2; E sits at (2,3)

	if !b.Step(target, Left) || target.Col != 0 {
		t.Fatalf("target at col %d after a step left, want 0", target.Col)
	}
	if b.Step(target, Left) {
		t.Error("stepping into the left wall should report false")
	}
	if target.Col != 0 {
		t.Errorf("a refused step moved the vehicle to col %d", target.Col)
	}
}

// Clone must not share vehicles with the original — Board holds pointers.
func TestCloneIsDeep(t *testing.T) {
	b, err := ParseBoard(classic)
	if err != nil {
		t.Fatalf("ParseBoard: %v", err)
	}
	c := b.Clone()
	c.Step(c.Target(), Left)
	if b.Target().Col != 1 {
		t.Errorf("moving the clone moved the original to col %d", b.Target().Col)
	}
}

// The slide rule the experiment logs and the environment reports: consecutive
// steps of the same vehicle in the same direction are one slide.
func TestSlideCounter(t *testing.T) {
	cases := []struct {
		name  string
		steps [][2]int // {carID, dir}
		want  int
	}{
		{"nothing", nil, 0},
		{"one step", [][2]int{{0, Right}}, 1},
		{"three steps of one car one way", [][2]int{{0, Right}, {0, Right}, {0, Right}}, 1},
		{"same car, reversed", [][2]int{{0, Right}, {0, Left}}, 2},
		{"alternating cars", [][2]int{{0, Right}, {1, Right}, {0, Right}}, 3},
		// Car 0 is a real ID, so the zero value must not swallow its first step.
		{"car zero first", [][2]int{{0, Left}, {0, Left}}, 1},
	}
	for _, c := range cases {
		var s SlideCounter
		for _, st := range c.steps {
			s.Add(st[0], st[1])
		}
		if got := s.N(); got != c.want {
			t.Errorf("%s: N() = %d, want %d", c.name, got, c.want)
		}
	}
}
