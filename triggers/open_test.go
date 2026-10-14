// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"strings"
	"testing"
)

// The device-spec parser is the whole command-line contract of -ttl and
// -device, and it runs before any hardware is touched, so it can be checked
// without a bench.

func TestParseDeviceSpecShortAndLongFormsAgree(t *testing.T) {
	for _, c := range []struct{ short, long, key, want string }{
		{"megttlbox:/dev/ttyACM0", "megttlbox:port=/dev/ttyACM0", "port", "/dev/ttyACM0"},
		{"dlpio8:/dev/ttyUSB0", "dlpio8:port=/dev/ttyUSB0", "port", "/dev/ttyUSB0"},
		{"parallel:/dev/parport1", "parallel:port=/dev/parport1", "port", "/dev/parport1"},
		{"labjackt4:10.0.0.2", "labjackt4:host=10.0.0.2", "host", "10.0.0.2"},
		{"gpio:/dev/gpiochip4", "gpio:chip=/dev/gpiochip4", "chip", "/dev/gpiochip4"},
	} {
		for _, s := range []string{c.short, c.long} {
			spec, err := ParseDeviceSpec(s)
			if err != nil {
				t.Fatalf("ParseDeviceSpec(%q): %v", s, err)
			}
			if got := spec.Params[c.key]; got != c.want {
				t.Errorf("ParseDeviceSpec(%q): %s = %q, want %q", s, c.key, got, c.want)
			}
		}
	}
}

func TestParseDeviceSpecShortFormTakesMoreParameters(t *testing.T) {
	spec, err := ParseDeviceSpec("mmbts:/dev/ttyACM0,mode=s,pin=3")
	if err != nil {
		t.Fatalf("ParseDeviceSpec: %v", err)
	}
	if spec.Params["port"] != "/dev/ttyACM0" || spec.Params["mode"] != "s" || spec.Params["pin"] != "3" {
		t.Errorf("params = %v", spec.Params)
	}
}

func TestParseDeviceSpecAutoMeansProbe(t *testing.T) {
	for _, s := range []string{"dlpio8", "dlpio8:auto", "dlpio8:port=AUTO"} {
		spec, err := ParseDeviceSpec(s)
		if err != nil {
			t.Fatalf("ParseDeviceSpec(%q): %v", s, err)
		}
		if spec.Params["port"] != "" {
			t.Errorf("ParseDeviceSpec(%q): port = %q, want empty (auto-detect)", s, spec.Params["port"])
		}
	}
}

func TestParseDeviceSpecAliases(t *testing.T) {
	for alias, kind := range map[string]string{"parport": "parallel", "megttl:/dev/ttyACM0": "megttlbox"} {
		spec, err := ParseDeviceSpec(alias)
		if err != nil {
			t.Fatalf("ParseDeviceSpec(%q): %v", alias, err)
		}
		if spec.Kind != kind {
			t.Errorf("ParseDeviceSpec(%q).Kind = %q, want %q", alias, spec.Kind, kind)
		}
	}
}

func TestParseDeviceSpecRejects(t *testing.T) {
	for _, c := range []struct{ name, spec, want string }{
		{"unknown kind", "dlpio9", "unknown device kind"},
		{"empty", " ", "empty device spec"},
		{"unknown key", "dlpio8:pins=1", `takes no "pins"`},
		{"bare value after first field", "dlpio8:pin=1,/dev/ttyUSB0", "is not key=value"},
		{"bare value on a kind without address", "null:/dev/x", "is not key=value"},
		{"repeated key", "dlpio8:/dev/a,port=/dev/b", "given twice"},
		{"megttlbox needs a port", "megttlbox", "needs port="},
		{"mmbts needs a port", "mmbts:mode=p", "needs port="},
		{"labjackt4 needs a host", "labjackt4", "needs host="},
		{"bad mmbts mode", "mmbts:/dev/ttyACM0,mode=x", "mode="},
		{"pin out of range", "parallel:pin=9", "out of range"},
		{"short gpio pin list", "gpio:pins=17+27", "8 pin numbers"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseDeviceSpec(c.spec)
			if err == nil {
				t.Fatalf("ParseDeviceSpec(%q) succeeded; want an error mentioning %q", c.spec, c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("ParseDeviceSpec(%q) = %q, want it to mention %q", c.spec, err, c.want)
			}
		})
	}
}

func TestOpenEmptyMeansNoDevice(t *testing.T) {
	dev, desc, err := Open("  ")
	if dev != nil || desc != "" || err != nil {
		t.Errorf(`Open("") = %v, %q, %v; want nil, "", nil`, dev, desc, err)
	}
}

func TestOpenNull(t *testing.T) {
	dev, desc, err := Open("null")
	if err != nil {
		t.Fatalf("Open(null): %v", err)
	}
	if _, ok := dev.(NullOutputTTLDevice); !ok {
		t.Errorf("Open(null) device = %T", dev)
	}
	// A run on the null device must say so wherever its description is written.
	if !strings.Contains(desc, "NO HARDWARE") {
		t.Errorf("Open(null) description %q does not say NO HARDWARE", desc)
	}
}
