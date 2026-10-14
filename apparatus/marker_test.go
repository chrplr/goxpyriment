// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// The Marker's state machine and its TTL clearing are tested here without
// SDL: beginFrame/afterFlip are what Update calls around the present, and the
// output is a fake that records every write. Drawing the square needs a
// renderer and is checked by tests/test_onset_marker.

type write struct {
	code byte
	at   time.Time
}

type fakeOutput struct {
	mu      sync.Mutex
	writes  []write
	allLow  int
	closed  int
	failing bool
}

func (f *fakeOutput) Send(code byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failing {
		return errors.New("device unplugged")
	}
	f.writes = append(f.writes, write{code, time.Now()})
	return nil
}
func (f *fakeOutput) AllLow() error { f.mu.Lock(); f.allLow++; f.mu.Unlock(); return nil }
func (f *fakeOutput) Close() error  { f.mu.Lock(); f.closed++; f.mu.Unlock(); return nil }

func (f *fakeOutput) snapshot() []write {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]write(nil), f.writes...)
}

// flip runs one Update's worth of marker hooks and reports whether the square
// was white in that frame.
func flip(m *Marker) bool {
	white := m.beginFrame()
	m.afterFlip(uint64(time.Now().UnixNano()))
	return white
}

func init() { markerClockNS = func() uint64 { return uint64(time.Now().UnixNano()) } }

func TestMarkerSquareIsWhiteForFramesThenBlack(t *testing.T) {
	m := NewMarker(MarkerOptions{Photodiode: true, Size: 100, Frames: 3})
	if flip(m) {
		t.Fatal("square white before any onset")
	}
	m.Arm(0)
	want := []bool{true, true, true, false, false}
	for i, w := range want {
		if got := flip(m); got != w {
			t.Errorf("frame %d after Arm: white=%v, want %v", i, got, w)
		}
	}
	if m.Stats().Onsets != 1 {
		t.Errorf("Onsets = %d, want 1", m.Stats().Onsets)
	}
}

// Busy must stay true until a black frame has been presented: N white flips
// plus one, or a presentation that stops flipping leaves the square white.
func TestMarkerBusyCoversTheBlackFrame(t *testing.T) {
	m := NewMarker(MarkerOptions{Photodiode: true, Size: 100, Frames: 3})
	m.Arm(0)
	n := 0
	for m.Busy() {
		flip(m)
		n++
		if n > 10 {
			t.Fatal("Busy never went false")
		}
	}
	if n != 4 {
		t.Errorf("Busy for %d flips, want 4 (3 white + 1 black)", n)
	}
}

func TestMarkerRearmWhileWhiteRestartsAndCounts(t *testing.T) {
	m := NewMarker(MarkerOptions{Photodiode: true, Size: 100, Frames: 3})
	m.Arm(0)
	flip(m)
	flip(m)
	m.Arm(0) // one white frame left: the flashes merge
	for i := 0; i < 3; i++ {
		if !flip(m) {
			t.Fatalf("frame %d after the re-arm is black; want 3 white frames", i)
		}
	}
	if flip(m) {
		t.Error("still white 4 frames after the re-arm")
	}
	if got := m.Stats().Merged; got != 1 {
		t.Errorf("Merged = %d, want 1", got)
	}
}

func TestMarkerTTLOnlyNeverBusy(t *testing.T) {
	out := &fakeOutput{}
	m := NewMarker(MarkerOptions{Output: out, TTLDuration: time.Hour})
	defer m.Close()
	m.Arm(5)
	if m.Busy() {
		t.Error("Busy with no photodiode square: presentations would hold for nothing")
	}
	flip(m)
	if w := out.snapshot(); len(w) != 1 || w[0].code != 5 {
		t.Errorf("writes = %v, want the code 5 once", w)
	}
}

func TestMarkerArmZeroWritesNothing(t *testing.T) {
	out := &fakeOutput{}
	m := NewMarker(MarkerOptions{Photodiode: true, Size: 10, Output: out, TTLDuration: time.Hour})
	defer m.Close()
	m.Arm(0)
	flip(m)
	if w := out.snapshot(); len(w) != 0 {
		t.Errorf("Arm(0) wrote %v; want the square only", w)
	}
}

