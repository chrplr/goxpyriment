// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package present

import "testing"

func TestFallbackReturnsFlipTSImmediately(t *testing.T) {
	tm := NewFallback()
	defer tm.Close()
	const ts uint64 = 1_234_567_890
	got, src, ok := tm.OnsetForFlip(ts)
	if !ok {
		t.Fatal("fallback OnsetForFlip should always be ok=true")
	}
	if got != ts {
		t.Errorf("fallback should echo flipTS, got %d want %d", got, ts)
	}
	if src != VsyncEstimated {
		t.Errorf("fallback source should be VsyncEstimated, got %v", src)
	}
}

func TestFallbackPrecision(t *testing.T) {
	if got := NewFallback().Precision(); got != VsyncEstimated {
		t.Errorf("fallback Precision should be VsyncEstimated, got %v", got)
	}
}

func TestFallbackRecordFlipNoop(t *testing.T) {
	tm := NewFallback()
	defer tm.Close()
	// Should not panic; should not change behaviour.
	tm.RecordFlip(42)
	got, _, _ := tm.OnsetForFlip(99)
	if got != 99 {
		t.Errorf("fallback OnsetForFlip should still echo its argument, got %d", got)
	}
}

func TestFallbackCloseIdempotent(t *testing.T) {
	tm := NewFallback()
	if err := tm.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestOnsetSourceString(t *testing.T) {
	cases := map[OnsetSource]string{
		VsyncEstimated:   "vsync-estimated",
		HardwareVerified: "hardware-verified",
		LookAhead:        "look-ahead",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", s, got, want)
		}
	}
}
