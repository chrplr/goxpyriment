// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// NetStation controls an EGI (Electrical Geodesics) EEG recording host over
// TCP/IP using the ECI (Experiment Control Interface) protocol. Unlike the TTL
// devices in this package it does not carry electrical trigger lines; it sends
// named *event markers* into the EEG stream and controls recording remotely.
// It therefore implements neither [OutputTTLDevice] nor [InputTTLDevice] — it
// has its own richer API (see [SerialPort] for the other non-TTL device here).
//
// The typical session:
//
//	ns, err := triggers.NewNetStation("134.225.198.12") // default port 55513
//	if err != nil { log.Fatal(err) }
//	defer ns.Close()
//
//	ns.Synchronize()        // align the host clock to ours
//	ns.StartRecording()
//	ns.SendEvent("STIM")    // mark stimulus onset (send near the VSYNC flip)
//	ns.StopRecording()
//
// Byte order: goxpyriment always advertises the "QNTEL" (Intel/little-endian)
// ECI variant during the handshake and encodes every multi-byte field
// little-endian, so the driver is portable regardless of the host CPU it runs
// on. (The original MATLAB NetStation routines switched between QNTEL and QMAC-
// only because they wrote in the machine's *native* order.)
//
// # What ECI does and does not control (NetStation 5.x)
//
// ECI has exactly nine commands — endian query, attention, clock sync, NTP
// clock sync, NTP return clock, begin/end recording, event data and exit.
// None of them names the output file, selects a recording format or finalizes
// it. The .mff bundle is written entirely by NetStation Acquisition, so its
// format and version are a property of the host and its workspace, not of this
// client.
//
// What this client *can* get wrong is leaving a recording open. An .mff that
// contains Acquiring.xml and no info.xml — so neither the events nor the signal
// can be read back — is EGI's documented signature of an acquisition that never
// completed its recording. Every command is therefore acknowledgement-checked
// (see [NetStation.Close]): if the host refuses a begin/end recording, the call
// returns an error instead of silently continuing towards an unusable file.
//
// Ported from Gergely Csibra's NetStation MATLAB routines (2006), themselves
// based on Rick Gilmore's routines (2005), with the acknowledgement and
// handshake behaviour cross-checked against the NetStation 5.3-tested
// egi-pynetstation client (github.com/nimh-sfim/egi-pynetstation).
type NetStation struct {
	conn       net.Conn
	host       string
	timeout    time.Duration
	epoch      time.Time // reference for event/synchronization timestamps
	recording  bool
	eciVersion byte

	// Teardown delays, defaulted from the ns*Delay constants. Fields rather
	// than constants so tests can zero them; there is no reason to change them
	// at runtime.
	stopFlushDelay  time.Duration
	closeSettle     time.Duration
	closeFinalDelay time.Duration
}

const (
	nsDefaultPort    = 55513
	nsDefaultTimeout = 2 * time.Second

	// Delays taken from the reference Disconnect routine: they give the host
	// time to flush trailing events before recording is stopped and the socket
	// is torn down, so the last markers are not lost.
	nsStopFlushDelay  = 500 * time.Millisecond
	nsCloseSettle     = 1 * time.Second
	nsCloseFinalDelay = 500 * time.Millisecond

	// Events longer than this are almost always a mistake (a raw timestamp
	// passed as a duration); mirror the reference guard and clamp to 1 ms.
	nsMaxEventDuration = 120 * time.Second
)

// ECI acknowledgement bytes. The SDK documents only 'Z'; the others were
// established by the reference clients through testing against live hosts.
const (
	nsAckSuccess  byte = 'Z'  // command accepted
	nsAckIdentify byte = 'I'  // identify reply — one version byte follows
	nsAckFailure  byte = 'F'  // command refused
	nsAckNoRecDev byte = 'R'  // refused: no recording device ready
	nsAckNTPClock byte = 'S'  // NTP return clock — an NTPv4 timecode follows
	nsAckOne      byte = 0x01 // undocumented success reply seen from NetStation
)

// checkAck turns an ECI acknowledgement byte into an error.
//
// The driver used to read the ack and discard it. That made two host-side
// refusals invisible — 'F' (command refused) and 'R' (no recording device) —
// so a StartRecording that the host had rejected still reported success, the
// run proceeded, and the only symptom was an unreadable .mff afterwards.
func checkAck(b byte) error {
	switch b {
	case nsAckSuccess, nsAckOne, nsAckNTPClock:
		return nil
	case nsAckFailure:
		return fmt.Errorf("host refused the command (ECI 'F')")
	case nsAckNoRecDev:
		return fmt.Errorf("host is not ready to record (ECI 'R') — check that a session is open " +
			"and an amplifier is connected in NetStation Acquisition")
	default:
		return fmt.Errorf("unexpected acknowledgement %#02x from host", b)
	}
}

// NetStationOption configures a [NetStation] at construction time.
type NetStationOption func(*NetStation)

// WithNSTimeout sets the TCP dial timeout and the per-command acknowledgement
// read timeout. Default: 2 s.
func WithNSTimeout(d time.Duration) NetStationOption {
	return func(ns *NetStation) { ns.timeout = d }
}

