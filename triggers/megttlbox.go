// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"go.bug.st/serial"
)

// MEGTTLBox is a Go client for the NeuroSpin Arduino Mega–based TTL interface
// used in MEG experiments. It exposes 8 TTL output lines (for trigger codes to
// the STI recording channel) and 8 TTL input lines (for fiber-optic response
// pad buttons).
//
// It implements both [OutputTTLDevice] and [InputTTLDevice].
//
// Hardware: Arduino Mega 2560, USB-CDC at 115200 baud.
//   - Output lines 0–7 → Arduino pins D30–D37 → STI channel
//   - Input  lines 0–7 → Arduino pins D22–D29 ← FORP button presses
//
// Construct with [NewMEGTTLBox]. Always defer [MEGTTLBox.Close].
//
//	box, err := triggers.NewMEGTTLBox("/dev/ttyACM0")
//	if err != nil { log.Fatal(err) }
//	defer box.Close()
//
//	box.Pulse(0, 5*time.Millisecond)
//	mask, rt, _ := box.WaitForInput(ctx)
type MEGTTLBox struct {
	port            serial.Port
	resetDelay      time.Duration
	pollInterval    time.Duration
	triggerDurMS    uint16  // last duration sent to device (ms)
	triggerDurKnown bool    // whether triggerDurMS has been sent at least once
	info            MEGInfo // firmware identification, probed at open

	// Clock alignment for firmware timestamps. syncHost is our best estimate
	// of the host instant at which the device reported syncRaw.
	syncHost  time.Time
	syncRaw   uint32
	syncValid bool
	dropped   bool // sticky: firmware reported a queue overflow
}

// MEGTTLBoxOption configures a [MEGTTLBox] at construction time.
type MEGTTLBoxOption func(*MEGTTLBox)

// WithResetDelay sets how long [NewMEGTTLBox] waits after asserting DTR for
// the Arduino firmware to boot. Pass 0 to skip the reset wait (e.g. when
// connecting to a device that is already running). Default: 2 s.
func WithResetDelay(d time.Duration) MEGTTLBoxOption {
	return func(b *MEGTTLBox) { b.resetDelay = d }
}

// WithPollInterval sets the polling interval used by [MEGTTLBox.WaitForInput]
// and [MEGTTLBox.DrainInputs]. Default: 5 ms.
func WithPollInterval(d time.Duration) MEGTTLBoxOption {
	return func(b *MEGTTLBox) { b.pollInterval = d }
}

const (
	megBaudRate            = 115200
	megDefaultResetDelay   = 2 * time.Second
	megDefaultPollInterval = 5 * time.Millisecond
	megReadTimeout         = 200 * time.Millisecond
)

// Binary protocol opcodes (must match Arduino firmware exactly).
const (
	megOpGetInfo            = 1  // → 'M','T','B', version uint8, caps uint8
	megOpSetTriggerDuration = 10 // + uint16 LE (ms)
	megOpSendTriggerMask    = 11 // + uint8 mask — pulse all set lines
	megOpSendTriggerOnLine  = 12 // + uint8 line (0–7)
	megOpSetHighMask        = 13 // + uint8 mask — persistent HIGH
	megOpSetLowMask         = 14 // + uint8 mask — persistent LOW
	megOpSetHighOnLine      = 15 // + uint8 line (0–7)
	megOpSetLowOnLine       = 16 // + uint8 line (0–7)
	megOpSetPortMask        = 17 // + uint8 mask — atomic; needs MEGCapAtomicPort
	megOpGetResponseButton  = 20 // → returns uint8 button mask
	megOpGetEvent           = 21 // → flags uint8, mask uint8, micros uint32 LE
	megOpGetMicros          = 22 // → uint32 LE device micros()
	megOpClearEvents        = 23 // drop queued events, re-seed the detector
	megOpSetDebounce        = 24 // + uint16 LE (µs), 0 disables
)

