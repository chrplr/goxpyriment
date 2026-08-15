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
	fd, path, crtc, err := findDRMNode()
	if err != nil {
		t.Skipf("no DRM vblank source here: %v", err)
	}
	defer syscall.Close(fd) //nolint:errcheck
	// Built directly rather than through newBackendOn, which needs SDL loaded to
	// capture the clock epoch. Only the ioctl path is under test, and a zero
	// offset leaves timestamps in CLOCK_MONOTONIC, which is what the poll
	// measures against anyway.
	b := &drmBackend{fd: fd, path: path, crtc: crtc}
	t.Logf("using %s crtc %d", path, crtc)

	seq, _, err := b.query(requestType(b.crtc)|drmVblankRelative, 0)
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
