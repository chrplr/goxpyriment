package sysinfo

import (
	"strings"
	"testing"
)

// The rendered line is what ends up in a recorded system report, and the whole
// point of the struct is that a reader can tell the three cases apart at a
// glance: real-time in use, real-time available but not used, and real-time not
// obtainable at all. Those have different fixes, so they must not read alike.
func TestSchedulingInfoStringDistinguishesTheThreeCases(t *testing.T) {
	tests := []struct {
		name     string
		info     SchedulingInfo
		contains []string
		absent   []string
	}{
		{
			name: "real-time in use",
			info: SchedulingInfo{Policy: "SCHED_FIFO", Priority: 50,
				RealTime: true, RealTimeMax: 50},
			contains: []string{"SCHED_FIFO", "50", "REAL-TIME"},
			absent:   []string{"NOT available"},
		},
		{
			name:     "available but not used",
			info:     SchedulingInfo{Policy: "SCHED_OTHER", RealTimeMax: 50},
			contains: []string{"SCHED_OTHER", "available up to 50", "not used"},
			absent:   []string{"REAL-TIME", "NOT available"},
		},
		{
			name:     "not obtainable — the forgotten grant",
			info:     SchedulingInfo{Policy: "SCHED_OTHER", RealTimeMax: 0},
			contains: []string{"SCHED_OTHER", "NOT available"},
			absent:   []string{"REAL-TIME"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.info.String()
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("%q missing %q", got, want)
				}
			}
			for _, no := range tt.absent {
				if strings.Contains(got, no) {
					t.Errorf("%q should not contain %q", got, no)
				}
			}
		})
	}
}

// A platform that cannot see the answer must not render as one that looked and
// found nothing: "NOT available to this user" is a claim, and only a platform
// that can actually check RLIMIT_RTPRIO is entitled to make it.
func TestSchedulingInfoWithLimitedFidelityMakesNoClaim(t *testing.T) {
	info := SchedulingInfo{Policy: "SCHED_OTHER (assumed)",
		Fidelity: "UNTESTED on macOS; cannot see per-thread policy"}
	got := info.String()
	if strings.Contains(got, "NOT available") {
		t.Errorf("%q claims real-time is unavailable, but this platform cannot tell", got)
	}
	if !strings.Contains(got, "UNTESTED") {
		t.Errorf("%q does not surface the fidelity caveat", got)
	}
}

// The live reporter must at least return something renderable on whatever
// platform the tests run on.
func TestSchedulingCollects(t *testing.T) {
	if got := Scheduling().String(); got == "" {
		t.Error("Scheduling() rendered an empty string")
	}
}
