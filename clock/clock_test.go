// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package clock

import (
	"testing"
	"time"
)

func TestWaitBlocksAtLeastDuration(t *testing.T) {
	start := time.Now()
	Wait(20)
	if elapsed := time.Since(start); elapsed < 18*time.Millisecond {
		t.Errorf("Wait(20) returned after %v, expected ≥ ~20ms", elapsed)
	}
}

func TestGetTimeMonotonic(t *testing.T) {
	t0 := GetTime()
	Wait(5)
	t1 := GetTime()
	if t1 < t0 {
		t.Errorf("GetTime went backwards: %d then %d", t0, t1)
	}
	if GetTimeNS() <= 0 {
		t.Error("GetTimeNS should be positive after package init")
	}
}

func TestClockNowAdvances(t *testing.T) {
	c := NewClock()
	first := c.Now()
	Wait(5)
	second := c.Now()
	if second <= first {
		t.Errorf("Clock.Now did not advance: %v then %v", first, second)
	}
	if c.NowMillis() < 0 || c.NowNanos() < 0 {
		t.Error("Clock millis/nanos should be non-negative")
	}
}

func TestClockReset(t *testing.T) {
	c := NewClock()
	Wait(10)
	c.Reset()
	if now := c.Now(); now > 5*time.Millisecond {
		t.Errorf("after Reset, Now() = %v, expected near zero", now)
	}
}

func TestSleepUntilWaitsForTarget(t *testing.T) {
	c := NewClock()
	target := 25 * time.Millisecond
	c.SleepUntil(target)
	if now := c.Now(); now < target {
		t.Errorf("SleepUntil(%v) returned at %v, before target", target, now)
	}
}

func TestSleepUntilPastTargetReturnsImmediately(t *testing.T) {
	c := NewClock()
	Wait(10)
	start := time.Now()
	c.SleepUntil(1 * time.Millisecond) // already elapsed
	if elapsed := time.Since(start); elapsed > 5*time.Millisecond {
		t.Errorf("SleepUntil on a past target blocked for %v, expected immediate return", elapsed)
	}
}
