//go:build linux

package sysinfo

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

func raiseToRealTime(priority int) error {
	if priority < 1 || priority > 99 {
		return fmt.Errorf("real-time priority %d out of range (1-99)", priority)
	}

	// pid 0 means the calling thread, not the process. See RaiseToRealTime.
	param := struct{ SchedPriority int32 }{int32(priority)}
	_, _, errno := unix.Syscall(unix.SYS_SCHED_SETSCHEDULER, 0, schedFIFO,
		uintptr(unsafe.Pointer(&param)))
	if errno == 0 {
		return nil
	}
	if errno != unix.EPERM {
		return fmt.Errorf("sched_setscheduler: %w", errno)
	}

	// EPERM has two quite different causes and the same message, which is how
	// people conclude the setup does not work when in fact they asked for more
	// than they were granted. Read the limit and say which it was.
	var rlim unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_RTPRIO, &rlim); err == nil {
		if rlim.Cur == 0 {
			return fmt.Errorf("real-time scheduling is not permitted for this " +
				"user (RLIMIT_RTPRIO is 0). Grant it with a file in " +
				"/etc/security/limits.d/, then log out and back in — the limit " +
				"is read at login, so a new terminal is not enough")
		}
		if uint64(priority) > rlim.Cur {
			return fmt.Errorf("priority %d exceeds the %d this user is granted "+
				"(RLIMIT_RTPRIO); ask for %d or less, or raise the grant",
				priority, rlim.Cur, rlim.Cur)
		}
	}
	return fmt.Errorf("sched_setscheduler: %w", errno)
}