// get_event reply flags.
const (
	megEvPresent = 0x01 // an event follows; mask and timestamp are meaningful
	megEvDropped = 0x02 // the firmware's queue overflowed; events were lost
)

const (
	// megSyncSamples is how many round trips SyncClock takes, keeping the
	// fastest. A slow round trip brackets the device's reading loosely, so the
	// tightest one gives the best midpoint estimate.
	megSyncSamples = 7

	// megResyncAfter bounds how stale the clock offset may get. Device
	// timestamps are decoded as a signed 32-bit microsecond delta from the last
	// sync, which stays exact while |delta| < 2^31 µs ≈ 35.8 min — so
	// re-syncing well inside that makes micros() wrap a non-issue.
	megResyncAfter = 20 * time.Minute
)

// ErrMEGNoTimestamps is returned by the event API when the firmware does not
// advertise [MEGCapTimestamps]. Such firmware ignores the event opcodes
// silently, so this is reported up front rather than as a timeout.
var ErrMEGNoTimestamps = errors.New("megttlbox: firmware has no timestamped input events (reflash to enable)")

// InputEvent is a change of button state, timestamped by the firmware at the
// moment it happened rather than when the host got round to asking.
type InputEvent struct {
	// Mask is the button bitmask *after* the change. A press sets a bit, a
	// release clears one; Mask == 0 means every button is now up.
	Mask byte
	// TS is the change translated into the host clock, so it can be compared
	// directly against stimulus timestamps.
	TS time.Time
	// Raw is the device's own micros() value, kept for diagnostics.
	Raw uint32
}

// Pressed reports whether any button is down after this event.
func (e InputEvent) Pressed() bool { return e.Mask != 0 }

// megInfoTimeout bounds the get_info probe. Firmware without the opcode never
// answers, so this is how long [NewMEGTTLBox] waits before deciding "legacy".
const megInfoTimeout = 300 * time.Millisecond

// megInfoMagic opens every get_info reply, so a non-TTL-box device answering on
// the same port is not mistaken for one.
var megInfoMagic = [3]byte{'M', 'T', 'B'}

// Capability bits reported in [MEGInfo.Capabilities]. Firmware version 1
// reports none of them; they exist so a host can feature-detect rather than
// assume, as firmware gains the corresponding opcodes.
const (
	// MEGCapAtomicPort means the firmware can set all 8 output lines in a
	// single port write, making a trigger code glitch-free.
	MEGCapAtomicPort uint8 = 1 << 0
	// MEGCapTimestamps means the firmware timestamps input transitions with
	// micros() rather than leaving the host to poll for them.
	MEGCapTimestamps uint8 = 1 << 1
)

// MEGInfo describes the firmware on the other end of the port, as reported by
// [MEGTTLBox.Info].
type MEGInfo struct {
	Version      uint8 // firmware protocol version; 0 when Legacy
	Capabilities uint8 // bitmask of MEGCap* bits
	// Legacy is true for firmware predating the get_info opcode. Such firmware
	// silently ignores unknown opcodes, so it is detected by the probe timing
	// out rather than by any positive signal. Only the original opcode set
	// (10–16, 20) may be used against it.
	Legacy bool
}

// Has reports whether the firmware advertises the given capability bit.
// It is always false for legacy firmware.
func (i MEGInfo) Has(cap uint8) bool { return i.Capabilities&cap != 0 }

// String renders the firmware identification for logs.
func (i MEGInfo) String() string {
	if i.Legacy {
		return "legacy firmware (no get_info)"
	}
	return fmt.Sprintf("firmware v%d, caps 0x%02X", i.Version, i.Capabilities)
}

// Sentinel errors returned by MEGTTLBox methods.
var (
	ErrMEGNotOpen     = errors.New("megttlbox: port not open")
	ErrMEGTimeout     = errors.New("megttlbox: read timeout")
	ErrMEGBadLine     = errors.New("megttlbox: line out of range (0–7)")
	ErrMEGBadDuration = errors.New("megttlbox: duration out of range (0–65535 ms)")
)

