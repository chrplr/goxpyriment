// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import "testing"

func TestParseResolution(t *testing.T) {
	good := []struct {
		in   string
		w, h int
	}{
		{"", 0, 0},
		{"  ", 0, 0},
		{"1920x1080", 1920, 1080},
		{"1920X1080", 1920, 1080},
		{"1920×1080", 1920, 1080},
		{" 1024 x 768 ", 1024, 768},
	}
	for _, c := range good {
		w, h, err := ParseResolution(c.in)
		if err != nil || w != c.w || h != c.h {
			t.Errorf("ParseResolution(%q) = %d, %d, %v; want %d, %d, nil", c.in, w, h, err, c.w, c.h)
		}
	}
	// A typo must not become "no resolution": that would silently open at the
	// current mode.
	for _, in := range []string{"1920", "1920x", "x1080", "1920x1080x60", "0x1080", "1920x0", "-1920x1080", "1920,1080", "fullhd", "1920.5x1080"} {
		if w, h, err := ParseResolution(in); err == nil {
			t.Errorf("ParseResolution(%q) = %d, %d, nil; want an error", in, w, h)
		}
	}
}