// EventKey is an optional key/value pair attached to a NetStation [Event].
// Code is a 4-character identifier (e.g. "tria", "cel#"); shorter strings are
// padded with spaces and longer ones truncated to 4 bytes. Value is a signed
// 16-bit integer (−32767..32767).
type EventKey struct {
	Code  string
	Value int16
}

// Event describes a NetStation event marker.
type Event struct {
	// Code is the 4-character event code (e.g. "STIM"). Padded with spaces or
	// truncated to 4 bytes. Empty defaults to "EVEN".
	Code string
	// Start is the event onset. The zero value means "now". Pass the VSYNC
	// flip time here to mark the true stimulus onset.
	Start time.Time
	// Duration is the event duration; values ≤ 0 default to 1 ms.
	Duration time.Duration
	// Keys are optional key/value metadata pairs.
	Keys []EventKey
}

// NewNetStation opens a TCP connection to a NetStation host and performs the
// ECI handshake. host may be "10.0.0.5" (port 55513 assumed) or
// "10.0.0.5:55513". Returns a ready device; call [NetStation.Close] when done.
func NewNetStation(host string, opts ...NetStationOption) (*NetStation, error) {
	ns := &NetStation{
		timeout:         nsDefaultTimeout,
		epoch:           time.Now(),
		stopFlushDelay:  nsStopFlushDelay,
		closeSettle:     nsCloseSettle,
		closeFinalDelay: nsCloseFinalDelay,
	}
	for _, opt := range opts {
		opt(ns)
	}
	if !strings.Contains(host, ":") {
		host = fmt.Sprintf("%s:%d", host, nsDefaultPort)
	}
	ns.host = host

	conn, err := net.DialTimeout("tcp", host, ns.timeout)
	if err != nil {
		return nil, fmt.Errorf("netstation: connect %s: %w", host, err)
	}
	ns.conn = conn

	if err := ns.handshake(); err != nil {
		conn.Close()
		ns.conn = nil
		return nil, err
	}
	return ns, nil
}

// handshake advertises the QNTEL (Intel/little-endian) ECI variant and checks
// the host's identify reply and protocol version.
func (ns *NetStation) handshake() error {
	if _, err := ns.conn.Write([]byte("QNTEL")); err != nil {
		return fmt.Errorf("netstation: handshake write: %w", err)
	}
	reply, err := ns.readN(1)
	if err != nil {
		return fmt.Errorf("netstation: handshake read: %w", err)
	}
	switch reply[0] {
	case nsAckIdentify: // identify OK — one version byte follows
		ver, err := ns.readN(1)
		if err != nil {
			return fmt.Errorf("netstation: handshake version read: %w", err)
		}
		// Accept whatever the host reports rather than pinning version 1: the
		// reference clients only record it, and rejecting an unknown value
		// would lock the driver out of future NetStation releases for no gain.
		ns.eciVersion = ver[0]
	case nsAckFailure:
		return fmt.Errorf("netstation: ECI handshake refused by host")
	default:
		return fmt.Errorf("netstation: unexpected handshake reply %#02x", reply[0])
	}
	// The NetStation 5-tested reference client sends an attention command
	// immediately after the endian query; mirror that.
	if err := ns.command([]byte{'A'}); err != nil {
		return fmt.Errorf("netstation: handshake attention: %w", err)
	}
	return nil
}

// ECIVersion returns the protocol version byte the host reported during the
// handshake (1 for every NetStation release tested so far).
func (ns *NetStation) ECIVersion() byte { return ns.eciVersion }

// Synchronize aligns the host's clock to ours: it sends the current experiment
// time so that subsequent event timestamps are interpreted in the same frame.
// Call it once after connecting (and again if you suspect drift).
func (ns *NetStation) Synchronize() error {
	if err := ns.command([]byte{'A'}); err != nil {
		return fmt.Errorf("netstation: Synchronize (attention): %w", err)
	}
	var buf bytes.Buffer
	buf.WriteByte('T')
	binary.Write(&buf, binary.LittleEndian, ns.millisSince(time.Now()))
	if err := ns.command(buf.Bytes()); err != nil {
		return fmt.Errorf("netstation: Synchronize (time): %w", err)
	}
	return nil
}

// StartRecording instructs the host to begin recording. No-op if already
// recording.
func (ns *NetStation) StartRecording() error {
	if ns.recording {
		return nil
	}
	if err := ns.command([]byte{'B'}); err != nil {
		return fmt.Errorf("netstation: StartRecording: %w", err)
	}
	ns.recording = true
	return nil
}

// StopRecording instructs the host to stop recording. No-op if not recording.
// A short settle delay precedes the stop so trailing events are flushed.
func (ns *NetStation) StopRecording() error {
	if !ns.recording {
		return nil
	}
	time.Sleep(ns.stopFlushDelay)
	if err := ns.command([]byte{'E'}); err != nil {
		return fmt.Errorf("netstation: StopRecording: %w", err)
	}
	ns.recording = false
	return nil
}

