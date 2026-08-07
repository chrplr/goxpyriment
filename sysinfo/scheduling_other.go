//go:build !linux && !windows && !darwin && !js && !wasip1

package sysinfo

// UNTESTED fallback for the remaining Unix-like platforms -- the BSDs, Solaris
// and so on. Reports the nice value, which POSIX guarantees, and nothing else.
//
// js and wasip1 are excluded because x/sys/unix compiles there but defines
// none of this; see scheduling_sandbox.go, which is also the honest answer for
// those targets, since a browser has no OS scheduler to report.

import "golang.org/x/sys/unix"

func collectScheduling() SchedulingInfo {
	info := SchedulingInfo{
		Policy:   "unknown",
		Fidelity: "no scheduling reporter for this platform; nice value only",
	}
	if prio, err := unix.Getpriority(unix.PRIO_PROCESS, 0); err == nil {
		info.Nice = 20 - prio
	}
	return info
}
