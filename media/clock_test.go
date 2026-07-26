// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package media

import (
	"testing"
	"time"
)

func TestMasterClockMonotonic(t *testing.T) {
	c := NewMasterClock()
	a := c.Now()
	time.Sleep(2 * time.Millisecond)
	b := c.Now()
	if !(b > a) {
		t.Fatalf("expected b > a, got a=%v b=%v", a, b)
	}
}

func TestMasterClockReset(t *testing.T) {
	c := NewMasterClock()
	time.Sleep(2 * time.Millisecond)
	c.Reset()
	if v := c.Now(); v > time.Millisecond {
		t.Fatalf("expected Now() ≈ 0 after Reset, got %v", v)
	}
}

func TestMasterClockBurstFreezes(t *testing.T) {
	c := NewMasterClock()
	c.BeginBurst()
	a := c.Now()
	time.Sleep(2 * time.Millisecond)
	b := c.Now()
	if a != b {
		t.Fatalf("expected frozen value, got a=%v b=%v", a, b)
	}
	if !c.Frozen() {
		t.Fatal("expected Frozen() == true inside burst")
	}
	c.EndBurst()
	if c.Frozen() {
		t.Fatal("expected Frozen() == false after EndBurst")
	}
	cAfter := c.Now()
	if !(cAfter > b) {
		t.Fatalf("expected clock to thaw and advance, got b=%v after=%v", b, cAfter)
	}
}

func TestMasterClockBurstNests(t *testing.T) {
	c := NewMasterClock()
	c.BeginBurst()
	c.BeginBurst()
	c.EndBurst()
	if !c.Frozen() {
		t.Fatal("expected still frozen after one EndBurst of two BeginBurst")
	}
	c.EndBurst()
	if c.Frozen() {
		t.Fatal("expected thawed after matching EndBurst calls")
	}
}

func TestMasterClockEndWithoutBegin(t *testing.T) {
	c := NewMasterClock()
	// Should be a no-op; must not panic or go negative.
	c.EndBurst()
	c.EndBurst()
	if c.Frozen() {
		t.Fatal("EndBurst without BeginBurst should leave clock thawed")
	}
}