// NewMEGTTLBox opens the serial port at portPath, asserts DTR to trigger the
// Arduino hardware reset, waits for the firmware to boot, then applies any
// options. Returns an error if the port cannot be opened.
func NewMEGTTLBox(portPath string, opts ...MEGTTLBoxOption) (*MEGTTLBox, error) {
	mode := &serial.Mode{
		BaudRate: megBaudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	p, err := serial.Open(portPath, mode)
	if err != nil {
		return nil, fmt.Errorf("megttlbox: open %s: %w", portPath, err)
	}
	p.SetReadTimeout(megReadTimeout)

	b := &MEGTTLBox{
		port:         p,
		resetDelay:   megDefaultResetDelay,
		pollInterval: megDefaultPollInterval,
	}

	for _, opt := range opts {
		opt(b)
	}

	// Assert DTR to reset the Arduino, then wait for firmware boot.
	if b.resetDelay > 0 {
		if err := p.SetDTR(true); err != nil {
			p.Close()
			return nil, fmt.Errorf("megttlbox: set DTR on %s: %w", portPath, err)
		}
		time.Sleep(b.resetDelay)
		p.ResetInputBuffer()
	}

	// Identify the firmware once, so callers can feature-detect instead of
	// assuming. A timeout here means legacy firmware, which is fine; a reply
	// that arrives but is malformed means this is not a TTL box at all.
	info, err := b.probeInfo()
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("megttlbox: identify %s: %w", portPath, err)
	}
	b.info = info

	// Align the clocks up front so the first timestamped event is usable
	// without the caller having to think about it.
	if b.info.Has(MEGCapTimestamps) {
		if err := b.SyncClock(); err != nil {
			p.Close()
			return nil, fmt.Errorf("megttlbox: sync clock on %s: %w", portPath, err)
		}
	}

	return b, nil
}

// tx sends raw bytes to the device.
func (b *MEGTTLBox) tx(data ...byte) error {
	if b.port == nil {
		return ErrMEGNotOpen
	}
	_, err := b.port.Write(data)
	return err
}

// rx1 reads exactly one byte from the device, retrying on short reads.
func (b *MEGTTLBox) rx1() (byte, error) {
	if b.port == nil {
		return 0, ErrMEGNotOpen
	}
	buf := make([]byte, 1)
	for i := 0; i < 3; i++ {
		n, err := b.port.Read(buf)
		if n == 1 {
			return buf[0], nil
		}
		if err != nil {
			return 0, fmt.Errorf("megttlbox: reading byte: %w", err)
		}
	}
	return 0, ErrMEGTimeout
}

// rxFull reads exactly len(buf) bytes, or gives up at the deadline. A single
// Read can return a short count even when more bytes are still in flight, so a
// multi-byte reply has to be accumulated rather than read in one call.
func (b *MEGTTLBox) rxFull(buf []byte, deadline time.Time) error {
	if b.port == nil {
		return ErrMEGNotOpen
	}
	for got := 0; got < len(buf); {
		if time.Now().After(deadline) {
			return ErrMEGTimeout
		}
		n, err := b.port.Read(buf[got:])
		if err != nil {
			return fmt.Errorf("megttlbox: reading reply: %w", err)
		}
		got += n
	}
	return nil
}

// Info returns the firmware identification probed at open. It does not touch
// the device.
func (b *MEGTTLBox) Info() MEGInfo { return b.info }

