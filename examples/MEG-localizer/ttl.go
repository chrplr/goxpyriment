// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build !js

package main

import (
	"fmt"
	"strings"

	"github.com/chrplr/goxpyriment/triggers"
)

// openTTL opens the trigger device named by the -ttl flag.
//
// spec is "" (no triggers) or DEVICE[:PORT]:
//
//	megttlbox:/dev/ttyACM0   NeuroSpin MEG TTL box (Arduino Mega); port required
//	mmbts:/dev/ttyACM0       NEUROSPEC MMBT-S; port required (it cannot be probed)
//	dlpio8[:PORT|auto]       DLP-IO8-G; a port name, or auto (the default) to probe for one
//	parallel[:/dev/parport0] parallel port; the first accessible one by default
//
// It returns the device and a description of what was opened ("" when
// triggers are disabled). Every failure is an error, including "auto" finding
// nothing: a run that asked for triggers must not start silently without
// them, since the recording would look fine until it was analysed.
func openTTL(spec string) (ttlDevice, string, error) {
	if spec == "" {
		return nullTTL{}, "", nil
	}
	name, port, _ := strings.Cut(spec, ":")
	switch name {
	case "megttlbox":
		if port == "" {
			return nullTTL{}, "", fmt.Errorf("megttlbox needs a port: -ttl megttlbox:/dev/ttyACM0")
		}
		dev, err := triggers.NewMEGTTLBox(port)
		if err != nil {
			return nullTTL{}, "", err
		}
		return dev, "megttlbox " + port + " (" + dev.Info().String() + ")", nil
	case "mmbts":
		if port == "" {
			return nullTTL{}, "", fmt.Errorf("mmbts needs a port: -ttl mmbts:/dev/ttyACM0")
		}
		dev, err := triggers.NewMMBTS(port)
		if err != nil {
			return nullTTL{}, "", err
		}
		return dev, "mmbts " + port, nil
	case "dlpio8":
		if port == "" || port == "auto" {
			dev, found, err := triggers.AutoDetectDLPIO8()
			if err != nil {
				return nullTTL{}, "", err
			}
			if found == "" {
				return nullTTL{}, "", fmt.Errorf("dlpio8: no DLP-IO8-G found on any serial port")
			}
			return dev, "dlpio8 " + found, nil
		}
		dev, err := triggers.NewDLPIO8(port)
		if err != nil {
			return nullTTL{}, "", err
		}
		return dev, "dlpio8 " + port, nil
	case "parallel":
		if port == "" {
			ports := triggers.AvailableParallelPorts()
			if len(ports) == 0 {
				return nullTTL{}, "", fmt.Errorf("parallel: no accessible parallel port " +
					"(needs the ppdev module and rw access to /dev/parport*)")
			}
			port = ports[0]
		}
		dev := triggers.NewParallelPort(port)
		if err := dev.Open(); err != nil {
			return nullTTL{}, "", err
		}
		return dev, "parallel " + port, nil
	}
	return nullTTL{}, "", fmt.Errorf("unknown device %q: choose megttlbox, mmbts, dlpio8 or parallel", name)
}
