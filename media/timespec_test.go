// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package media

import (
	"testing"
	"time"
)

func TestParseTimeSpec(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"ms:500", 500 * time.Millisecond},
		{"s:3", 3 * time.Second},
		{"m:1", time.Minute},
		{"h:2", 2 * time.Hour},
		{"s:3,ms:100", 3*time.Second + 100*time.Millisecond},
		{"h:0,m:1,s:30", time.Minute + 30*time.Second},
		{" s:1 , ms:5 ", time.Second + 5*time.Millisecond},
		{"ms:0.5", 500 * time.Microsecond},
	}
	for _, c := range cases {
		got, err := ParseTimeSpec(c.in)
		if err != nil {
			t.Errorf("ParseTimeSpec(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseTimeSpec(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseTimeSpecErrors(t *testing.T) {
	bad := []string{
		"",
		"500",
		"x:1",
		"s:",
		"s:abc",
		"s:1,",
		",s:1",
	}
	for _, in := range bad {
		if _, err := ParseTimeSpec(in); err == nil {
			t.Errorf("ParseTimeSpec(%q): expected error, got nil", in)
		}
	}
}

func TestParseFrameSpec(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"f:1", 1},
		{"f:186", 186},
		{"f:0", 0},
		{"frame:42", 42},
		{" f: 7 ", 7},
	}
	for _, c := range cases {
		got, err := ParseFrameSpec(c.in)
		if err != nil {
			t.Errorf("ParseFrameSpec(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseFrameSpec(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseFrameSpecErrors(t *testing.T) {
	bad := []string{
		"",
		"1",
		"x:1",
		"f:abc",
		"f:-1",
	}
	for _, in := range bad {
		if _, err := ParseFrameSpec(in); err == nil {
			t.Errorf("ParseFrameSpec(%q): expected error, got nil", in)
		}
	}
}
