//go:build js || wasip1

package sysinfo

import "runtime"

// collectScheduling reports that scheduling is not observable here.
//
// In a browser or a WASI sandbox there is no OS scheduler to ask: the host
// runtime decides when this code runs and exposes nothing about it. That is not
// a missing implementation to be filled in later, so Fidelity says what is
// actually the case rather than leaving a caller to read an all-zero struct as
// "normal priority".
//
// x/sys/unix does compile for these targets, which is the trap: it simply
// defines none of the scheduling calls, so a build tag of "every Unix except
// the three we named" silently includes them and fails at link time.
func collectScheduling() SchedulingInfo {
	return SchedulingInfo{
		Policy: "n/a",
		Fidelity: "scheduling is not observable on " + runtime.GOOS +
			"; the host runtime decides when this code runs",
	}
}
