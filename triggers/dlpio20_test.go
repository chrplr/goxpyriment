// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"bytes"
	"io"
	"testing"
	"time"

	"go.bug.st/serial"
)

// fakePort is an in-memory serial.Port: it records everything written and
// replays a canned response to reads. The DLP-IO20 driver is written from the
// datasheet and has never run against real hardware, so these tests pin the
// exact bytes it puts on the wire.
type fakePort struct {
	written bytes.Buffer
	replies []byte // consumed one byte per Read
	resets  int
}

func (p *fakePort) Write(b []byte) (int, error) { return p.written.Write(b) }

func (p *fakePort) Read(b []byte) (int, error) {
	if len(p.replies) == 0 {
		return 0, io.EOF
	}
	b[0] = p.replies[0]
	p.replies = p.replies[1:]
	return 1, nil
}

func (p *fakePort) ResetInputBuffer() error            { p.resets++; return nil }
func (p *fakePort) SetMode(*serial.Mode) error         { return nil }
func (p *fakePort) Drain() error                       { return nil }
func (p *fakePort) ResetOutputBuffer() error           { return nil }
func (p *fakePort) SetDTR(bool) error                  { return nil }
func (p *fakePort) SetRTS(bool) error                  { return nil }
func (p *fakePort) SetReadTimeout(time.Duration) error { return nil }
func (p *fakePort) Close() error                       { return nil }
func (p *fakePort) Break(time.Duration) error          { return nil }
func (p *fakePort) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return &serial.ModemStatusBits{}, nil
}

// newFakeIO20 returns a DLPIO20 wired to a fakePort, with the default windows.
func newFakeIO20() (*DLPIO20, *fakePort) {
	p := &fakePort{}
	d := &DLPIO20{
		port:         p,
		outputs:      io20DefaultOutputs,
		inputs:       io20DefaultInputs,
		pollInterval: io20DefaultPollInterval,
		readTimeout:  io20DefaultReadTimeout,
	}
	return d, p
}

// TestWritePacketFraming checks the length-prefix framing: byte 0 is the
// packet length including itself (datasheet §8, Table 2).
func TestWritePacketFraming(t *testing.T) {
	d, p := newFakeIO20()

	if err := d.writePacket(io20CmdPing); err != nil {
		t.Fatalf("writePacket(ping): %v", err)
	}
	if got, want := p.written.Bytes(), []byte{0x02, 0x27}; !bytes.Equal(got, want) {
		t.Errorf("ping packet = % X, want % X", got, want)
	}

	p.written.Reset()
	if err := d.writePacket(io20CmdDigitalIO, 0x02, io20DirOutput, io20ValHigh); err != nil {
		t.Fatalf("writePacket(digital): %v", err)
	}
	// The datasheet's own example: set AN2 as a digital output, HIGH.
	if got, want := p.written.Bytes(), []byte{0x05, 0x35, 0x02, 0x00, 0x01}; !bytes.Equal(got, want) {
		t.Errorf("digital packet = % X, want % X", got, want)
	}
}

func TestSetChannelPackets(t *testing.T) {
	for _, tc := range []struct {
		name string
		ch   IO20Channel
		high bool
		want []byte
	}{
		{"AN0 high", IO20_AN0, true, []byte{0x05, 0x35, 0x00, 0x00, 0x01}},
		{"AN0 low", IO20_AN0, false, []byte{0x05, 0x35, 0x00, 0x00, 0x00}},
		{"AN13 high", IO20_AN13, true, []byte{0x05, 0x35, 0x0D, 0x00, 0x01}},
		{"RA4 high", IO20_RA4, true, []byte{0x05, 0x35, 0x0E, 0x00, 0x01}},
		{"RB7 high", IO20_RB7, true, []byte{0x05, 0x35, 0x12, 0x00, 0x01}},
		{"RB6 low", IO20_RB6, false, []byte{0x05, 0x35, 0x13, 0x00, 0x00}},
	} {
		d, p := newFakeIO20()
		if err := d.SetChannel(tc.ch, tc.high); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := p.written.Bytes(); !bytes.Equal(got, tc.want) {
			t.Errorf("%s: wrote % X, want % X", tc.name, got, tc.want)
		}
	}
}

func TestSetChannelRejectsInvalid(t *testing.T) {
	d, _ := newFakeIO20()
	if err := d.SetChannel(IO20Channel(0x14), true); err == nil {
		t.Error("SetChannel(0x14): expected an error for an out-of-range channel")
	}
}

// TestReadChannelPacket checks that a read switches the channel to input mode
// (dir = 0x01) and still carries the unused value byte.
func TestReadChannelPacket(t *testing.T) {
	d, p := newFakeIO20()
	p.replies = []byte{0x01}

	v, err := d.ReadChannel(IO20_AN8)
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if v != 1 {
		t.Errorf("ReadChannel = %d, want 1", v)
	}
	if got, want := p.written.Bytes(), []byte{0x05, 0x35, 0x08, 0x01, 0x00}; !bytes.Equal(got, want) {
		t.Errorf("read packet = % X, want % X", got, want)
	}
	if p.resets == 0 {
		t.Error("ReadChannel should flush the input buffer before reading")
	}
}

