// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package media

import (
	"fmt"
	"testing"
	"time"
)

// fakeTimer is a hardware-verified present timer whose vsync timestamps are
// published by the test: a flip is ready once stamps[flipTS] is set.
type fakeTimer struct {
	stamps map[uint64]uint64
	all    bool // every flip is ready, stamped with its own flipTS + 1
}

func (f *fakeTimer) RecordFlip(uint64) {}
func (f *fakeTimer) OnsetForFlip(flipTS uint64) (uint64, OnsetSource, bool) {
	if f.all {
		return flipTS + 1, HardwareVerified, true
	}
	ts, ok := f.stamps[flipTS]
	return ts, HardwareVerified, ok
}
func (f *fakeTimer) Precision() OnsetSource { return HardwareVerified }
func (f *fakeTimer) Close() error           { return nil }
func (f *fakeTimer) Description() string    { return "fake" }

const testFrame = 10 * time.Millisecond

// newTestManager builds a manager with no screen. now is the SDL clock the
// stale check reads, in ns.
func newTestManager(timer *fakeTimer, now *uint64) *MovieManager {
	return &MovieManager{
		clock:           NewMasterClock(),
		presentTimer:    timer,
		extStimuli:      make(map[string]func() bool),
		onsetCallbacks:  make(map[string][]onsetCB),
		offsetCallbacks: make(map[string][]onsetCB),
		lastVisible:     make(map[string]bool),
		pendingVisible:  make(map[string]bool),
		ticksNS:         func() uint64 { return *now },
		frameDuration:   func() time.Duration { return testFrame },
	}
}

// recorder logs each callback as "kind@ts/source".
type recorder struct{ got []string }

func (r *recorder) cb(kind string) func(Onset) {
	return func(o Onset) { r.got = append(r.got, fmt.Sprintf("%s@%d/%v", kind, o.TimestampNS, o.Source)) }
}

// flip draws (so the visibility predicates are read) and notifies.
func flip(t *testing.T, mgr *MovieManager, now *uint64, ts uint64) {
	t.Helper()
	if err := mgr.DrawWithoutFlip(); err != nil {
		t.Fatal(err)
	}
	*now = ts
	mgr.NotifyFlipped(ts)
}

func setupOnsetOffset(t *testing.T, timer *fakeTimer) (*MovieManager, *recorder, *bool, *uint64) {
	now := new(uint64)
	mgr := newTestManager(timer, now)
	rec := &recorder{}
	visible := new(bool)
	mgr.RegisterStimulusName("S", func() bool { return *visible })
	mgr.OnDisplayOnset("S", rec.cb("onset"))
	mgr.OnDisplayOffset("S", rec.cb("offset"))
	return mgr, rec, visible, now
}

func expect(t *testing.T, rec *recorder, want ...string) {
	t.Helper()
	if fmt.Sprint(rec.got) != fmt.Sprint(want) {
		t.Fatalf("callbacks = %v, want %v", rec.got, want)
	}
}

// The onset's flip waits for its stamp; the offset's flip, one frame later,
// already has its own. The offset must not fire first.
func TestNotifyFlippedKeepsFlipOrder(t *testing.T) {
	timer := &fakeTimer{stamps: map[uint64]uint64{}}
	mgr, rec, visible, now := setupOnsetOffset(t, timer)

	*visible = true
	flip(t, mgr, now, 1000)
	expect(t, rec) // stamp for 1000 not in yet

	// Both stamps arrive before the next flip.
	timer.stamps[1000] = 1500
	timer.stamps[2000] = 2500
	*visible = false
	flip(t, mgr, now, 2000)
	expect(t, rec, "onset@1500/hardware-verified", "offset@2500/hardware-verified")
}

// Only the later flip's stamp ever arrives: the earlier flip falls back to
// its FlipTS at once, still ahead of the later one.
func TestNotifyFlippedLaterStampReleasesEarlierFlip(t *testing.T) {
	timer := &fakeTimer{stamps: map[uint64]uint64{}}
	mgr, rec, visible, now := setupOnsetOffset(t, timer)

	*visible = true
	flip(t, mgr, now, 1000)
	timer.stamps[2000] = 2500
	*visible = false
	flip(t, mgr, now, 2000)
	expect(t, rec, "onset@1000/vsync-estimated", "offset@2500/hardware-verified")
}

// A flip whose stamp never arrives fires once it is staleFlipMultiplier
// frames old, and the flips queued behind it follow in order.
func TestNotifyFlippedStaleFlipFallsBack(t *testing.T) {
	timer := &fakeTimer{stamps: map[uint64]uint64{}}
	mgr, rec, visible, now := setupOnsetOffset(t, timer)

	*visible = true
	flip(t, mgr, now, 1000)
	*visible = false
	flip(t, mgr, now, 1000+uint64(testFrame)) // nothing stamped, nothing stale
	expect(t, rec)

	// A flip with no callbacks, late enough that both queued flips are stale.
	flip(t, mgr, now, 1000+uint64(10*testFrame))
	off := 1000 + uint64(testFrame)
	expect(t, rec, "onset@1000/vsync-estimated", fmt.Sprintf("offset@%d/vsync-estimated", off))
}

