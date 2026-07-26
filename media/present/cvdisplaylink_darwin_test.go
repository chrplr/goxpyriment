// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build darwin

package present

import "testing"

// TestCVAPILoadable confirms purego can dlopen CoreVideo + libSystem
// and bind every symbol we need. If this fails, every CVDisplayLink-
// based feature also fails — and AutoDetect falls back to
// vsync-estimated.
func TestCVAPILoadable(t *testing.T) {
	if err := loadCVAPI(); err != nil {
		t.Fatalf("loadCVAPI: %v", err)
	}
	if cvAPI.MachTimebaseInfo == nil {
		t.Fatal("mach_timebase_info not bound")
	}
	if cvAPI.MachAbsoluteTime == nil {
		t.Fatal("mach_absolute_time not bound")
	}
	if cvAPI.CVDisplayLinkCreateWithActiveCGDisplays == nil {
		t.Fatal("CVDisplayLinkCreateWithActiveCGDisplays not bound")
	}
	if cvAPI.CVDisplayLinkSetOutputCallback == nil {
		t.Fatal("CVDisplayLinkSetOutputCallback not bound")
	}
}

func TestMachTimebaseInfoSane(t *testing.T) {
	if err := loadCVAPI(); err != nil {
		t.Fatalf("loadCVAPI: %v", err)
	}
	var tb machTimebase
	if rc := cvAPI.MachTimebaseInfo(&tb); rc != 0 {
		t.Fatalf("mach_timebase_info returned %d", rc)
	}
	if tb.Numer == 0 || tb.Denom == 0 {
		t.Fatalf("mach_timebase_info: numer=%d denom=%d", tb.Numer, tb.Denom)
	}
	t.Logf("mach timebase: %d/%d", tb.Numer, tb.Denom)
}

func TestMachAbsoluteTimeMonotonic(t *testing.T) {
	if err := loadCVAPI(); err != nil {
		t.Fatalf("loadCVAPI: %v", err)
	}
	a := cvAPI.MachAbsoluteTime()
	b := cvAPI.MachAbsoluteTime()
	if a == 0 {
		t.Error("mach_absolute_time returned 0")
	}
	if b < a {
		t.Errorf("mach_absolute_time should be monotonic, got %d then %d", a, b)
	}
}

func TestMachToNS(t *testing.T) {
	cases := []struct {
		mach, numer, denom, want uint64
	}{
		{1234567, 1, 1, 1234567},   // identity ratio
		{3, 125, 3, 125},           // typical Apple Silicon ratio
		{6, 125, 3, 250},           // doubled
		{0, 125, 3, 0},             // zero in
		{1_000_000_000, 1, 1, 1e9}, // 1 second
	}
	for _, c := range cases {
		got := machToNS(c.mach, c.numer, c.denom)
		if got != c.want {
			t.Errorf("machToNS(%d, %d, %d) = %d, want %d",
				c.mach, c.numer, c.denom, got, c.want)
		}
	}
}

func TestMachToNSNoOverflow(t *testing.T) {
	// At ~2e17 mach units, mach * 125 would overflow uint64 if the
	// multiply happened before the divide. Verify our hi/lo split
	// handles it: the result must be nonzero, in range, and within a
	// few ns of the float-precision reference value.
	const big uint64 = 2e17
	got := machToNS(big, 125, 3)
	if got == 0 {
		t.Errorf("machToNS(%d, 125, 3) returned 0 (overflow?)", big)
	}
	// Exact result ≈ 8.33e18 — fits in uint64 (max 1.84e19). float64
	// has 52-bit mantissa, plenty of precision at this magnitude.
	wantApprox := uint64(float64(big) * 125.0 / 3.0)
	diff := int64(got) - int64(wantApprox)
	if diff < 0 {
		diff = -diff
	}
	if diff > 1000 {
		t.Errorf("machToNS(%d, 125, 3) = %d, want ~%d (diff=%d > 1000 ns tolerance)",
			big, got, wantApprox, diff)
	}
}
