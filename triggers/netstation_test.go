// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeECIHost is a stand-in NetStation host: it accepts one connection, replies
// to the "QNTEL" handshake with 'I'+version, and then answers every subsequent
// command with a scripted acknowledgement byte, recording everything it read.
// A real amplifier is not needed to check that the driver speaks ECI correctly.
type fakeECIHost struct {
	addr     string
	acks     []byte // consumed one per command; last value repeats
	mu       sync.Mutex
	received bytes.Buffer
	done     chan struct{}
}

func newFakeECIHost(t *testing.T, acks []byte) *fakeECIHost {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	h := &fakeECIHost{addr: ln.Addr().String(), acks: acks, done: make(chan struct{})}
	go func() {
		defer close(h.done)
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		h.serve(conn)
	}()
	t.Cleanup(func() { <-h.done })
	return h
}

// serve reads the handshake, then replies to each command. Commands are framed
// by their leading opcode byte, so the reader must know each one's length.
func (h *fakeECIHost) serve(conn net.Conn) {
	buf := make([]byte, 1)
	readN := func(n int) []byte {
		b := make([]byte, n)
		if _, err := readFull(conn, b); err != nil {
			return nil
		}
		h.mu.Lock()
		h.received.Write(b)
		h.mu.Unlock()
		return b
	}

	// Handshake: 5 bytes "QNTEL" → 'I' + version byte.
	if q := readN(5); q == nil {
		return
	}
	if _, err := conn.Write([]byte{nsAckIdentify, 1}); err != nil {
		return
	}

	for {
		if _, err := readFull(conn, buf); err != nil {
			return
		}
		h.mu.Lock()
		h.received.Write(buf)
		h.mu.Unlock()

		switch buf[0] {
		case 'A', 'B', 'E', 'X': // no payload
		case 'T': // int32 milliseconds
			if readN(4) == nil {
				return
			}
		case 'D': // uint16 block size, then that many bytes
			sz := readN(2)
			if sz == nil {
				return
			}
			if readN(int(binary.LittleEndian.Uint16(sz))) == nil {
				return
			}
		default:
			return
		}

		ack := h.acks[0]
		if len(h.acks) > 1 {
			h.acks = h.acks[1:]
		}
		if _, err := conn.Write([]byte{ack}); err != nil {
			return
		}
		if buf[0] == 'X' {
			return
		}
	}
}

func (h *fakeECIHost) bytesReceived() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]byte(nil), h.received.Bytes()...)
}

