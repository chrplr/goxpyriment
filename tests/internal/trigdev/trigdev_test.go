// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package trigdev

import (
	"strings"
	"testing"
)

// ParseSpec is the whole command-line contract of this package, and it runs
// before any hardware is touched — so it is the one part that can be checked
// without a bench.

func TestParseSpecDefaultsToPinOne(t *testing.T) {
	for _, kind := range []string{"dlpio8", "dlpio20", "parallel", "gpio", "ft232h"} {
		spec, err := ParseSpec(kind)
		if err != nil {
			t.Fatalf("ParseSpec(%q): %v", kind, err)
		}
		if spec.Kind != kind {
			t.Errorf("ParseSpec(%q).Kind = %q", kind, spec.Kind)
		}
		if spec.Pin != 1 || spec.Line() != 0 {
			t.Errorf("ParseSpec(%q): pin %d line %d, want pin 1 line 0", kind, spec.Pin, spec.Line())
		}
	}
}

func TestParseSpecReadsParameters(t *testing.T) {
	spec, err := ParseSpec("dlpio8:port=/dev/ttyUSB0,pin=3")
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if spec.Params["port"] != "/dev/ttyUSB0" {
		t.Errorf("port = %q, want /dev/ttyUSB0", spec.Params["port"])
	}
	// The device is labelled 1-8; the triggers API takes 0-7. Getting this
	// wrong drives the neighbouring line, which looks like a miswiring.
	if spec.Pin != 3 || spec.Line() != 2 {
		t.Errorf("pin %d line %d, want pin 3 line 2", spec.Pin, spec.Line())
	}
}

func TestParseSpecGPIOPinListUsesPlus(t *testing.T) {
	// ',' separates parameters, so the pin list cannot use it.
	spec, err := ParseSpec("gpio:chip=/dev/gpiochip4,pins=17+27+22+5+6+13+19+26,pin=2")
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	pins, err := ParsePins(spec.Params["pins"])
	if err != nil {
		t.Fatalf("ParsePins: %v", err)
	}
	if pins[spec.Line()] != 27 {
		t.Errorf("pin=2 selects chip line %d, want 27", pins[spec.Line()])
	}
	if spec.Params["chip"] != "/dev/gpiochip4" {
		t.Errorf("chip = %q", spec.Params["chip"])
	}
}

func TestParseSpecRejects(t *testing.T) {
	cases := []struct {
		name, spec, want string
	}{
		{"unknown kind", "dlpio9:pin=1", "unknown device kind"},
		{"empty", "  ", "empty device spec"},
		{"pin zero", "parallel:pin=0", "out of range"},
		{"pin nine", "parallel:pin=9", "out of range"},
		{"pin not a number", "parallel:pin=D0", "not a number"},
		// A typo must not be silently ignored: it would leave the probe on a
		// line the program never drives.
		{"unknown key", "dlpio8:pins=17+27+22+5+6+13+19+26", `takes no "pins"`},
		{"not key=value", "dlpio8:/dev/ttyUSB0", "is not key=value"},
		{"repeated key", "dlpio8:pin=1,pin=2", "given twice"},
		// Nothing to auto-detect for these two.
		{"megttlbox needs port", "megttlbox:pin=1", "needs port="},
		{"labjackt4 needs host", "labjackt4:pin=1", "needs host="},
		{"gpio short pin list", "gpio:pins=17+27", "8 pin numbers"},
		{"gpio duplicate pin", "gpio:pins=17+17+22+5+6+13+19+26", "repeats line 17"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseSpec(c.spec)
			if err == nil {
				t.Fatalf("ParseSpec(%q) succeeded; want an error mentioning %q", c.spec, c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("ParseSpec(%q) = %q, want it to mention %q", c.spec, err, c.want)
			}
		})
	}
}

func TestParsePinsRejectsCommaSeparatedList(t *testing.T) {
	// Timing-Tests' -gpio-pins is comma-separated; this one is not. Pasting
	// one into the other must fail loudly rather than parse as a single pin.
	if _, err := ParsePins("17,27,22,5,6,13,19,26"); err == nil {
		t.Fatal("ParsePins accepted a comma-separated list; want an error naming '+'")
	}
}

func TestFlagsAcceptsTwoLinesOfOneDeviceButNotADuplicate(t *testing.T) {
	var f Flags
	if err := f.Set("parallel:pin=1"); err != nil {
		t.Fatalf("first device: %v", err)
	}
	// Two lines of one box is a legitimate measurement: it gives the skew
	// between two lines of the same device.
	if err := f.Set("parallel:pin=2"); err != nil {
		t.Fatalf("second line of the same device: %v", err)
	}
	if err := f.Set("parallel:pin=1"); err == nil {
		t.Fatal("a byte-identical device spec was accepted twice; want an error")
	}
	if len(f) != 2 {
		t.Errorf("collected %d specs, want 2", len(f))
	}
}

func TestFlagsStringRoundTripsTheRawSpecs(t *testing.T) {
	var f Flags
	for _, s := range []string{"parallel:pin=1", "dlpio8:port=/dev/ttyUSB0,pin=2"} {
		if err := f.Set(s); err != nil {
			t.Fatalf("Set(%q): %v", s, err)
		}
	}
	got := f.String()
	for _, s := range []string{"parallel:pin=1", "dlpio8:port=/dev/ttyUSB0,pin=2"} {
		if !strings.Contains(got, s) {
			t.Errorf("Flags.String() = %q, missing %q", got, s)
		}
	}
}

func TestT4TerminalNameFollowsTheHardwareLabels(t *testing.T) {
	// The DIO numbering is contiguous, the terminals are not: DIO7 is the last
	// screw terminal, DIO8 is the first pin of the DB15.
	for _, c := range []struct {
		dio  int
		want string
	}{
		{4, "FIO4"}, {7, "FIO7"}, {8, "EIO0"}, {11, "EIO3"}, {16, "CIO0"},
	} {
		if got := T4TerminalName(c.dio); got != c.want {
			t.Errorf("T4TerminalName(%d) = %q, want %q", c.dio, got, c.want)
		}
	}
}
