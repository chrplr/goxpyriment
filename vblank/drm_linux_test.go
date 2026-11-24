// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build linux

package vblank

import (
	"syscall"
	"testing"
	"unsafe"
)

func TestMonotonicNSWorks(t *testing.T) {
	a, err := monotonicNS()
	if err != nil {
		t.Fatalf("monotonicNS: %v", err)
	}
	b, err := monotonicNS()
	if err != nil {
		t.Fatalf("monotonicNS: %v", err)
	}
	if a == 0 {
		t.Error("monotonicNS returned 0")
	}
	if b < a {
		t.Errorf("monotonicNS should be monotonic, got %d then %d", a, b)
	}
}

// TestDrmWaitVblankLayout pins the binary layout of drmWaitVblank to
// the kernel's `union drm_wait_vblank` (24 bytes total). If Go ever
// changes struct padding rules, this test catches the breakage at
// compile/test time rather than at first vsync query in production.
func TestDrmWaitVblankLayout(t *testing.T) {
	const want = 24
	got := unsafe.Sizeof(drmWaitVblank{})
	if got != want {
		t.Errorf("drmWaitVblank size = %d bytes, want %d", got, want)
	}
	// Type and Sequence must occupy the first 8 bytes (matches reply
	// layout where tval_sec starts at offset 8 within Data).
	v := drmWaitVblank{Type: 1, Sequence: 2}
	v.Data[0] = 0xAA
	if v.Type != 1 || v.Sequence != 2 || v.Data[0] != 0xAA {
		t.Error("drmWaitVblank fields are not independent — layout drift")
	}
}

// TestPollForSequenceResolvesNextVblank exercises the path that only runs when
// the caller's query beats the vblank IRQ.
//
// That path is the whole point of the sequence handling, and it is also the one
// least likely to be covered by an ordinary run: on a machine where the query
// consistently lands after the IRQ it never executes, so a bug in it would stay
// hidden until the run that needed it — which is exactly the machine with no
// developer sitting at it. Here it is provoked deliberately, by asking for the
// vblank after the most recent one, which cannot have happened yet.
//
// Skips where there is no usable DRM node, so it is silent on CI and on any
// machine without a display.
func TestPollForSequenceResolvesNextVblank(t *testing.T) {
	// An empty Target: the test does not care which head answers, only that the
	// poll resolves the sequence it asks for, and naming a display would make
	// the test skip on the machines it most needs to run on.
	fd, path, pipe, err := findDRMNode(Target{})
	if err != nil {
		t.Skipf("no DRM vblank source here: %v", err)
	}
	defer syscall.Close(fd) //nolint:errcheck
	// Built directly rather than through newBackendOn, which needs SDL loaded
	// for its clock. Only the ioctl path is under test, and query and the poll
	// work in CLOCK_MONOTONIC without converting to SDL ticks.
	b := &drmBackend{fd: fd, path: path, pipe: pipe}
	t.Logf("using %s crtc %d (%s)", path, pipe.index, pipe)

	seq, _, err := b.query(requestType(b.pipe.index)|drmVblankRelative, 0)
	if err != nil {
		t.Fatalf("relative query: %v", err)
	}

	want := seq + 1
	// Two frames at 24 Hz. The production budget is deliberately shorter than a
	// frame — it corrects a microsecond-scale race, not a wait for the display —
	// but this test starts cold, so the next vblank really can be a frame away.
	const budget uint64 = 84_000_000
	gotSeq, gotTS, waited, ok := b.pollForSequence(want, budget)
	if !ok {
		t.Fatalf("poll for sequence %d gave up after %.3f ms; the display should "+
			"produce a vblank well inside the budget", want, float64(waited)/1e6)
	}
	if int32(gotSeq-want) < 0 {
		t.Errorf("poll returned sequence %d, which is before the requested %d", gotSeq, want)
	}
	if gotTS == 0 {
		t.Error("poll returned a zero timestamp, which collides with the unset sentinel")
	}
	if waited >= budget {
		t.Errorf("poll took %.3f ms, at or beyond the %.3f ms budget",
			float64(waited)/1e6, float64(budget)/1e6)
	}
	t.Logf("resolved vblank %d (wanted %d) after %.3f ms", gotSeq, want, float64(waited)/1e6)
}

