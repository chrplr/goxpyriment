//go:build linux

package sysinfo

import (
	"strconv"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Linux scheduling policies, from sched.h. x/sys/unix does not wrap
// sched_getscheduler or sched_getparam, so the syscalls are made directly.
const (
	schedOther    = 0
	schedFIFO     = 1
	schedRR       = 2
	schedBatch    = 3
	schedIdle     = 5
	schedDeadline = 6
)

var schedNames = map[int]string{
	schedOther:    "SCHED_OTHER",
	schedFIFO:     "SCHED_FIFO",
	schedRR:       "SCHED_RR",
	schedBatch:    "SCHED_BATCH",
	schedIdle:     "SCHED_IDLE",
	schedDeadline: "SCHED_DEADLINE",
}

// collectScheduling reports the policy, priority and nice value of this
// process, plus the real-time priority ceiling this user is allowed.
//
// The ceiling comes from RLIMIT_RTPRIO and is the diagnostic half of the
// answer: it distinguishes "this run did not ask for real-time scheduling"
// from "this user could not have had it", which look identical in timing data
// and have completely different fixes.
func collectScheduling() SchedulingInfo {
	info := SchedulingInfo{Policy: "unknown"}

	// sched_getscheduler(0) — the calling thread's policy.
	if r, _, errno := unix.Syscall(unix.SYS_SCHED_GETSCHEDULER, 0, 0, 0); errno == 0 {
		policy := int(r)
		if name, ok := schedNames[policy]; ok {
			info.Policy = name
		} else {
			info.Policy = "policy " + strconv.Itoa(policy)
		}
		info.RealTime = policy == schedFIFO || policy == schedRR ||
			policy == schedDeadline
	}

	// sched_getparam(0, &param) — sched_priority is the only member.
	var param struct{ SchedPriority int32 }
	if _, _, errno := unix.Syscall(unix.SYS_SCHED_GETPARAM, 0,
		uintptr(unsafe.Pointer(&param)), 0); errno == 0 {
		info.Priority = int(param.SchedPriority)
	}

	if prio, err := unix.Getpriority(unix.PRIO_PROCESS, 0); err == nil {
		// Getpriority returns the nice value biased by 20 to keep it positive.
		info.Nice = 20 - prio
	}

	var rlim unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_RTPRIO, &rlim); err == nil {
		info.RealTimeMax = int(rlim.Cur)
	}

	return info
}