// probeInfo asks the firmware to identify itself (opcode 1).
//
// Firmware predating the opcode falls through the sketch's `default:` branch
// and answers nothing at all, so "legacy" is inferred from a timeout — there is
// no positive signal to wait for. A reply that arrives without the expected
// magic is a different matter: something is on this port that is not a TTL box,
// and that is reported as an error rather than silently downgraded.
func (b *MEGTTLBox) probeInfo() (MEGInfo, error) {
	if b.port == nil {
		return MEGInfo{}, ErrMEGNotOpen
	}
	b.port.ResetInputBuffer()
	if err := b.tx(megOpGetInfo); err != nil {
		return MEGInfo{}, fmt.Errorf("megttlbox.Info: %w", err)
	}
	var reply [5]byte
	if err := b.rxFull(reply[:], time.Now().Add(megInfoTimeout)); err != nil {
		if errors.Is(err, ErrMEGTimeout) {
			return MEGInfo{Legacy: true}, nil
		}
		return MEGInfo{}, fmt.Errorf("megttlbox.Info: %w", err)
	}
	if [3]byte(reply[:3]) != megInfoMagic {
		return MEGInfo{}, fmt.Errorf("megttlbox.Info: bad signature % 02X (not a TTL box?)", reply[:3])
	}
	return MEGInfo{Version: reply[3], Capabilities: reply[4]}, nil
}

// --- Timestamped input events (firmware with MEGCapTimestamps) ---

// SyncClock estimates the offset between the device's micros() clock and the
// host clock, so firmware timestamps can be reported as host instants.
//
// It is called automatically by [NewMEGTTLBox] on capable firmware and refreshed
// as needed, so most callers never need it. Call it explicitly to re-align after
// the host clock has been stepped.
//
// Each sample brackets the device's reading between two host readings and takes
// the midpoint; the fastest round trip gives the tightest bracket, so that is
// the one kept.
func (b *MEGTTLBox) SyncClock() error {
	if !b.info.Has(MEGCapTimestamps) {
		return ErrMEGNoTimestamps
	}
	best := time.Duration(math.MaxInt64)
	for i := 0; i < megSyncSamples; i++ {
		b.port.ResetInputBuffer()
		t0 := time.Now()
		if err := b.tx(megOpGetMicros); err != nil {
			return fmt.Errorf("megttlbox.SyncClock: %w", err)
		}
		var raw [4]byte
		if err := b.rxFull(raw[:], time.Now().Add(megReadTimeout)); err != nil {
			return fmt.Errorf("megttlbox.SyncClock: %w", err)
		}
		t1 := time.Now()
		if rtt := t1.Sub(t0); rtt < best {
			best = rtt
			b.syncRaw = binary.LittleEndian.Uint32(raw[:])
			b.syncHost = t0.Add(rtt / 2)
		}
	}
	b.syncValid = true
	return nil
}

// deviceToHost converts a device micros() value into a host instant.
//
// The subtraction is deliberately done in uint32 and then cast to int32: that
// wraps exactly as the device's own counter does, so a timestamp taken either
// side of the ~71.6 minute rollover still decodes correctly, provided it is
// within ±2^31 µs (~35.8 min) of the sync point. [megResyncAfter] keeps it so.
func (b *MEGTTLBox) deviceToHost(raw uint32) time.Time {
	delta := int32(raw - b.syncRaw)
	return b.syncHost.Add(time.Duration(delta) * time.Microsecond)
}

// maybeSync (re)aligns the clocks when the offset is missing or stale.
func (b *MEGTTLBox) maybeSync() error {
	if b.syncValid && time.Since(b.syncHost) < megResyncAfter {
		return nil
	}
	return b.SyncClock()
}

// PollEvent fetches the oldest queued input event, if any. ok is false when the
// queue is empty. It never blocks beyond one round trip.
func (b *MEGTTLBox) PollEvent() (ev InputEvent, ok bool, err error) {
	if !b.info.Has(MEGCapTimestamps) {
		return InputEvent{}, false, ErrMEGNoTimestamps
	}
	if err := b.maybeSync(); err != nil {
		return InputEvent{}, false, err
	}
	b.port.ResetInputBuffer()
	if err := b.tx(megOpGetEvent); err != nil {
		return InputEvent{}, false, fmt.Errorf("megttlbox.PollEvent: %w", err)
	}
	var r [6]byte
	if err := b.rxFull(r[:], time.Now().Add(megReadTimeout)); err != nil {
		return InputEvent{}, false, fmt.Errorf("megttlbox.PollEvent: %w", err)
	}
	if r[0]&megEvDropped != 0 {
		b.dropped = true
	}
	if r[0]&megEvPresent == 0 {
		return InputEvent{}, false, nil
	}
	raw := binary.LittleEndian.Uint32(r[2:])
	return InputEvent{Mask: r[1], Raw: raw, TS: b.deviceToHost(raw)}, true, nil
}

