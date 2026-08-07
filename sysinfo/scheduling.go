package sysinfo

import (
	"fmt"
	"strings"
)

// SchedulingInfo describes how the operating system is scheduling this process.
//
// It belongs in a system report because it changes timing results by more than
// anything the experiment code does, and it is invisible in the data
// afterwards. A run at normal priority and the same run under real-time
// scheduling produce different distributions from identical code, and nothing
// in the recorded stimulus timings says which one you are looking at.
//
// Fidelity differs by platform and the struct does not pretend otherwise; see
// Fidelity. Comparing a raw Priority across operating systems is meaningless —
// compare RealTime, and read Policy as text.
type SchedulingInfo struct {
	// Policy is the scheduling class, in the platform's own vocabulary:
	// "SCHED_FIFO", "SCHED_OTHER" on Unix, "HIGH_PRIORITY_CLASS" and friends on
	// Windows. Deliberately not normalised, because the classes do not
	// correspond one to one.
	Policy string

	// Priority is the priority within that class, again in platform terms.
	Priority int

	// Nice is the Unix nice value, or the nearest platform equivalent.
	// Zero where the concept does not apply.
	Nice int

	// RealTime reports whether this process is actually running under a
	// real-time scheduling class. This is the field an experiment should assert
	// on before trusting its own timing.
	RealTime bool

	// RealTimeMax is the highest real-time priority this user could obtain
	// (Linux RLIMIT_RTPRIO), independent of whether the process asked for it.
	//
	// This is the most useful field in the struct and the reason it exists.
	// RealTime says what happened; RealTimeMax says what was possible. A run
	// recording RealTime=false with RealTimeMax=0 has diagnosed itself: the
	// privilege was never granted, so no amount of chrt would have helped, and
	// the usual cause is a limits.d file that was never written or a session
	// that was never logged out and back in. Without it you are left comparing
	// timing distributions and guessing.
	RealTimeMax int

	// Fidelity records what this platform's implementation cannot see, empty
	// when it sees everything. It is reported rather than silently omitted:
	// "no real-time policy detected" means something quite different on Linux,
	// where the question is answerable, than on macOS, where this struct simply
	// cannot observe a thread time-constraint policy.
	Fidelity string
}

// String renders the scheduling state for a system report.
func (s SchedulingInfo) String() string {
	var parts []string
	if s.Policy != "" {
		parts = append(parts, kv("policy", s.Policy))
	}
	if s.RealTime {
		parts = append(parts, kv("priority", fmt.Sprint(s.Priority)), "REAL-TIME")
	} else {
		parts = append(parts, kv("nice", fmt.Sprint(s.Nice)))
		switch {
		case s.RealTimeMax > 0:
			parts = append(parts, fmt.Sprintf("(real-time available up to %d, not used)",
				s.RealTimeMax))
		case s.Fidelity == "":
			parts = append(parts, "(real-time NOT available to this user)")
		}
	}
	if s.Fidelity != "" {
		parts = append(parts, "["+s.Fidelity+"]")
	}
	return strings.Join(parts, "  ")
}

// Scheduling returns how this process is currently being scheduled.
func Scheduling() SchedulingInfo { return collectScheduling() }
