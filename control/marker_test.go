// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"flag"
	"fmt"
	"testing"
	"time"
)

// withFreshFlags runs f against an empty command line and no registered
// marker flags, restoring both afterwards.
func withFreshFlags(t *testing.T, f func()) {
	t.Helper()
	oldCL, oldVals, oldFatal := flag.CommandLine, markerFlagValues, markerFatal
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	markerFlagValues = nil
	defer func() { flag.CommandLine, markerFlagValues, markerFatal = oldCL, oldVals, oldFatal }()
	f()
}

func TestMarkerFlagsOffByDefault(t *testing.T) {
	withFreshFlags(t, func() {
		if _, ok := markerConfigFromFlags(); ok {
			t.Fatal("marker config reported without MarkerFlags being called")
		}
		MarkerFlags()
		if err := flag.CommandLine.Parse(nil); err != nil {
			t.Fatal(err)
		}
		c, ok := markerConfigFromFlags()
		if !ok {
			t.Fatal("no marker config after MarkerFlags")
		}
		if c.enabled() {
			t.Errorf("config %+v enabled with no flags given", c)
		}
		if c.PhotodiodeSize != DefaultPhotodiodeSize || c.Frames != DefaultMarkerFrames {
			t.Errorf("defaults = size %v frames %d", c.PhotodiodeSize, c.Frames)
		}
	})
}

func TestMarkerFlagsParse(t *testing.T) {
	withFreshFlags(t, func() {
		MarkerFlags()
		args := []string{"-photodiode", "-photodiode-size", "80", "-marker-frames", "2",
			"-ttl", "megttlbox:/dev/ttyACM0", "-ttl-ms", "10"}
		if err := flag.CommandLine.Parse(args); err != nil {
			t.Fatal(err)
		}
		c, _ := markerConfigFromFlags()
		want := MarkerConfig{Photodiode: true, PhotodiodeSize: 80, Frames: 2,
			TTL: "megttlbox:/dev/ttyACM0", TTLDuration: 10 * time.Millisecond}
		if c != want {
			t.Errorf("config = %+v, want %+v", c, want)
		}
	})
}

// A paradigm can turn the square on by default and still be overridden.
func TestMarkerFlagsDefaults(t *testing.T) {
	withFreshFlags(t, func() {
		MarkerFlags(MarkerConfig{Photodiode: true})
		if err := flag.CommandLine.Parse(nil); err != nil {
			t.Fatal(err)
		}
		if c, _ := markerConfigFromFlags(); !c.Photodiode {
			t.Error("default Photodiode: true not applied")
		}
	})
	withFreshFlags(t, func() {
		MarkerFlags(MarkerConfig{Photodiode: true})
		if err := flag.CommandLine.Parse([]string{"-photodiode=false"}); err != nil {
			t.Fatal(err)
		}
		if c, _ := markerConfigFromFlags(); c.Photodiode {
			t.Error("-photodiode=false did not override the default")
		}
	})
}

// Registering after the parse would leave the flags unparsed and silently at
// their defaults; registering twice would panic in package flag. Both must be
// refused with a message that says what to do.
func TestMarkerFlagsMisuseIsFatal(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func()
	}{
		{"after Parse", func() { _ = flag.CommandLine.Parse(nil) }},
		{"twice", func() { MarkerFlags() }},
	} {
		t.Run(c.name, func(t *testing.T) {
			withFreshFlags(t, func() {
				var msg string
				markerFatal = func(format string, args ...any) { msg = fmt.Sprintf(format, args...) }
				c.setup()
				MarkerFlags()
				if msg == "" {
					t.Error("MarkerFlags misuse was accepted")
				}
			})
		})
	}
}

func TestMarkerConfigValidate(t *testing.T) {
	for _, c := range []MarkerConfig{
		{Photodiode: true, PhotodiodeSize: -1},
		{Frames: -1},
		{TTLDuration: -time.Millisecond},
	} {
		if err := c.validate(); err == nil {
			t.Errorf("validate(%+v) accepted an invalid config", c)
		}
	}
	if err := (MarkerConfig{Photodiode: true, PhotodiodeSize: 100, Frames: 3}).validate(); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}
}

// A program-supplied device is used as is, without going through a spec.
func TestOpenMarkerOutputPrefersSuppliedDevice(t *testing.T) {
	dev := &countingOutput{}
	e := &Experiment{MarkerConfig: MarkerConfig{TTL: "this-is-not-a-device", TTLDevice: dev}}
	out, _, err := e.openMarkerOutput()
	if err != nil {
		t.Fatal(err)
	}
	if out != dev {
		t.Errorf("output = %v, want the supplied device", out)
	}
}

func TestOpenMarkerOutputRejectsBadSpec(t *testing.T) {
	e := &Experiment{MarkerConfig: MarkerConfig{TTL: "nosuchdevice"}}
	if _, _, err := e.openMarkerOutput(); err == nil {
		t.Error("an unknown -ttl device was accepted")
	}
}

type countingOutput struct{ sends int }

func (c *countingOutput) Send(byte) error { c.sends++; return nil }
func (c *countingOutput) AllLow() error   { return nil }
func (c *countingOutput) Close() error    { return nil }