// EventsDropped reports whether the firmware's event queue has overflowed since
// the last call, and clears the flag.
//
// An overflow means presses were lost, not merely delayed, so a trial that sees
// this should be treated as suspect rather than silently trusted. It takes 32
// unread transitions to trigger — in practice a mechanical button chattering,
// which is what the debounce in [MEGTTLBox.SetDebounce] is for.
func (b *MEGTTLBox) EventsDropped() bool {
	d := b.dropped
	b.dropped = false
	return d
}

// DrainEvents discards queued events and re-seeds the firmware's change
// detector, so a button already held down is not reported as a fresh press.
// Call it between trials, in place of [MEGTTLBox.DrainInputs], when using the
// event API.
func (b *MEGTTLBox) DrainEvents() error {
	if !b.info.Has(MEGCapTimestamps) {
		return ErrMEGNoTimestamps
	}
	if err := b.tx(megOpClearEvents); err != nil {
		return fmt.Errorf("megttlbox.DrainEvents: %w", err)
	}
	b.dropped = false
	return nil
}

// SetDebounce tells the firmware to ignore transitions occurring within d of the
// previous one. Pass 0 to disable, which is the default.
//
// Leave it off for fibre-optic pads, which do not bounce: suppressing real
// transitions is worse than reporting extra ones. Use it for mechanical buttons,
// where chatter can otherwise overflow the event queue.
func (b *MEGTTLBox) SetDebounce(d time.Duration) error {
	if !b.info.Has(MEGCapTimestamps) {
		return ErrMEGNoTimestamps
	}
	us := d.Microseconds()
	if us < 0 || us > 65535 {
		return fmt.Errorf("megttlbox.SetDebounce: %w: %v (0–65535 µs)", ErrMEGBadDuration, d)
	}
	v := uint16(us)
	if err := b.tx(megOpSetDebounce, byte(v&0xFF), byte(v>>8)); err != nil {
		return fmt.Errorf("megttlbox.SetDebounce: %w", err)
	}
	return nil
}

// WaitForPressTS blocks until a button goes down, and returns the event carrying
// the firmware's timestamp of the press.
//
// This is the accurate counterpart to [MEGTTLBox.WaitForInput]. WaitForInput
// measures elapsed host time and so is floored by the poll interval; here the
// host's polling only affects how soon it *learns* of the press, not the
// recorded instant. Subtract your stimulus-onset timestamp from ev.TS to get a
// reaction time.
//
// Release events are skipped. Call [MEGTTLBox.DrainEvents] first to clear
// presses left over from a previous trial.
func (b *MEGTTLBox) WaitForPressTS(ctx context.Context) (InputEvent, error) {
	if !b.info.Has(MEGCapTimestamps) {
		return InputEvent{}, ErrMEGNoTimestamps
	}
	for {
		ev, ok, err := b.PollEvent()
		if err != nil {
			return InputEvent{}, err
		}
		if ok {
			if ev.Pressed() {
				return ev, nil
			}
			// A release: keep draining rather than sleeping, since more
			// events may already be queued behind it.
			continue
		}
		select {
		case <-ctx.Done():
			return InputEvent{}, ctx.Err()
		case <-time.After(b.pollInterval):
		}
	}
}

