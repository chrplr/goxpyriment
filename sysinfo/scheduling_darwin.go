//go:build darwin

package sysinfo

// UNTESTED. Written against the documented BSD/Darwin APIs but never run on
// macOS. The Linux implementation was verified against chrt and nice on real
// hardware; this one has not been exercised, and Fidelity says so at runtime.

import "golang.org/x/sys/unix"

// collectScheduling reports the nice value, and is explicit that this is not
// the whole picture on Darwin.
//
// macOS does expose sched_getscheduler, but reading it would be misleading:
// real-time behaviour on Darwin is not a scheduling policy on the process, it
// is a per-thread time-constraint policy set through thread_policy_set with
// THREAD_TIME_CONSTRAINT_POLICY. A process whose audio or display thread holds
// such a policy still reports SCHED_OTHER, so reporting that would give a
// confident "not real-time" answer to a question this code cannot see.
//
// Rather than report a wrong answer, RealTime is left false and Fidelity says
// what was not looked at. A caller that needs the real answer on macOS has to
// ask the thread, not the process.
func collectScheduling() SchedulingInfo {
	info := SchedulingInfo{
		Policy: "SCHED_OTHER (assumed)",
		Fidelity: "UNTESTED on macOS; cannot see per-thread " +
			"THREAD_TIME_CONSTRAINT_POLICY, so RealTime is not authoritative",
	}
	if prio, err := unix.Getpriority(unix.PRIO_PROCESS, 0); err == nil {
		info.Nice = 20 - prio
	}
	return info
}
