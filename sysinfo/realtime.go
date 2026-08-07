package sysinfo

// RaiseToRealTime asks the OS to schedule the CALLING THREAD at real-time
// priority, returning an error explaining why not if it cannot.
//
// This is what `chrt -f <priority> <command>` does, minus the exec. It needs no
// privilege: RLIMIT_RTPRIO is a resource limit, so a process may raise its own
// priority up to whatever limit it was granted. On Linux that grant comes from
// a file in /etc/security/limits.d/ and takes effect at login; see
// docs/SettingPriorityUnderLinux.md.
//
// **It is not equivalent to chrt**, and the difference is worth understanding
// before relying on it. chrt sets the policy before exec, so every thread the
// Go runtime later creates inherits it. This call sets only the thread it runs
// on, because that is what sched_setscheduler does on Linux. So the garbage
// collector and the runtime's other threads stay at normal priority.
//
// That is mostly an advantage: a runaway goroutine cannot starve every core,
// and the GC is not competing at real-time priority. The cost is a priority
// inversion that chrt does not have — if the real-time thread waits on a
// stop-the-world GC phase whose workers are being starved by other load, it
// waits at their speed, not its own. Avoiding allocation in the timing-critical
// path is the mitigation, not a higher priority.
//
// Call it from a goroutine locked to its thread with runtime.LockOSThread, or
// the elevation lands on whichever thread happened to be running and the
// goroutine may be moved off it afterwards.
func RaiseToRealTime(priority int) error { return raiseToRealTime(priority) }