// setTriggerDuration sends opcode 10 only when the duration has changed.
func (b *MEGTTLBox) setTriggerDuration(dur time.Duration) error {
	ms := dur.Milliseconds()
	if ms < 0 || ms > 65535 {
		return ErrMEGBadDuration
	}
	v := uint16(ms)
	if b.triggerDurKnown && b.triggerDurMS == v {
		return nil
	}
	lo := byte(v & 0xFF)
	hi := byte(v >> 8)
	if err := b.tx(megOpSetTriggerDuration, lo, hi); err != nil {
		return fmt.Errorf("megttlbox.setTriggerDuration: %w", err)
	}
	b.triggerDurMS = v
	b.triggerDurKnown = true
	return nil
}

// --- OutputTTLDevice ---

// Send sets all 8 output lines persistently from a bitmask.
// Bit N drives line N HIGH; a zero bit drives it LOW.
// Implements [OutputTTLDevice].
//
// Firmware advertising [MEGCapAtomicPort] gets a single set-port command, which
// assigns all 8 lines in one instruction: the code is never visible
// half-written, so an amplifier cannot latch an intermediate value.
//
// Older firmware has no such opcode and needs two commands, set-high then
// set-low, so the lines do not change simultaneously. Those two are written with
// a *single* Write so both ride the same USB transfer; issued as two writes they
// can land in different USB frames, leaving the port at (previous | mask) for up
// to a frame — a valid-looking but wrong trigger code that a 1 kHz amplifier can
// latch. Sharing one transfer narrows that window to the firmware's parse time
// but does not close it. Reflash to close it.
func (b *MEGTTLBox) Send(mask byte) error {
	if b.info.Has(MEGCapAtomicPort) {
		if err := b.tx(megOpSetPortMask, mask); err != nil {
			return fmt.Errorf("megttlbox.Send: %w", err)
		}
		return nil
	}
	if err := b.tx(megOpSetHighMask, mask, megOpSetLowMask, ^mask); err != nil {
		return fmt.Errorf("megttlbox.Send: %w", err)
	}
	return nil
}

// SetHigh drives a single output line HIGH persistently. line is 0-indexed (0–7).
// Implements [OutputTTLDevice].
func (b *MEGTTLBox) SetHigh(line int) error {
	if line < 0 || line > 7 {
		return ErrMEGBadLine
	}
	return b.tx(megOpSetHighOnLine, byte(line))
}

// SetLow drives a single output line LOW persistently. line is 0-indexed (0–7).
// Implements [OutputTTLDevice].
func (b *MEGTTLBox) SetLow(line int) error {
	if line < 0 || line > 7 {
		return ErrMEGBadLine
	}
	return b.tx(megOpSetLowOnLine, byte(line))
}

// Pulse fires a TTL pulse on the given line for dur, then blocks for dur.
// The device executes the pulse autonomously; the host sleeps to match the
// interface contract of blocking for the full duration.
// Implements [OutputTTLDevice].
func (b *MEGTTLBox) Pulse(line int, dur time.Duration) error {
	if line < 0 || line > 7 {
		return ErrMEGBadLine
	}
	if err := b.setTriggerDuration(dur); err != nil {
		return fmt.Errorf("megttlbox.Pulse: %w", err)
	}
	if err := b.tx(megOpSendTriggerOnLine, byte(line)); err != nil {
		return fmt.Errorf("megttlbox.Pulse: %w", err)
	}
	time.Sleep(dur)
	return nil
}

// PulseMask fires a TTL pulse on every output line with a bit set in mask.
// The device executes the pulse autonomously; the host sleeps for dur.
func (b *MEGTTLBox) PulseMask(mask byte, dur time.Duration) error {
	if err := b.setTriggerDuration(dur); err != nil {
		return fmt.Errorf("megttlbox.PulseMask: %w", err)
	}
	if err := b.tx(megOpSendTriggerMask, mask); err != nil {
		return fmt.Errorf("megttlbox.PulseMask: %w", err)
	}
	time.Sleep(dur)
	return nil
}

// AllLow drives all 8 output lines LOW. Implements [OutputTTLDevice].
func (b *MEGTTLBox) AllLow() error {
	return b.tx(megOpSetLowMask, 0xFF)
}