func TestMarkerCodeIsWrittenOnlyOnTheArmedFlip(t *testing.T) {
	out := &fakeOutput{}
	m := NewMarker(MarkerOptions{Photodiode: true, Size: 10, Output: out, TTLDuration: time.Hour})
	defer m.Close()
	m.Arm(7)
	for i := 0; i < 5; i++ {
		flip(m)
	}
	if w := out.snapshot(); len(w) != 1 {
		t.Errorf("%d writes over 5 flips, want 1 (the onset only)", len(w))
	}
	if s := m.Stats(); s.Codes != 1 || s.Onsets != 1 {
		t.Errorf("stats = %+v", s)
	}
}

func TestMarkerClearsAfterTTLDuration(t *testing.T) {
	out := &fakeOutput{}
	const dur = 20 * time.Millisecond
	m := NewMarker(MarkerOptions{Output: out, TTLDuration: dur})
	defer m.Close()
	m.Arm(3)
	flip(m)
	w := waitForWrites(t, out, 2)
	if w[1].code != 0 {
		t.Fatalf("second write = %d, want the clear (0)", w[1].code)
	}
	if gap := w[1].at.Sub(w[0].at); gap < dur {
		t.Errorf("cleared %v after the code, want >= %v", gap, dur)
	}
	if m.Stats().Clears != 1 {
		t.Errorf("Clears = %d, want 1", m.Stats().Clears)
	}
}

// A second code before the first is cleared must push the clear later, not
// be cut short by the first one's deadline.
func TestMarkerSecondCodePostponesTheClear(t *testing.T) {
	out := &fakeOutput{}
	const dur = 40 * time.Millisecond
	m := NewMarker(MarkerOptions{Output: out, TTLDuration: dur})
	defer m.Close()
	m.Fire(1)
	time.Sleep(dur / 2)
	m.Fire(2)
	w := waitForWrites(t, out, 3)
	if w[0].code != 1 || w[1].code != 2 || w[2].code != 0 {
		t.Fatalf("writes = %v, want 1, 2, then 0", w)
	}
	if gap := w[2].at.Sub(w[1].at); gap < dur {
		t.Errorf("cleared %v after the second code, want >= %v", gap, dur)
	}
	if s := m.Stats(); s.Overlaps != 1 || s.Fires != 2 {
		t.Errorf("stats = %+v, want 1 overlap and 2 fires", s)
	}
}

func TestMarkerFireWithoutOutputIsANoop(t *testing.T) {
	m := NewMarker(MarkerOptions{Photodiode: true, Size: 10})
	if ts := m.Fire(9); ts != 0 {
		t.Errorf("Fire with no output returned %d, want 0", ts)
	}
	if m.Busy() {
		t.Error("Fire armed the square; it should only write the code")
	}
}

func TestMarkerWriteErrorsAreCountedNotFatal(t *testing.T) {
	out := &fakeOutput{failing: true}
	m := NewMarker(MarkerOptions{Output: out, TTLDuration: time.Hour})
	defer m.Close()
	m.Fire(1)
	m.Arm(2)
	flip(m)
	if got := m.Stats().WriteErrors; got != 2 {
		t.Errorf("WriteErrors = %d, want 2", got)
	}
}

func TestMarkerCloseIsIdempotentAndDropsLaterWrites(t *testing.T) {
	out := &fakeOutput{}
	m := NewMarker(MarkerOptions{Output: out, TTLDuration: time.Hour})
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if out.allLow != 1 || out.closed != 1 {
		t.Errorf("AllLow %d, Close %d; want 1 each", out.allLow, out.closed)
	}
	m.Fire(4)
	if w := out.snapshot(); len(w) != 0 {
		t.Errorf("wrote %v after Close", w)
	}
}