// TestExpectedSeq pins which vblank a frame is resolved to, given where its
// query landed relative to the vblank grid.
//
// The two-frame cases are the regression: the count used to be predicted as
// lastSeq+1 whatever the elapsed time, so a two-frame hold whose query beat the
// IRQ read lastSeq+1, saw the count advance, and accepted a vblank one frame
// early.
func TestExpectedSeq(t *testing.T) {
	const (
		p60    = 16_666_667 // 60 Hz
		p360   = 2_777_778  // 360 Hz: half a frame is under the poll budget
		last   = 1000
		lastTS = 5_000_000_000
		budget = drmSeqPollBudgetNS
	)
	cases := []struct {
		name   string
		now    uint64
		period uint64
		want   uint32
	}{
		{"one frame, query beat the IRQ", lastTS + p60 - 10_000, p60, last + 1},
		{"one frame, query after the IRQ", lastTS + p60 + 300_000, p60, last + 1},
		{"two frames, query beat the IRQ", lastTS + 2*p60 - 10_000, p60, last + 2},
		{"two frames, query after the IRQ", lastTS + 2*p60 + 300_000, p60, last + 2},
		{"five frames late", lastTS + 5*p60 + 1_000_000, p60, last + 5},
		{"called again inside the same frame", lastTS + p60/3, p60, last + 1},
		{"360 Hz, exactly on a boundary", lastTS + 3*p360, p360, last + 3},
		{"360 Hz, just before a boundary", lastTS + 3*p360 - 50_000, p360, last + 3},
		{"no period known", lastTS + 2*p60, 0, last + 1},
		{"clock before the last stamp", lastTS - 1, p60, last + 1},
	}
	for _, c := range cases {
		if got := expectedSeq(last, lastTS, c.now, c.period, budget); got != c.want {
			t.Errorf("%s: expectedSeq = %d, want %d", c.name, got, c.want)
		}
	}
	// The counter wraps; the prediction must wrap with it.
	if got := expectedSeq(^uint32(0), lastTS, lastTS+2*p60, p60, budget); got != 1 {
		t.Errorf("across the wrap: expectedSeq = %d, want 1", got)
	}
}

// TestToTicksTracksAMovingOffset checks that the CLOCK_MONOTONIC -> SDL ticks
// conversion uses the offset as it is now, not as it was at construction.
//
// The two clocks run at different rates (SDL is on CLOCK_MONOTONIC_RAW), so
// their offset moves through a run. A fake SDL clock stands in for that here,
// with its offset stepped between conversions.
func TestToTicksTracksAMovingOffset(t *testing.T) {
	var lag uint64 = 3_000_000_000
	b := &drmBackend{ticks: func() uint64 {
		m, err := monotonicNS()
		if err != nil {
			t.Fatalf("monotonicNS: %v", err)
		}
		return m - lag
	}}
	stamp, err := monotonicNS()
	if err != nil {
		t.Fatalf("monotonicNS: %v", err)
	}
	// A clean offset sample is far under this; a preempted one is retried, so
	// even a loaded test machine should not reach it.
	const tolerance = 100_000

	check := func(label string) {
		t.Helper()
		got, err := b.toTicks(stamp)
		if err != nil {
			t.Fatalf("%s: toTicks: %v", label, err)
		}
		if want := stamp - lag; got+tolerance < want || got > want+tolerance {
			t.Errorf("%s: toTicks = %d, want %d +- %d ns", label, got, want, tolerance)
		}
	}
	check("initial offset")
	lag += 1_000_000 // the clocks have drifted 1 ms apart
	check("after the offset moved")
}
