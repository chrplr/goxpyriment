// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"slices"
	"testing"
)

// A blind probe opens every port, and opening a USB-CDC port asserts DTR, which
// resets the device behind it. On a rig with an Arduino trigger box that means
// autodetecting one instrument reboots another — so CDC-looking names must be
// dropped before anything is opened.
func TestFilterProbeCandidatesDropsCDCPorts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ports []string
		want  []string
	}{
		{
			name:  "linux: an Arduino alongside two FTDI devices",
			ports: []string{"/dev/ttyACM0", "/dev/ttyUSB0", "/dev/ttyUSB1"},
			want:  []string{"/dev/ttyUSB0", "/dev/ttyUSB1"},
		},
		{
			name:  "macOS: usbmodem is CDC, usbserial is FTDI",
			ports: []string{"/dev/cu.usbmodem14201", "/dev/cu.usbserial-A50285BI"},
			want:  []string{"/dev/cu.usbserial-A50285BI"},
		},
		{
			name:  "macOS: the tty.* spelling too",
			ports: []string{"/dev/tty.usbmodem1", "/dev/tty.usbserial-1"},
			want:  []string{"/dev/tty.usbserial-1"},
		},
		{
			name:  "windows: COM names carry no hint, so all are kept",
			ports: []string{"COM3", "COM4"},
			want:  []string{"COM3", "COM4"},
		},
		{
			name:  "nothing attached",
			ports: nil,
			want:  []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := filterProbeCandidates(tc.ports)
			if !slices.Equal(got, tc.want) {
				t.Errorf("filterProbeCandidates(%v) = %v, want %v", tc.ports, got, tc.want)
			}
		})
	}
}

// The candidate list must never contain a port that would reset a board when
// opened, whatever this machine happens to have attached.
func TestDLPPortCandidatesNeverIncludesCDC(t *testing.T) {
	ports, err := dlpPortCandidates()
	if err != nil {
		t.Fatalf("dlpPortCandidates: %v", err)
	}
	for _, p := range ports {
		for _, bad := range []string{"ttyACM", "usbmodem"} {
			if len(p) >= len(bad) && containsSub(p, bad) {
				t.Errorf("candidate %q would reset a USB-CDC device when opened", p)
			}
		}
	}
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
