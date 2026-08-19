// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build !js

package main

import (
	"fmt"

	"github.com/chrplr/goxpyriment/triggers"
)

// openTTL opens the DLP-IO8-G named by the -dlpio8 flag.
//
// spec is "" (no triggers), "auto" (probe the serial ports for a DLP-IO8-G),
// or a device name such as /dev/ttyUSB0 or COM3. It returns the device, the
// port it was opened on ("" when triggers are disabled), and an error only
// when a device was explicitly named and could not be opened — a run that
// asked for triggers must not start silently without them.
//
// "auto" is deliberately more forgiving: triggers.AutoDetectDLPIO8 logs and
// returns a null device when no board is present, which is what you want when
// the same command line is used on a machine without the hardware.
func openTTL(spec string) (ttlDevice, string, error) {
	switch spec {
	case "":
		return nullTTL{}, "", nil
	case "auto":
		dev, port, err := triggers.AutoDetectDLPIO8()
		if err != nil {
			return nullTTL{}, "", err
		}
		if port == "" {
			return nullTTL{}, "", nil // none found; AutoDetectDLPIO8 has logged it
		}
		return dev, port, nil
	default:
		dev, err := triggers.NewDLPIO8(spec)
		if err != nil {
			return nullTTL{}, "", fmt.Errorf("opening DLP-IO8-G on %s: %w", spec, err)
		}
		return dev, spec, nil
	}
}
