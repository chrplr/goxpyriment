// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package triggers

import (
	"context"
	"errors"
	"fmt"
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
	triggerDurMS    uint16 // last duration sent to device (ms)
	triggerDurKnown bool   // whether triggerDurMS has been sent at least once
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
	megOpSetTriggerDuration = 10 // + uint16 LE (ms)
	megOpSendTriggerMask    = 11 // + uint8 mask — pulse all set lines
	megOpSendTriggerOnLine  = 12 // + uint8 line (0–7)
	megOpSetHighMask        = 13 // + uint8 mask — persistent HIGH
	megOpSetLowMask         = 14 // + uint8 mask — persistent LOW
	megOpSetHighOnLine      = 15 // + uint8 line (0–7)
	megOpSetLowOnLine       = 16 // + uint8 line (0–7)
	megOpGetResponseButton  = 20 // → returns uint8 button mask
)

// Sentinel errors returned by MEGTTLBox methods.
var (
	ErrMEGNotOpen    = errors.New("megttlbox: port not open")
	ErrMEGTimeout    = errors.New("megttlbox: read timeout")
	ErrMEGBadLine    = errors.New("megttlbox: line out of range (0–7)")
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
			return 0, err
		}
	}
	return 0, ErrMEGTimeout
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
		return err
	}
	b.triggerDurMS = v
	b.triggerDurKnown = true
	return nil
}

// --- OutputTTLDevice ---

// Send sets all 8 output lines persistently from a bitmask.
// Bit N drives line N HIGH; a zero bit drives it LOW.
// Implements [OutputTTLDevice].
func (b *MEGTTLBox) Send(mask byte) error {
	if err := b.tx(megOpSetHighMask, mask); err != nil {
		return err
	}
	return b.tx(megOpSetLowMask, ^mask)
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
		return err
	}
	if err := b.tx(megOpSendTriggerOnLine, byte(line)); err != nil {
		return err
	}
	time.Sleep(dur)
	return nil
}

// PulseMask fires a TTL pulse on every output line with a bit set in mask.
// The device executes the pulse autonomously; the host sleeps for dur.
func (b *MEGTTLBox) PulseMask(mask byte, dur time.Duration) error {
	if err := b.setTriggerDuration(dur); err != nil {
		return err
	}
	if err := b.tx(megOpSendTriggerMask, mask); err != nil {
		return err
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
		return 0, err
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
		return 0, err
	}
	return (mask >> uint(line)) & 0x01, nil
}

// WaitForInput blocks until any input line becomes active or ctx is cancelled.
// Returns the active-line bitmask and the elapsed reaction time.
// Implements [InputTTLDevice].
func (b *MEGTTLBox) WaitForInput(ctx context.Context) (byte, time.Duration, error) {
	start := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return 0, time.Since(start), err
		}
		mask, err := b.ReadAll()
		if err != nil {
			return 0, time.Since(start), err
		}
		if mask != 0 {
			return mask, time.Since(start), nil
		}
		time.Sleep(b.pollInterval)
	}
}

// DrainInputs polls until all input lines are inactive or ctx is cancelled.
// Call this before [WaitForInput] to clear any latched presses from a previous
// trial. Implements [InputTTLDevice].
func (b *MEGTTLBox) DrainInputs(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		mask, err := b.ReadAll()
		if err != nil {
			return err
		}
		if mask == 0 {
			return nil
		}
		time.Sleep(b.pollInterval)
	}
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
