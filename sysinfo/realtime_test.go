package sysinfo

import (
	"runtime"
	"strings"
	"testing"
)

// Range validation must reject before the syscall, so an out-of-range request
// gives a clear message rather than a bare EINVAL.
func TestRaiseToRealTimeRejectsOutOfRangePriority(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("range check is in the linux implementation")
	}
	for _, p := range []int{0, -1, 100, 1000} {
		err := RaiseToRealTime(p)
		if err == nil {
			t.Errorf("RaiseToRealTime(%d) succeeded; want an error", p)
			continue
		}
		if !strings.Contains(err.Error(), "range") {
			t.Errorf("RaiseToRealTime(%d) = %q, want it to mention the range", p, err)
		}
	}
}

// On a platform with no implementation the error must name the platform, so a
// user is not left wondering whether the call did something.
func TestRaiseToRealTimeUnsupportedNamesThePlatform(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("linux implements it")
	}
	err := RaiseToRealTime(50)
	if err == nil {
		t.Fatal("RaiseToRealTime succeeded on a platform with no implementation")
	}
	if !strings.Contains(err.Error(), runtime.GOOS) {
		t.Errorf("%q does not name the platform", err)
	}
}