// readFull is io.ReadFull with a deadline, so a stuck test fails fast.
func readFull(conn net.Conn, b []byte) (int, error) {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n := 0
	for n < len(b) {
		m, err := conn.Read(b[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// fastNS dials the fake host with a short timeout and no teardown delays, so
// tests do not pay Close's two seconds of settle time.
func fastNS(t *testing.T, h *fakeECIHost) *NetStation {
	t.Helper()
	ns, err := NewNetStation(h.addr, WithNSTimeout(2*time.Second))
	if err != nil {
		t.Fatalf("NewNetStation: %v", err)
	}
	ns.stopFlushDelay, ns.closeSettle, ns.closeFinalDelay = 0, 0, 0
	return ns
}

func TestNetStationHandshake(t *testing.T) {
	h := newFakeECIHost(t, []byte{nsAckSuccess})
	ns := fastNS(t, h)

	if got := ns.ECIVersion(); got != 1 {
		t.Errorf("ECIVersion = %d, want 1", got)
	}
	// "QNTEL" then the attention command the NetStation 5 reference sends.
	if got, want := string(h.bytesReceived()), "QNTELA"; got != want {
		t.Errorf("handshake sent %q, want %q", got, want)
	}
	_ = ns.Close()
}

// TestCommandRejectsFailureAck is the regression guard for the bug that let a
// refused recording pass as success.
func TestCommandRejectsFailureAck(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ack     byte
		wantSub string
	}{
		{"failure", nsAckFailure, "refused"},
		{"no recording device", nsAckNoRecDev, "not ready to record"},
		{"garbage", 0x7E, "unexpected acknowledgement"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Ack 1 answers the handshake attention; ack 2 the StartRecording.
			h := newFakeECIHost(t, []byte{nsAckSuccess, tc.ack})
			ns := fastNS(t, h)
			defer ns.Close()

			err := ns.StartRecording()
			if err == nil {
				t.Fatalf("StartRecording: expected an error for ack %#02x", tc.ack)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not mention %q", err, tc.wantSub)
			}
			if ns.Recording() {
				t.Error("Recording() is true after a refused StartRecording")
			}
		})
	}
}

func TestAcceptedAcks(t *testing.T) {
	for _, ack := range []byte{nsAckSuccess, nsAckOne, nsAckNTPClock} {
		if err := checkAck(ack); err != nil {
			t.Errorf("checkAck(%#02x) = %v, want nil", ack, err)
		}
	}
}

func TestStartStopRecordingCommands(t *testing.T) {
	h := newFakeECIHost(t, []byte{nsAckSuccess})
	ns := fastNS(t, h)

	if err := ns.StartRecording(); err != nil {
		t.Fatalf("StartRecording: %v", err)
	}
	if !ns.Recording() {
		t.Error("Recording() is false after StartRecording")
	}
	if err := ns.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}
	if ns.Recording() {
		t.Error("Recording() is true after StopRecording")
	}
	if err := ns.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got, want := string(h.bytesReceived()), "QNTELABEX"; got != want {
		t.Errorf("session sent %q, want %q", got, want)
	}
}

// TestCloseStopsAnOpenRecording is the important one for the unfinalized-.mff
// symptom: if the caller forgets StopRecording, Close must still send 'E'
// before 'X' so NetStation finalizes the file.
func TestCloseStopsAnOpenRecording(t *testing.T) {
	h := newFakeECIHost(t, []byte{nsAckSuccess})
	ns := fastNS(t, h)

	if err := ns.StartRecording(); err != nil {
		t.Fatalf("StartRecording: %v", err)
	}
	if err := ns.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got, want := string(h.bytesReceived()), "QNTELABEX"; got != want {
		t.Errorf("session sent %q, want %q (Close must stop recording first)", got, want)
	}
}

// TestCloseReportsRefusedStop guards the other half: a host that refuses the
// stop must not produce a silent success.
func TestCloseReportsRefusedStop(t *testing.T) {
	// handshake attention OK, BeginRecording OK, EndRecording refused, exit OK.
	h := newFakeECIHost(t, []byte{nsAckSuccess, nsAckSuccess, nsAckFailure, nsAckSuccess})
	ns := fastNS(t, h)

	if err := ns.StartRecording(); err != nil {
		t.Fatalf("StartRecording: %v", err)
	}
	err := ns.Close()
	if err == nil {
		t.Fatal("Close: expected an error when the host refuses to stop recording")
	}
	for _, sub := range []string{"unfinalized", "stop recording"} {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("error %q does not mention %q", err, sub)
		}
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	h := newFakeECIHost(t, []byte{nsAckSuccess})
	ns := fastNS(t, h)
	if err := ns.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := ns.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

// TestEventDatagram pins the on-wire event block, which is the part a NetStation
// host parses most strictly.
func TestEventDatagram(t *testing.T) {
	h := newFakeECIHost(t, []byte{nsAckSuccess})
	ns := fastNS(t, h)
	defer ns.Close()

	start := ns.epoch.Add(1500 * time.Millisecond)
	if err := ns.SendEventFull(Event{
		Code:     "STIM",
		Start:    start,
		Duration: 2 * time.Millisecond,
		Keys:     []EventKey{{Code: "corr", Value: 1}},
	}); err != nil {
		t.Fatalf("SendEventFull: %v", err)
	}

	got := h.bytesReceived()
	got = got[len("QNTELA"):] // drop the handshake

	want := []byte{'D'}
	want = binary.LittleEndian.AppendUint16(want, 15+12) // block size, one key
	want = binary.LittleEndian.AppendUint32(want, 1500)  // start, ms since epoch
	want = binary.LittleEndian.AppendUint32(want, 2)     // duration, ms
	want = append(want, "STIM"...)
	want = append(want, 0, 0, 1) // label len, description len, key count
	want = append(want, "corr"...)
	want = append(want, "shor"...)
	want = binary.LittleEndian.AppendUint16(want, 2) // value length in bytes
	want = binary.LittleEndian.AppendUint16(want, 1) // value

	if !bytes.Equal(got, want) {
		t.Errorf("event datagram =\n  % X\nwant\n  % X", got, want)
	}
}

// TestEventCodePadding checks the fixed-width 4-byte code field.
func TestEventCodePadding(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"STIM", "STIM"},
		{"T1", "T1  "},
		{"", "    "},
		{"TOOLONG", "TOOL"},
	} {
		var buf bytes.Buffer
		writeCode4(&buf, tc.in)
		if got := buf.String(); got != tc.want {
			t.Errorf("writeCode4(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestEventDurationClamp checks the guard against a raw timestamp passed as a
// duration, which would otherwise mark an event lasting hours.
func TestEventDurationClamp(t *testing.T) {
	h := newFakeECIHost(t, []byte{nsAckSuccess})
	ns := fastNS(t, h)
	defer ns.Close()

	if err := ns.SendEventFull(Event{Code: "STIM", Duration: time.Hour}); err != nil {
		t.Fatalf("SendEventFull: %v", err)
	}
	got := h.bytesReceived()[len("QNTELA"):]
	// 'D' + uint16 size + int32 start, then the duration field.
	dur := binary.LittleEndian.Uint32(got[7:11])
	if dur != 1 {
		t.Errorf("duration = %d ms, want 1 (an over-long duration must clamp)", dur)
	}
}