func TestMarkerNilIsSafe(t *testing.T) {
	var m *Marker
	m.Arm(1)
	if m.Busy() || m.Fire(1) != 0 || m.TTLEnabled() || m.PhotodiodeEnabled() || m.Close() != nil {
		t.Error("nil Marker did something")
	}
	_ = m.Stats()
}

// The hooks run on every frame, inside timing loops that switch the garbage
// collector off: they must not allocate.
func TestMarkerHooksDoNotAllocate(t *testing.T) {
	out := &fakeOutput{}
	m := NewMarker(MarkerOptions{Photodiode: true, Size: 10, Output: out, TTLDuration: time.Hour})
	defer m.Close()
	out.writes = make([]write, 0, 1000) // the fake's own log must not count
	allocs := testing.AllocsPerRun(200, func() {
		m.Arm(1)
		flip(m)
		flip(m)
	})
	if allocs != 0 {
		t.Errorf("%.1f allocations per armed flip, want 0", allocs)
	}
}

func waitForWrites(t *testing.T, out *fakeOutput, n int) []write {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if w := out.snapshot(); len(w) >= n {
			return w
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("got %d writes, want %d: %v", len(out.snapshot()), n, out.snapshot())
	return nil
}

// lineOutput is a device that can also drive one line on its own, like every
// triggers.OutputTTLDevice; it records which call each write went through.
type lineOutput struct {
	mu  sync.Mutex
	ops []string
}

func (l *lineOutput) record(op string) error {
	l.mu.Lock()
	l.ops = append(l.ops, op)
	l.mu.Unlock()
	return nil
}
func (l *lineOutput) Send(code byte) error   { return l.record(fmt.Sprintf("Send(%d)", code)) }
func (l *lineOutput) SetHigh(line int) error { return l.record(fmt.Sprintf("SetHigh(%d)", line)) }
func (l *lineOutput) SetLow(line int) error  { return l.record(fmt.Sprintf("SetLow(%d)", line)) }
func (l *lineOutput) AllLow() error          { return nil }
func (l *lineOutput) Close() error           { return nil }

func (l *lineOutput) waitOps(t *testing.T, n int) []string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		l.mu.Lock()
		ops := append([]string(nil), l.ops...)
		l.mu.Unlock()
		if len(ops) >= n {
			return ops
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("fewer than %d writes: %v", n, l.ops)
	return nil
}

// A single-bit code goes out as one SetHigh and comes back with one SetLow:
// on the DLP boxes Send writes the eight lines one command at a time, and a
// recording can latch the half-written code.
func TestMarkerSingleBitCodeUsesOneLine(t *testing.T) {
	out := &lineOutput{}
	m := NewMarker(MarkerOptions{Output: out, TTLDuration: 5 * time.Millisecond})
	defer m.Close()
	m.Fire(4)
	if got := fmt.Sprint(out.waitOps(t, 2)); got != "[SetHigh(2) SetLow(2)]" {
		t.Errorf("writes = %s, want [SetHigh(2) SetLow(2)]", got)
	}
}

func TestMarkerMultiBitCodeUsesSend(t *testing.T) {
	out := &lineOutput{}
	m := NewMarker(MarkerOptions{Output: out, TTLDuration: 5 * time.Millisecond})
	defer m.Close()
	m.Fire(6)
	if got := fmt.Sprint(out.waitOps(t, 2)); got != "[Send(6) Send(0)]" {
		t.Errorf("writes = %s, want [Send(6) Send(0)]", got)
	}
}

// A single-bit code over another one still on the lines must replace it, which
// SetHigh alone would not: it goes through Send, and the clear lowers the one
// line Send left high.
func TestMarkerSingleBitOverlapReplacesTheCode(t *testing.T) {
	out := &lineOutput{}
	m := NewMarker(MarkerOptions{Output: out, TTLDuration: 20 * time.Millisecond})
	defer m.Close()
	m.Fire(1)
	m.Fire(2)
	if got := fmt.Sprint(out.waitOps(t, 3)); got != "[SetHigh(0) Send(2) SetLow(1)]" {
		t.Errorf("writes = %s, want [SetHigh(0) Send(2) SetLow(1)]", got)
	}
}
