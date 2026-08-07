//go:build !linux

package sysinfo

import (
	"fmt"
	"runtime"
)

// Real-time elevation is deliberately Linux-only.
//
// Windows has SetPriorityClass and macOS has thread_policy_set, but neither is
// the same thing, and on both platforms the setting that matters most for
// stimulus timing is something else entirely: the timer resolution on Windows,
// and a per-thread time-constraint policy on macOS. Pretending a portable
// "raise priority" call covers them would produce a program that reports
// success while doing something different on each platform.
func raiseToRealTime(int) error {
	return fmt.Errorf("real-time scheduling is not implemented on %s", runtime.GOOS)
}
