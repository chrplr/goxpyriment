// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

// newFakeMMBTS returns an MMBTS wired to the in-memory fakePort defined in
// dlpio20_test.go, bypassing NewMMBTS so no hardware is needed. The box never
// replies, so no canned reads are set up.
func newFakeMMBTS(mode MMBTSMode) (*MMBTS, *fakePort) {
	p := &fakePort{}
	b := &MMBTS{
		port:       p,
		mode:       mode,
		pulseWidth: mmbtsFirmwarePulse,
	}
	return b, p
}

func TestMMBTSSendWritesOneByte(t *testing.T) {
	b, p := newFakeMMBTS(MMBTSSimpleMode)

	if err := b.Send(0xA5); err != nil {
		t.Fatalf("Send(0xA5): %v", err)
	}
	if got, want := p.written.Bytes(), []byte{0xA5}; !bytes.Equal(got, want) {
		t.Errorf("Send wrote % X, want % X", got, want)
	}
}

// In Simple mode the byte latches, so the shadow mask must accumulate across
// calls: the box has no per-line command, and each write replaces all 8 lines.
func TestMMBTSSimpleModeAccumulatesMask(t *testing.T) {
	b, p := newFakeMMBTS(MMBTSSimpleMode)

	for _, step := range []struct {
		do   func() error
		want byte
	}{
		{func() error { return b.SetHigh(0) }, 0x01},
		{func() error { return b.SetHigh(7) }, 0x81},
		{func() error { return b.SetLow(0) }, 0x80},
		{func() error { return b.AllLow() }, 0x00},
	} {
		if err := step.do(); err != nil {
			t.Fatalf("step to 0x%02X: %v", step.want, err)
		}
	}
	if got, want := p.written.Bytes(), []byte{0x01, 0x81, 0x80, 0x00}; !bytes.Equal(got, want) {
		t.Errorf("wire = % X, want % X", got, want)
	}
}

// In Pulse mode the firmware clears the port 8 ms after each byte. A shadow
// older than that describes lines that are already LOW, and reusing it would
// re-raise a line the recording has already seen fall.
func TestMMBTSPulseModeShadowExpires(t *testing.T) {
	b, p := newFakeMMBTS(MMBTSPulseMode)

	// Within the window the previous bit is still up, so it is preserved.
	b.mask, b.maskAt = 0x01, time.Now()
	if err := b.SetHigh(1); err != nil {
		t.Fatalf("SetHigh(1): %v", err)
	}
	if got, want := p.written.Bytes(), []byte{0x03}; !bytes.Equal(got, want) {
		t.Errorf("within the pulse window: wrote % X, want % X", got, want)
	}

	// Past the window the firmware has cleared the port, so only the new bit
	// may be raised.
	p.written.Reset()
	b.mask, b.maskAt = 0x01, time.Now().Add(-20*time.Millisecond)
	if err := b.SetHigh(1); err != nil {
		t.Fatalf("SetHigh(1): %v", err)
	}
	if got, want := p.written.Bytes(), []byte{0x02}; !bytes.Equal(got, want) {
		t.Errorf("after the pulse window: wrote % X, want % X", got, want)
	}
}

func TestMMBTSLineRangeChecks(t *testing.T) {
	b, p := newFakeMMBTS(MMBTSSimpleMode)

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"SetHigh(8)", b.SetHigh(8)},
		{"SetHigh(-1)", b.SetHigh(-1)},
		{"SetLow(8)", b.SetLow(8)},
		{"SetLow(-1)", b.SetLow(-1)},
		{"Pulse(8)", b.Pulse(8, time.Millisecond)},
	} {
		if !errors.Is(tc.err, ErrMMBTSBadLine) {
			t.Errorf("%s = %v, want ErrMMBTSBadLine", tc.name, tc.err)
		}
	}
	// An out-of-range line must not put anything on the wire: a stray byte is
	// a trigger code the recording cannot be told to ignore.
	if n := p.written.Len(); n != 0 {
		t.Errorf("out-of-range calls wrote %d bytes (% X), want none", n, p.written.Bytes())
	}
}

func TestMMBTSPulseSimpleModeWritesBothEdges(t *testing.T) {
	b, p := newFakeMMBTS(MMBTSSimpleMode)

	if err := b.Pulse(2, time.Millisecond); err != nil {
		t.Fatalf("Pulse(2): %v", err)
	}
	if got, want := p.written.Bytes(), []byte{0x04, 0x00}; !bytes.Equal(got, want) {
		t.Errorf("Pulse wrote % X, want % X", got, want)
	}
}

// In Pulse mode the firmware times the fall, so only the raise goes on the
// wire — but the call must still block for the width it cannot shorten, or the
// next write would race the automatic clear.
func TestMMBTSPulsePulseModeWritesOnlyTheRaise(t *testing.T) {
	b, p := newFakeMMBTS(MMBTSPulseMode)

	start := time.Now()
	if err := b.Pulse(2, time.Millisecond); err != nil {
		t.Fatalf("Pulse(2): %v", err)
	}
	elapsed := time.Since(start)

	if got, want := p.written.Bytes(), []byte{0x04}; !bytes.Equal(got, want) {
		t.Errorf("Pulse wrote % X, want % X", got, want)
	}
	if elapsed < mmbtsFirmwarePulse {
		t.Errorf("Pulse blocked for %v, want at least the firmware width %v", elapsed, mmbtsFirmwarePulse)
	}
}

func TestMMBTSCloseIsIdempotent(t *testing.T) {
	b, p := newFakeMMBTS(MMBTSSimpleMode)

	if err := b.SetHigh(3); err != nil {
		t.Fatalf("SetHigh(3): %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got, want := p.written.Bytes(), []byte{0x08, 0x00}; !bytes.Equal(got, want) {
		t.Errorf("after Close the wire = % X, want % X (Close must drive the lines LOW)", got, want)
	}

	p.written.Reset()
	if err := b.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if n := p.written.Len(); n != 0 {
		t.Errorf("second Close wrote %d bytes, want none", n)
	}
	if err := b.Send(0x01); !errors.Is(err, ErrMMBTSNotOpen) {
		t.Errorf("Send after Close = %v, want ErrMMBTSNotOpen", err)
	}
}

func TestMMBTSModeString(t *testing.T) {
	if got := MMBTSPulseMode.String(); got != "pulse" {
		t.Errorf("MMBTSPulseMode = %q, want %q", got, "pulse")
	}
	if got := MMBTSSimpleMode.String(); got != "simple" {
		t.Errorf("MMBTSSimpleMode = %q, want %q", got, "simple")
	}
}