// Close fires what is still waiting instead of dropping it.
func TestCloseFiresPendingFlips(t *testing.T) {
	timer := &fakeTimer{stamps: map[uint64]uint64{}}
	mgr, rec, visible, now := setupOnsetOffset(t, timer)

	*visible = true
	flip(t, mgr, now, 1000)
	*visible = false
	flip(t, mgr, now, 1001) // still inside the wait window
	expect(t, rec)

	timer.stamps[1000] = 1500 // arrives in time for Close; 1001's never does
	if err := mgr.Close(); err != nil {
		t.Fatal(err)
	}
	expect(t, rec, "onset@1500/hardware-verified", "offset@1001/vsync-estimated")

	if err := mgr.Close(); err != nil { // idempotent, nothing fires twice
		t.Fatal(err)
	}
	expect(t, rec, "onset@1500/hardware-verified", "offset@1001/vsync-estimated")
}

// testMovie is a Movie with no file behind it: enough for the condition and
// NotifyFlipped logic, which never decode.
func testMovie(mgr *MovieManager, opts ...Option) *Movie {
	m := &Movie{mgr: mgr, fps: 30, fcount: 300, rate: 1, repeatCount: 1}
	for _, o := range opts {
		o(m)
	}
	mgr.add(m)
	return m
}

// OnAtDisplay(Frame(0)) must not fire for a movie that is not playing, and
// must fire once after Play.
func TestOnAtDisplayIgnoresInactiveMovie(t *testing.T) {
	now := new(uint64)
	mgr := newTestManager(&fakeTimer{all: true}, now)
	m := testMovie(mgr)
	n := 0
	m.OnAtDisplay(Frame(0), func(Onset) { n++ })

	*now = 1000
	mgr.NotifyFlipped(1000)
	mgr.NotifyFlipped(2000)
	if n != 0 {
		t.Fatalf("fired %d times before Play, want 0", n)
	}
	m.Play()
	mgr.NotifyFlipped(3000)
	mgr.NotifyFlipped(4000)
	if n != 1 {
		t.Fatalf("fired %d times after Play, want 1", n)
	}
}

// Starting or seeking past a target skips it; targets at or after the jump
// stay armed.
func TestJumpSkipsTargetsBeforeIt(t *testing.T) {
	type cond struct {
		name string
		t    Target
		skip bool
	}
	// At 30 fps, 1 s lands on frame 31.
	conds := []cond{
		{"Frame(0)", Frame(0), true},
		{"Frame(1)", Frame(1), true},
		{"Frame(30)", Frame(30), true},
		{"Frame(31)", Frame(31), false},
		{"Frame(40)", Frame(40), false},
		{"AtTime(999ms)", AtTime(999 * time.Millisecond), true},
		{"AtTime(1s)", AtTime(time.Second), false},
		{"Done", Done{}, false},
	}
	check := func(t *testing.T, m *Movie, wantSkip func(cond) bool) {
		t.Helper()
		for i, c := range conds {
			if got := m.conditions[i].fired; got != wantSkip(c) {
				t.Errorf("%s: fired=%v, want %v", c.name, got, wantSkip(c))
			}
		}
	}
	arm := func(m *Movie) {
		for _, c := range conds {
			m.OnAt(c.t, func(Onset) {})
		}
	}
	skipped := func(c cond) bool { return c.skip }
	none := func(cond) bool { return false }

	t.Run("WithFromTime", func(t *testing.T) {
		m := testMovie(newTestManager(&fakeTimer{all: true}, new(uint64)), WithFromTime(time.Second))
		arm(m)
		m.Play()
		check(t, m, skipped)
	})
	t.Run("SeekTime", func(t *testing.T) {
		m := testMovie(newTestManager(&fakeTimer{all: true}, new(uint64)))
		arm(m)
		m.Play()
		check(t, m, none)
		if err := m.SeekTime(time.Second); err != nil {
			t.Fatal(err)
		}
		check(t, m, skipped)
	})
	t.Run("SeekFrame", func(t *testing.T) {
		m := testMovie(newTestManager(&fakeTimer{all: true}, new(uint64)))
		arm(m)
		m.Play()
		if err := m.SeekFrame(31); err != nil {
			t.Fatal(err)
		}
		check(t, m, skipped)
	})
	t.Run("SeekFrame lands on frame n", func(t *testing.T) {
		m := testMovie(newTestManager(&fakeTimer{all: true}, new(uint64)))
		for n := 2; n <= 300; n++ {
			m.conditions = nil
			m.OnAt(Frame(n-1), func(Onset) {})
			m.OnAt(Frame(n), func(Onset) {})
			if err := m.SeekFrame(n); err != nil {
				t.Fatal(err)
			}
			if !m.conditions[0].fired || m.conditions[1].fired {
				t.Fatalf("SeekFrame(%d): Frame(%d) fired=%v, Frame(%d) fired=%v; want true, false",
					n, n-1, m.conditions[0].fired, n, m.conditions[1].fired)
			}
		}
	})
}
