// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package triggers

import (
	"bytes"
	"encoding/binary"
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
// Ported from Gergely Csibra's NetStation MATLAB routines (2006), themselves
// based on Rick Gilmore's routines (2005).
type NetStation struct {
	conn      net.Conn
	host      string
	timeout   time.Duration
	epoch     time.Time // reference for event/synchronization timestamps
	recording bool
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
		timeout: nsDefaultTimeout,
		epoch:   time.Now(),
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
	case 'I': // identify OK — one version byte follows
		ver, err := ns.readN(1)
		if err != nil {
			return fmt.Errorf("netstation: handshake version read: %w", err)
		}
		if ver[0] != 1 {
			return fmt.Errorf("netstation: unsupported ECI version %d (expected 1)", ver[0])
		}
		return nil
	case 'F':
		return fmt.Errorf("netstation: ECI handshake refused by host")
	default:
		return fmt.Errorf("netstation: unexpected handshake reply %q", reply[0])
	}
}

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
	time.Sleep(nsStopFlushDelay)
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
	// Block size: everything after this uint16 field. Fixed part is 15 bytes
	// (int32 start + int32 duration + 4-byte code + int16 label-length + uint8
	// key-count); each key adds 12 bytes.
	binary.Write(&buf, binary.LittleEndian, uint16(15+12*keyn))
	binary.Write(&buf, binary.LittleEndian, ns.millisSince(start))
	binary.Write(&buf, binary.LittleEndian, int32(dur.Milliseconds()))
	writeCode4(&buf, code)
	binary.Write(&buf, binary.LittleEndian, int16(0)) // label length (no label)
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
func (ns *NetStation) Close() error {
	if ns.conn == nil {
		return nil
	}
	if ns.recording {
		time.Sleep(nsStopFlushDelay)
		_ = ns.command([]byte{'E'}) // stop recording
		ns.recording = false
	}
	time.Sleep(nsCloseSettle)
	_ = ns.command([]byte{'X'}) // end ECI session
	time.Sleep(nsCloseFinalDelay)
	err := ns.conn.Close()
	ns.conn = nil
	return err
}

// command writes a payload and reads the host's one-byte acknowledgement,
// keeping the request/response stream in step. The ack value is drained but not
// validated (the reference protocol replies 'Z' on success); write and read
// errors are surfaced.
func (ns *NetStation) command(payload []byte) error {
	if ns.conn == nil {
		return fmt.Errorf("host not connected")
	}
	if _, err := ns.conn.Write(payload); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if _, err := ns.readN(1); err != nil {
		return fmt.Errorf("acknowledgement: %w", err)
	}
	return nil
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
