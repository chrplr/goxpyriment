// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import "testing"

func TestGridResized(t *testing.T) {
	g := Grid{{1, 2, 3}, {4, 5, 6}}
	got := g.Resized(3, 2)
	want := Grid{{1, 2}, {4, 5}, {0, 0}}
	if !got.Equal(want) {
		t.Errorf("Resized(3,2) = %v, want %v", got, want)
	}
	if !g.Equal(Grid{{1, 2, 3}, {4, 5, 6}}) {
		t.Errorf("Resized modified its receiver: %v", g)
	}
}

func TestGridEqualAndClone(t *testing.T) {
	g := Grid{{1, 0}, {0, 9}}
	c := g.Clone()
	if !g.Equal(c) {
		t.Fatal("clone differs from original")
	}
	c[0][0] = 5
	if g.Equal(c) || g[0][0] != 1 {
		t.Error("Clone shares storage with the original")
	}
	if g.Equal(Grid{{1, 0}}) || g.Equal(Grid{{1, 0, 0}, {0, 9, 0}}) {
		t.Error("Equal ignores size")
	}
}

func TestGridString(t *testing.T) {
	if s := (Grid{{0, 0, 0, 0}, {0, 1, 2, 0}, {0, 0, 0, 0}}).String(); s != "0000|0120|0000" {
		t.Errorf("String() = %q", s)
	}
	if s := Blank(2, 3).String(); s != "000|000" {
		t.Errorf("Blank(2,3).String() = %q", s)
	}
}

func TestGridValid(t *testing.T) {
	cases := []struct {
		g    Grid
		want bool
	}{
		{Grid{{0}}, true},
		{Grid{}, false},
		{Grid{{}}, false},
		{Grid{{1, 2}, {3}}, false},
		{Grid{{10}}, false},
		{Grid{{-1}}, false},
		{Blank(MaxGridSide, MaxGridSide), true},
		{Blank(MaxGridSide+1, 1), false},
	}
	for _, c := range cases {
		if got := c.g.valid(); got != c.want {
			t.Errorf("%v.valid() = %v, want %v", c.g, got, c.want)
		}
	}
}
