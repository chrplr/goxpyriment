//go:build windows

package sysinfo

// UNTESTED. Written against the documented Win32 API but never run on Windows.
// The Linux implementation in scheduling_linux.go was verified against chrt and
// nice on real hardware; this one has not been exercised at all, and the
// Fidelity field says so at runtime so a report cannot quietly imply otherwise.

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func unsafePtr[T any](p *T) unsafe.Pointer { return unsafe.Pointer(p) }

// Process priority classes, from processthreadsapi.h.
const (
	idlePriorityClass        = 0x00000040
	belowNormalPriorityClass = 0x00004000
	normalPriorityClass      = 0x00000020
	aboveNormalPriorityClass = 0x00008000
	highPriorityClass        = 0x00000080
	realtimePriorityClass    = 0x00000100
)

var priorityClassNames = map[uint32]string{
	idlePriorityClass:        "IDLE_PRIORITY_CLASS",
	belowNormalPriorityClass: "BELOW_NORMAL_PRIORITY_CLASS",
	normalPriorityClass:      "NORMAL_PRIORITY_CLASS",
	aboveNormalPriorityClass: "ABOVE_NORMAL_PRIORITY_CLASS",
	highPriorityClass:        "HIGH_PRIORITY_CLASS",
	realtimePriorityClass:    "REALTIME_PRIORITY_CLASS",
}

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procGetPriorityClass  = kernel32.NewProc("GetPriorityClass")
	procGetThreadPriority = kernel32.NewProc("GetThreadPriority")

	ntdll                      = windows.NewLazySystemDLL("ntdll.dll")
	procNtQueryTimerResolution = ntdll.NewProc("NtQueryTimerResolution")
)

// collectScheduling reports the process priority class and the current system
// timer resolution.
//
// Windows has no scheduling policy in the POSIX sense, so the priority class is
// the closest analogue and is reported in its own vocabulary rather than mapped
// onto SCHED_* names that would not mean the same thing.
//
// The timer resolution matters more than the priority class for millisecond
// stimulus timing, which is why it is collected here even though it is not
// scheduling: the default tick has historically been ~15.6 ms, and a process
// that has not raised it cannot place an event to the millisecond however high
// its priority. Go's runtime raises it on Windows, but that is a runtime
// implementation detail rather than a guarantee, so it is measured rather than
// assumed.
func collectScheduling() SchedulingInfo {
	info := SchedulingInfo{
		Policy:   "unknown",
		Fidelity: "UNTESTED on Windows; no POSIX policy, priority class shown instead",
	}

	if h, err := windows.GetCurrentProcess(); err == nil {
		if r, _, _ := procGetPriorityClass.Call(uintptr(h)); r != 0 {
			cls := uint32(r)
			if name, ok := priorityClassNames[cls]; ok {
				info.Policy = name
			} else {
				info.Policy = fmt.Sprintf("priority class 0x%X", cls)
			}
			info.RealTime = cls == realtimePriorityClass
		}
	}

	// GetThreadPriority returns THREAD_PRIORITY_ERROR_RETURN (0x7FFFFFFF) on
	// failure, which is why the sentinel is checked rather than the error.
	if h := windows.CurrentThread(); h != 0 {
		if r, _, _ := procGetThreadPriority.Call(uintptr(h)); int32(r) != 0x7FFFFFFF {
			info.Priority = int(int32(r))
		}
	}

	var minRes, maxRes, curRes uint32
	if r, _, _ := procNtQueryTimerResolution.Call(
		uintptr(unsafePtr(&minRes)), uintptr(unsafePtr(&maxRes)),
		uintptr(unsafePtr(&curRes))); r == 0 && curRes != 0 {
		// Values are in 100 ns units.
		info.Fidelity += fmt.Sprintf("; timer resolution %.3f ms",
			float64(curRes)/10000.0)
	}

	// Windows has no nice value and no RLIMIT_RTPRIO. Leaving RealTimeMax at 0
	// would read as "real-time not available to this user", which is not what it
	// means here, so the Fidelity note above carries the caveat instead.
	return info
}