// Close sets all output lines LOW and closes the serial port.
// Safe to call multiple times. Implements [OutputTTLDevice] and [InputTTLDevice].
func (b *MEGTTLBox) Close() error {
	if b.port == nil {
		return nil
	}
	_ = b.AllLow()
	err := b.port.Close()
	b.port = nil
	return err
}

// --- InputTTLDevice ---

// ReadAll returns the current state of all 8 input lines as a bitmask.
// Bit N reflects line N. Implements [InputTTLDevice].
func (b *MEGTTLBox) ReadAll() (byte, error) {
	if err := b.tx(megOpGetResponseButton); err != nil {
		return 0, fmt.Errorf("megttlbox.ReadAll: %w", err)
	}
	return b.rx1()
}

// ReadLine returns the state (0 or 1) of a single input line (0-indexed).
// Implements [InputTTLDevice].
func (b *MEGTTLBox) ReadLine(line int) (byte, error) {
	if line < 0 || line > 7 {
		return 0, ErrMEGBadLine
	}
	mask, err := b.ReadAll()
	if err != nil {
		return 0, fmt.Errorf("megttlbox.ReadLine: %w", err)
	}
	return (mask >> uint(line)) & 0x01, nil
}

// WaitForInput blocks until any input line becomes active or ctx is cancelled.
// Returns the active-line bitmask and the elapsed reaction time.
// Implements [InputTTLDevice].
func (b *MEGTTLBox) WaitForInput(ctx context.Context) (byte, time.Duration, error) {
	return pollWaitForInput(ctx, "megttlbox", b.ReadAll, b.pollInterval)
}

// DrainInputs polls until all input lines are inactive or ctx is cancelled.
// Call this before [WaitForInput] to clear any latched presses from a previous
// trial. Implements [InputTTLDevice].
func (b *MEGTTLBox) DrainInputs(ctx context.Context) error {
	return pollDrainInputs(ctx, "megttlbox", b.ReadAll, b.pollInterval)
}

// --- FORPButton ---

// FORPButton identifies a button on a Current Designs fiber-optic response pad
// (fORP) wired to the MEGTTLBox input lines at NeuroSpin.
//
// Each constant is the 0-indexed line number, so it doubles as a bit position
// in the bitmask returned by [MEGTTLBox.ReadAll] and [MEGTTLBox.WaitForInput].
type FORPButton uint8

const (
	FORPLeftBlue    FORPButton = 0 // line 0, pin D22, STI007
	FORPLeftYellow  FORPButton = 1 // line 1, pin D23, STI008
	FORPLeftGreen   FORPButton = 2 // line 2, pin D24, STI009
	FORPLeftRed     FORPButton = 3 // line 3, pin D25, STI010
	FORPRightBlue   FORPButton = 4 // line 4, pin D26, STI012
	FORPRightYellow FORPButton = 5 // line 5, pin D27, STI013
	FORPRightGreen  FORPButton = 6 // line 6, pin D28, STI014
	FORPRightRed    FORPButton = 7 // line 7, pin D29, STI015
)

var forpButtonNames = [8]string{
	"left blue", "left yellow", "left green", "left red",
	"right blue", "right yellow", "right green", "right red",
}

// String returns a human-readable button name (e.g. "left blue").
func (b FORPButton) String() string {
	if int(b) < len(forpButtonNames) {
		return forpButtonNames[b]
	}
	return fmt.Sprintf("button%d", b)
}

// DecodeMask converts a button bitmask (as returned by [MEGTTLBox.ReadAll] or
// [MEGTTLBox.WaitForInput]) into a slice of [FORPButton] values, ordered from
// lowest to highest bit.
func DecodeMask(mask byte) []FORPButton {
	var buttons []FORPButton
	for i := 0; i < 8; i++ {
		if mask&(1<<uint(i)) != 0 {
			buttons = append(buttons, FORPButton(i))
		}
	}
	return buttons
}
