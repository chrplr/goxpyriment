//go:build !linux && !windows && !darwin

package sysinfo

// UNTESTED fallback for platforms without a specific implementation. Reports
// the nice value, which POSIX guarantees, and nothing else.

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
