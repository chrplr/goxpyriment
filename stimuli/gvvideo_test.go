// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

import "testing"

func TestFramesPerVideoFrameAcceptsIntegerRatios(t *testing.T) {
	tests := []struct {
		name     string
		refresh  float64
		fps      float64
		wantHold int
	}{
		{"30 fps on 60 Hz", 60, 30, 2},
		{"60 fps on 60 Hz", 60, 60, 1},
		{"20 fps on 60 Hz", 60, 20, 3},
		{"15 fps on 60 Hz", 60, 15, 4},
		{"30 fps on 120 Hz", 120, 30, 4},
		{"60 fps on 120 Hz", 120, 60, 2},
		{"30 fps on 144 Hz", 144, 30, 0}, // 4.8 — rejected, see below
		{"24 fps on 144 Hz", 144, 24, 6},
		{"1 fps on 60 Hz", 60, 1, 60},

		// Real hardware and real files rarely report round numbers: a display
		// advertising 59.94 Hz and a clip authored at 29.97 fps must behave
		// exactly like 60 and 30.
		{"59.94 Hz with 29.97 fps", 59.94, 29.97, 2},
		{"59.94 Hz with 59.94 fps", 59.94, 59.94, 1},
		{"60.0001 Hz with 30 fps", 60.0001, 30, 2},
	}

	for _, tt := range tests {
		got, err := framesPerVideoFrame(tt.refresh, tt.fps)
		if tt.wantHold == 0 {
			if err == nil {
				t.Errorf("%s: expected an error, got hold=%d", tt.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.name, err)
			continue
		}
		if got != tt.wantHold {
			t.Errorf("%s: hold = %d, want %d", tt.name, got, tt.wantHold)
		}
	}
}

func TestFramesPerVideoFrameRejectsUnplayableRates(t *testing.T) {
	tests := []struct {
		name    string
		refresh float64
		fps     float64
	}{
		// Playing these anyway would need pulldown, making frame onsets
		// uneven — the opposite of what .gv is for.
		{"24 fps on 60 Hz needs 2.5", 60, 24},
		{"25 fps on 60 Hz needs 2.4", 60, 25},
		{"50 fps on 60 Hz needs 1.2", 60, 50},

		// The display simply cannot keep up.
		{"120 fps on 60 Hz", 60, 120},
		{"60 fps on 30 Hz", 30, 60},

		// Malformed input.
		{"zero fps", 60, 0},
		{"negative fps", 60, -30},
		{"fps rounds to zero", 60, 0.4},
		{"zero refresh", 0, 30},
	}

	for _, tt := range tests {
		if got, err := framesPerVideoFrame(tt.refresh, tt.fps); err == nil {
			t.Errorf("%s: expected an error, got hold=%d", tt.name, got)
		}
	}
}

// The rejection message suggests a rate that actually divides the refresh, so
// the advice is followable rather than merely discouraging.
func TestFramesPerVideoFrameSuggestsAWorkableRate(t *testing.T) {
	tests := []struct {
		refresh, want int
		fps           float64
	}{
		{refresh: 60, fps: 24, want: 20}, // largest divisor of 60 that is <= 24
		{refresh: 60, fps: 25, want: 20},
		{refresh: 60, fps: 50, want: 30},
		{refresh: 144, fps: 30, want: 24},
	}

	for _, tt := range tests {
		got := largestDivisorAtMost(tt.refresh, int(tt.fps))
		if got != tt.want {
			t.Errorf("largestDivisorAtMost(%d, %g) = %d, want %d", tt.refresh, tt.fps, got, tt.want)
		}
		if tt.refresh%got != 0 {
			t.Errorf("suggested %d does not divide %d", got, tt.refresh)
		}
		if _, err := framesPerVideoFrame(float64(tt.refresh), float64(got)); err != nil {
			t.Errorf("suggested rate %d is itself rejected on %d Hz: %v", got, tt.refresh, err)
		}
	}
}
