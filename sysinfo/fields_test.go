// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package sysinfo

import (
	"strings"
	"testing"
)

func fieldMap(t *testing.T, s SysInfo) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, kv := range s.Fields() {
		if _, dup := m[kv[0]]; dup {
			t.Errorf("duplicate key %q", kv[0])
		}
		m[kv[0]] = kv[1]
	}
	return m
}

func TestFieldsOmitsEmptyValues(t *testing.T) {
	// A machine where every probe failed must produce no lines at all, rather
	// than a block of empty keys: WriteHostInfo skips the section header on an
	// empty slice, and a header followed by nothing reads as a bug.
	if got := (SysInfo{}).Fields(); len(got) != 0 {
		t.Fatalf("empty SysInfo produced %d fields: %v", len(got), got)
	}
}

func TestFieldsCarriesTheConditionsThatChangeTiming(t *testing.T) {
	s := SysInfo{
		Machine: MachineInfo{SysVendor: "Dell Inc.", ProductName: "Precision 5490", DeviceType: "laptop"},
		System:  SystemInfo{OS: "Ubuntu 26.04 LTS", Kernel: "7.0.0-29-generic", Arch: "x86_64", Desktop: "ubuntu:GNOME"},
		CPU:     CPUInfo{Model: "Intel Core Ultra 7 165H", Cores: 16, Threads: 22, MHz: 2188, MaxMHz: 4700},
		Memory:  MemInfo{TotalKB: 31500000, UsedKB: 12460000},
		GPUs: []GPUInfo{
			{Model: "Intel Meteor Lake-P", Driver: "i915"},
			{Model: "NVIDIA AD107GLM", Driver: "nvidia"},
		},
		Audio: AudioInfo{
			Cards:  []AudioCard{{Name: "sof-soundwire", Driver: "snd_soc_sof_sdw"}},
			Server: "PipeWire", SrvVer: "1.6.2",
		},
	}
	m := fieldMap(t, s)

	for _, want := range []struct{ key, substr string }{
		{"machine", "Precision 5490"},
		{"os", "Ubuntu 26.04"},
		{"kernel", "7.0.0-29-generic x86_64"},
		{"desktop", "ubuntu:GNOME"}, // the compositor; SDL only says "wayland"
		{"cpu_topology", "16 cores / 22 threads"},
		{"cpu_mhz", "max 4700"}, // throttling is invisible without the maximum
		{"audio_server", "PipeWire 1.6.2"},
	} {
		if !strings.Contains(m[want.key], want.substr) {
			t.Errorf("field %q = %q, want it to contain %q", want.key, m[want.key], want.substr)
		}
	}

	// Both GPUs, each with its kernel driver. A laptop that renders on the
	// integrated chip while a discrete one is present is a different machine
	// from one that has only the integrated chip, and gl_renderer names the
	// winner alone.
	if !strings.Contains(m["gpu0"], "i915") || !strings.Contains(m["gpu1"], "nvidia") {
		t.Errorf("both GPUs and drivers expected, got gpu0=%q gpu1=%q", m["gpu0"], m["gpu1"])
	}
}

func TestFieldsDoesNotRepeatWhatSDLReports(t *testing.T) {
	// The display mode, the opened audio device and the renderer are recorded
	// from SDL under --SYSTEM INFO. Sourcing them a second time from the OS
	// would put two numbers that can disagree in one file.
	m := fieldMap(t, Collect())
	for _, forbidden := range []string{"display", "refresh", "resolution", "renderer", "audio_device", "vsync"} {
		for k := range m {
			if strings.Contains(k, forbidden) {
				t.Errorf("field %q duplicates something SDL already reports (%q)", k, forbidden)
			}
		}
	}
}

func TestSingleGPUOrCardIsNotNumbered(t *testing.T) {
	m := fieldMap(t, SysInfo{
		GPUs:  []GPUInfo{{Model: "V3D 4.2.14.0"}},
		Audio: AudioInfo{Cards: []AudioCard{{Name: "bcm2835"}}},
	})
	if _, ok := m["gpu"]; !ok {
		t.Errorf("a lone GPU should be keyed \"gpu\", got keys %v", m)
	}
	if _, ok := m["audio_card"]; !ok {
		t.Errorf("a lone sound card should be keyed \"audio_card\", got keys %v", m)
	}
}

func TestHostIsCached(t *testing.T) {
	// Host must not re-fork on every call: WriteHostInfo reads it after the
	// process has gone real-time.
	a, b := Host(), Host()
	if a.System.Hostname != b.System.Hostname || len(a.GPUs) != len(b.GPUs) {
		t.Errorf("Host() returned differing snapshots")
	}
}

func TestCPUClockReportedWithWhateverIsKnown(t *testing.T) {
	// macOS reports no maximum clock, and Apple Silicon reports no current one
	// either. Requiring both dropped the line entirely on those machines.
	for _, tc := range []struct {
		name     string
		cpu      CPUInfo
		want     string
		wantNone bool
	}{
		{name: "both", cpu: CPUInfo{MHz: 2188, MaxMHz: 4700}, want: "2188 (max 4700)"},
		{name: "current only (Intel Mac)", cpu: CPUInfo{MHz: 2600}, want: "2600"},
		{name: "max only", cpu: CPUInfo{MaxMHz: 3200}, want: "max 3200"},
		{name: "neither (Apple Silicon)", cpu: CPUInfo{Model: "Apple M3"}, wantNone: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fieldMap(t, SysInfo{CPU: tc.cpu})
			got, ok := m["cpu_mhz"]
			if tc.wantNone {
				if ok {
					t.Errorf("cpu_mhz = %q, want the field to be absent", got)
				}
				return
			}
			if got != tc.want {
				t.Errorf("cpu_mhz = %q, want %q", got, tc.want)
			}
		})
	}
}
