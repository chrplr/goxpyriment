// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

import "testing"

// The grid arithmetic is the part of sprite-sheet support that can be wrong
// silently: a transposed grid or an off-by-one cell size still renders, just
// with the wrong pixels. It needs no renderer, so it is tested directly.

func TestGridClipsTilesTheSheet(t *testing.T) {
	clips, err := gridClips(512, 256, 4, 2, 0, 0)
	if err != nil {
		t.Fatalf("gridClips: %v", err)
	}
	if len(clips) != 8 {
		t.Fatalf("got %d clips, want 8", len(clips))
	}
	// Row-major: index 1 is the second column of the first row, index 4 the
	// first column of the second row.
	if clips[0].X != 0 || clips[0].Y != 0 {
		t.Errorf("clip 0 at (%g,%g), want (0,0)", clips[0].X, clips[0].Y)
	}
	if clips[1].X != 128 || clips[1].Y != 0 {
		t.Errorf("clip 1 at (%g,%g), want (128,0)", clips[1].X, clips[1].Y)
	}
	if clips[4].X != 0 || clips[4].Y != 128 {
		t.Errorf("clip 4 at (%g,%g), want (0,128)", clips[4].X, clips[4].Y)
	}
	for i, c := range clips {
		if c.W != 128 || c.H != 128 {
			t.Errorf("clip %d is %gx%g, want 128x128", i, c.W, c.H)
		}
	}
	// The cells must cover the sheet exactly, with no strip left over.
	last := clips[len(clips)-1]
	if got := last.X + last.W; got != 512 {
		t.Errorf("last clip ends at x=%g, want 512", got)
	}
	if got := last.Y + last.H; got != 256 {
		t.Errorf("last clip ends at y=%g, want 256", got)
	}
}

func TestGridClipsHonoursMarginAndSpacing(t *testing.T) {
	// 4 cells of 100 px, 10 px gutters, 5 px border: 5+100+10+100+10+100+10+100+5
	clips, err := gridClips(440, 110, 4, 1, 5, 10)
	if err != nil {
		t.Fatalf("gridClips: %v", err)
	}
	for i, c := range clips {
		if c.W != 100 {
			t.Fatalf("clip %d is %g wide, want 100", i, c.W)
		}
		if want := float32(5 + i*110); c.X != want {
			t.Errorf("clip %d at x=%g, want %g", i, c.X, want)
		}
	}
	if got := clips[3].X + clips[3].W; got != 435 {
		t.Errorf("last clip ends at x=%g, want 435 (5 px border left over)", got)
	}
}

func TestGridClipsTransposeIsNotSilent(t *testing.T) {
	// A 4x2 grid and a 2x4 grid both yield 8 cells, so the count cannot catch
	// a transposition; the cell shape must.
	wide, err := gridClips(512, 256, 4, 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	tall, err := gridClips(512, 256, 2, 4, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(wide) != len(tall) {
		t.Fatalf("expected both to yield 8 cells, got %d and %d", len(wide), len(tall))
	}
	if wide[0].W == tall[0].W && wide[0].H == tall[0].H {
		t.Errorf("transposed grids produced identical cells %gx%g", wide[0].W, wide[0].H)
	}
}

func TestGridClipsRejectsImpossibleLayouts(t *testing.T) {
	cases := []struct {
		name            string
		w, h            float32
		cols, rows      int
		margin, spacing float32
	}{
		{"zero columns", 512, 256, 0, 2, 0, 0},
		{"negative rows", 512, 256, 4, -1, 0, 0},
		{"negative margin", 512, 256, 4, 2, -1, 0},
		{"margin swallows the sheet", 100, 100, 4, 2, 60, 0},
		{"spacing swallows the sheet", 100, 100, 40, 2, 0, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := gridClips(c.w, c.h, c.cols, c.rows, c.margin, c.spacing); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}
