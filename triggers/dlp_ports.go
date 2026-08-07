// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"path/filepath"
	"strings"

	"go.bug.st/serial"
)

// Autodetection has to open a port and write to it to find out what is there,
// and on a rig with several instruments that is not a harmless thing to do.
//
// Opening a USB-CDC port asserts DTR, which RESETS an Arduino: a trigger box
// mid-experiment reboots, and any line it was holding high drops. And an
// instrument with its own command grammar — a Black Box ToolKit, a scope —
// receiving unsolicited probe bytes can be left mid-stream needing an explicit
// break to recover.
//
// So probing is narrowed as far as each platform allows, rather than sweeping
// every port the OS reports.

// dlpByIDGlob matches a DLP module under the stable USB-id names Linux exposes.
const dlpByIDGlob = "/dev/serial/by-id/*DLP*"

// serialByIDDir exists whenever udev has enumerated any USB serial device.
const serialByIDDir = "/dev/serial/by-id"

// dlpPortCandidates returns the ports worth probing for a DLP module.
//
// On Linux the USB id is definitive: if any port carries "DLP" in its by-id
// name, only those are probed, and if the by-id directory exists but names no
// DLP then the device is simply not attached and NOTHING is probed. Either way
// no other instrument is touched.
//
// Elsewhere there is no by-id directory and the OS port list is all there is.
// Ports that look like USB-CDC are dropped from it, because those are the ones
// that reset when opened and a DLP is never among them — it is an FTDI device.
// On Windows the names carry no such information and every COM port is probed,
// which is worth knowing if an instrument shares the machine.
func dlpPortCandidates() ([]string, error) {
	if byID, err := filepath.Glob(dlpByIDGlob); err == nil && len(byID) > 0 {
		return byID, nil
	}
	// A by-id directory with no DLP in it is a positive answer: not attached.
	if entries, err := filepath.Glob(serialByIDDir + "/*"); err == nil && len(entries) > 0 {
		return nil, nil
	}
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}
	return filterProbeCandidates(ports), nil
}

// filterProbeCandidates drops ports that are USB-CDC devices, which reset when
// opened and are never a DLP module.
func filterProbeCandidates(ports []string) []string {
	out := make([]string, 0, len(ports))
	for _, p := range ports {
		base := filepath.Base(p)
		// Linux CDC ACM (an Arduino, among others) and the macOS equivalent.
		if strings.HasPrefix(base, "ttyACM") || strings.HasPrefix(base, "cu.usbmodem") ||
			strings.HasPrefix(base, "tty.usbmodem") {
			continue
		}
		out = append(out, p)
	}
	return out
}