// SendEvent is the common case: mark an event now, with the given 4-character
// code, a 1 ms duration and no keys. Send it as close as possible to the
// stimulus VSYNC flip.
func (ns *NetStation) SendEvent(code string) error {
	return ns.SendEventFull(Event{Code: code})
}

// SendEventFull sends a fully specified event marker (code, onset, duration and
// optional key/value pairs) to the host.
func (ns *NetStation) SendEventFull(ev Event) error {
	code := ev.Code
	if code == "" {
		code = "EVEN"
	}
	start := ev.Start
	if start.IsZero() {
		start = time.Now()
	}
	dur := ev.Duration
	if dur <= 0 || dur > nsMaxEventDuration {
		dur = time.Millisecond
	}
	keyn := len(ev.Keys)
	if keyn > 255 {
		return fmt.Errorf("netstation: too many event keys (%d, max 255)", keyn)
	}

	var buf bytes.Buffer
	buf.WriteByte('D')
	// Block size: everything after this uint16 field. The fixed part is 15
	// bytes — int32 start, int32 duration, 4-byte code, uint8 label length,
	// uint8 description length, uint8 key count — and each key adds 12 bytes.
	// Label and description are always empty here, so the two length bytes are
	// zero. (The 2006 MATLAB port wrote a single int16 zero at this position,
	// which is the same two bytes on the wire.)
	binary.Write(&buf, binary.LittleEndian, uint16(15+12*keyn))
	binary.Write(&buf, binary.LittleEndian, ns.millisSince(start))
	binary.Write(&buf, binary.LittleEndian, int32(dur.Milliseconds()))
	writeCode4(&buf, code)
	buf.WriteByte(0) // label length (no label)
	buf.WriteByte(0) // description length (no description)
	buf.WriteByte(uint8(keyn))
	for _, k := range ev.Keys {
		writeCode4(&buf, k.Code)
		buf.WriteString("shor")                           // data type: 16-bit signed
		binary.Write(&buf, binary.LittleEndian, int16(2)) // data length in bytes
		binary.Write(&buf, binary.LittleEndian, k.Value)
	}
	if err := ns.command(buf.Bytes()); err != nil {
		return fmt.Errorf("netstation: SendEvent %q: %w", code, err)
	}
	return nil
}

// Recording reports whether recording is currently active (per the last
// successful Start/StopRecording call).
func (ns *NetStation) Recording() bool { return ns.recording }

// Close stops recording (if active), disconnects the ECI session and closes
// the TCP connection. Safe to call multiple times. Deliberately blocks for a
// couple of seconds so the host can flush and finalize the recording.
//
// The connection is always torn down, but a failure to stop recording or to end
// the session is now reported rather than discarded: those are precisely the
// failures that leave NetStation with an unfinalized .mff (Acquiring.xml
// present, info.xml missing), and they used to pass silently. Always check this
// error — in a deferred call, log it:
//
//	defer func() {
//		if err := ns.Close(); err != nil {
//			log.Printf("netstation: %v", err)  // the recording may be unfinalized
//		}
//	}()
func (ns *NetStation) Close() error {
	if ns.conn == nil {
		return nil
	}
	var errs []error
	if ns.recording {
		time.Sleep(ns.stopFlushDelay)
		if err := ns.command([]byte{'E'}); err != nil { // stop recording
			errs = append(errs, fmt.Errorf("stop recording: %w", err))
		}
		ns.recording = false
	}
	time.Sleep(ns.closeSettle)
	if err := ns.command([]byte{'X'}); err != nil { // end ECI session
		errs = append(errs, fmt.Errorf("end ECI session: %w", err))
	}
	time.Sleep(ns.closeFinalDelay)
	if err := ns.conn.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close connection: %w", err))
	}
	ns.conn = nil
	if len(errs) > 0 {
		return fmt.Errorf("netstation: Close (the recording may be unfinalized): %w", errors.Join(errs...))
	}
	return nil
}

// command writes a payload, reads the host's one-byte acknowledgement and
// validates it, keeping the request/response stream in step. Write, read and
// refusal errors are all surfaced.
func (ns *NetStation) command(payload []byte) error {
	if ns.conn == nil {
		return fmt.Errorf("host not connected")
	}
	if _, err := ns.conn.Write(payload); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	ack, err := ns.readN(1)
	if err != nil {
		return fmt.Errorf("acknowledgement: %w", err)
	}
	return checkAck(ack[0])
}

// readN reads exactly n bytes subject to the configured timeout.
func (ns *NetStation) readN(n int) ([]byte, error) {
	if err := ns.conn.SetReadDeadline(time.Now().Add(ns.timeout)); err != nil {
		return nil, err
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(ns.conn, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// millisSince returns the milliseconds from the connection epoch to t as an
// int32, matching the ECI on-wire timestamp field.
func (ns *NetStation) millisSince(t time.Time) int32 {
	return int32(t.Sub(ns.epoch).Milliseconds())
}

// writeCode4 writes s as exactly 4 bytes: space-padded if shorter, truncated if
// longer (matching the ECI fixed-width code fields).
func writeCode4(buf *bytes.Buffer, s string) {
	var b [4]byte
	copy(b[:], "    ")
	copy(b[:], s)
	buf.Write(b[:])
}
