// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build !js

package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/triggers"
)

// Desktop TTL output. The browser build gets the no-op stubs in trigger_js.go
// instead: triggers/ drives serial and parallel ports and is excluded from
// GOOS=js, so importing it here would break the WASM bundle. Keeping the
// import behind a build tag is what lets this example stay browser-runnable
// as a demo while still firing real triggers in the scanner or the MEG.

// TriggerDeviceNames lists the accepted values of -trigger.
const TriggerDeviceNames = "none | dlpio8 | dlpio20 | ft232h | labjackt4 | parport | megttl"

// openTrigger opens the named TTL device and returns a fire function to call
// immediately after the onset flip, plus a close function.
//
// fire is never nil: "none" yields a no-op, so the frame loop needs no
// conditional and the timing of a test run matches that of a real one.
func openTrigger(name, device string, line, pulseMs int) (fire func(), closeFn func(), err error) {
	noop := func() {}
	if line < 0 || line > 7 {
		return nil, nil, fmt.Errorf("-trigger-line=%d out of range (0-7)", line)
	}
	dur := time.Duration(pulseMs) * time.Millisecond

	var dev triggers.OutputTTLDevice
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "none":
		return noop, noop, nil

	case "dlpio8":
		d, port, derr := triggers.AutoDetectDLPIO8()
		if derr != nil {
			return nil, nil, fmt.Errorf("dlpio8: %w", derr)
		}
		log.Printf("trigger: DLP-IO8 on %s", port)
		dev = d

	case "dlpio20":
		d, port, derr := triggers.AutoDetectDLPIO20()
		if derr != nil {
			return nil, nil, fmt.Errorf("dlpio20: %w", derr)
		}
		log.Printf("trigger: DLP-IO20 on %s", port)
		dev = d

	case "ft232h":
		d, derr := triggers.AutoDetectFT232H()
		if derr != nil {
			return nil, nil, fmt.Errorf("ft232h: %w", derr)
		}
		log.Print("trigger: FT232H")
		dev = d

	case "labjackt4":
		if device == "" {
			return nil, nil, fmt.Errorf("labjackt4 needs -trigger-device=<host:port>")
		}
		d, derr := triggers.NewLabJackT4(device)
		if derr != nil {
			return nil, nil, fmt.Errorf("labjackt4 %s: %w", device, derr)
		}
		log.Printf("trigger: LabJack T4 at %s", device)
		dev = d

	case "parport":
		path := device
		if path == "" {
			path = "/dev/parport0"
		}
		p := triggers.NewParallelPort(path)
		if oerr := p.Open(); oerr != nil {
			return nil, nil, fmt.Errorf("parport %s: %w", path, oerr)
		}
		log.Printf("trigger: parallel port %s", path)
		dev = p

	case "megttl":
		if device == "" {
			return nil, nil, fmt.Errorf("megttl needs -trigger-device=<serial port>")
		}
		d, derr := triggers.NewMEGTTLBox(device)
		if derr != nil {
			return nil, nil, fmt.Errorf("megttlbox %s: %w", device, derr)
		}
		log.Printf("trigger: MEG TTL box on %s", device)
		dev = d

	default:
		return nil, nil, fmt.Errorf("unknown -trigger=%q; expected %s", name, TriggerDeviceNames)
	}

	if aerr := dev.AllLow(); aerr != nil {
		log.Printf("trigger: could not reset lines low: %v", aerr)
	}

	// FireTriggerSync raises the line on the calling goroutine and defers only
	// the falling edge, so the rising edge lands tens of microseconds after the
	// flip. FireTrigger would block for the whole pulse; never use it here.
	fire = func() { triggers.FireTriggerSync(dev, line, dur) }
	closeFn = func() {
		if cerr := dev.AllLow(); cerr != nil {
			log.Printf("trigger: could not reset lines low: %v", cerr)
		}
		if cerr := dev.Close(); cerr != nil {
			log.Printf("trigger: close: %v", cerr)
		}
	}
	return fire, closeFn, nil
}