func TestReadChannelRejectsRelayDrivers(t *testing.T) {
	for _, ch := range []IO20Channel{IO20_P5, IO20_P6, IO20_P7} {
		d, _ := newFakeIO20()
		if _, err := d.ReadChannel(ch); err == nil {
			t.Errorf("ReadChannel(%s): expected an error — relay drivers cannot be read", ch)
		}
	}
}

// TestSendOrdersLinesLowToHigh checks that Send walks the output window in line
// order and maps bit N to output channel N.
func TestSendOrdersLinesLowToHigh(t *testing.T) {
	d, p := newFakeIO20()
	if err := d.Send(0b00000101); err != nil {
		t.Fatalf("Send: %v", err)
	}

	got := p.written.Bytes()
	if len(got) != 8*5 {
		t.Fatalf("Send wrote %d bytes, want %d (8 packets of 5)", len(got), 8*5)
	}
	for line := 0; line < 8; line++ {
		pkt := got[line*5 : line*5+5]
		wantHigh := byte(0x00)
		if line == 0 || line == 2 {
			wantHigh = 0x01
		}
		want := []byte{0x05, 0x35, byte(io20DefaultOutputs[line]), 0x00, wantHigh}
		if !bytes.Equal(pkt, want) {
			t.Errorf("line %d packet = % X, want % X", line, pkt, want)
		}
	}
}

func TestReadAllAssemblesBitmask(t *testing.T) {
	d, p := newFakeIO20()
	// Lines 0..7 read 1,0,1,1,0,0,0,1 → 0b10001101.
	p.replies = []byte{1, 0, 1, 1, 0, 0, 0, 1}

	mask, err := d.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if want := byte(0b10001101); mask != want {
		t.Errorf("ReadAll = %08b, want %08b", mask, want)
	}
}

func TestLineRangeChecks(t *testing.T) {
	d, _ := newFakeIO20()
	for _, line := range []int{-1, 8, 99} {
		if err := d.SetHigh(line); err == nil {
			t.Errorf("SetHigh(%d): expected a range error", line)
		}
		if err := d.SetLow(line); err == nil {
			t.Errorf("SetLow(%d): expected a range error", line)
		}
		if _, err := d.ReadLine(line); err == nil {
			t.Errorf("ReadLine(%d): expected a range error", line)
		}
	}
}

func TestIO20GroupValidation(t *testing.T) {
	eight := []IO20Channel{
		IO20_AN0, IO20_AN1, IO20_AN2, IO20_AN3, IO20_AN4, IO20_AN5, IO20_AN6, IO20_AN7,
	}
	if _, err := io20Group("output", eight); err != nil {
		t.Errorf("valid group rejected: %v", err)
	}
	if _, err := io20Group("output", eight[:7]); err == nil {
		t.Error("7-channel group: expected an error")
	}
	dup := append([]IO20Channel(nil), eight...)
	dup[7] = IO20_AN0
	if _, err := io20Group("output", dup); err == nil {
		t.Error("duplicate channel: expected an error")
	}
	bad := append([]IO20Channel(nil), eight...)
	bad[0] = IO20Channel(0x20)
	if _, err := io20Group("output", bad); err == nil {
		t.Error("out-of-range channel: expected an error")
	}
}

func TestIO20CheckOverlap(t *testing.T) {
	if err := io20CheckOverlap(io20DefaultOutputs, io20DefaultInputs); err != nil {
		t.Errorf("defaults should not overlap: %v", err)
	}
	if err := io20CheckOverlap(io20DefaultOutputs, io20DefaultOutputs); err == nil {
		t.Error("identical groups: expected an overlap error")
	}
}

func TestIO20ChannelNames(t *testing.T) {
	for _, tc := range []struct {
		ch   IO20Channel
		want string
	}{
		{IO20_AN0, "AN0"},
		{IO20_AN13, "AN13"},
		{IO20_RA4, "RA4"},
		{IO20_P7, "P7"},
		{IO20_RB7, "RB7"},
		{IO20_RB6, "RB6"},
	} {
		if got := tc.ch.String(); got != tc.want {
			t.Errorf("channel 0x%02X = %q, want %q", byte(tc.ch), got, tc.want)
		}
	}
	if got := IO20Channel(0x33).String(); got != "IO20Channel(0x33)" {
		t.Errorf("unknown channel = %q", got)
	}
}

// TestPingAcceptsOnlyY guards against cross-detecting a DLP-IO8, which answers
// the same 0x27 opcode with 'Q'.
func TestPingAcceptsOnlyY(t *testing.T) {
	d, p := newFakeIO20()
	p.replies = []byte{'Y'}
	if ok, err := d.ping(); err != nil || !ok {
		t.Errorf("ping with 'Y': got ok=%v err=%v, want ok=true", ok, err)
	}

	d, p = newFakeIO20()
	p.replies = []byte{'Q', 'Q', 'Q'} // a DLP-IO8 on this port
	if ok, err := d.ping(); err != nil || ok {
		t.Errorf("ping with 'Q': got ok=%v err=%v, want ok=false", ok, err)
	}
}
