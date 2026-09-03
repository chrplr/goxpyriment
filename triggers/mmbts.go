// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"errors"
	"fmt"
	"time"

	"go.bug.st/serial"
)

// NEUROSPEC MMBT-S Interface Box (USB-CDC / virtual COM port).
//
// The protocol is one byte: writing a value 0-255 to the port drives the eight
// TTL lines of the D-Sub 25 output socket, bit N to line N. Nothing is ever
// sent back — the box is write-only, which is why it cannot be probed or
// identified over the wire and why [NewMMBTS] takes the port name explicitly.
//
// D-Sub 25 pinout (manual v2.3 §3.2): pin 2 = bit 1 … pin 9 = bit 8, pins
// 20-25 = ground, the rest not connected. 5 V HIGH, 0 V LOW.
//
// The green LED beside the socket follows bit 1, so it lights on odd codes.
// That is the only feedback the box gives, and it is enough to tell a working
// link from a dead one without an amplifier.

const (
	// mmbtsBaudRate is fixed by the firmware. Do not make this configurable:
	// the box is an Arduino Micro (32u4), and opening its port at 1200 baud is
	// the bootloader touch — the device resets and disappears from the system
	// until it is replugged (manual v2.3 §3.3).
	mmbtsBaudRate = 9600

	// mmbtsFirmwarePulse is how long the firmware holds the lines in Pulse
	// mode before clearing them to zero by itself (manual v2.3 §2.3.2).
	mmbtsFirmwarePulse = 8 * time.Millisecond
)

// MMBTSMode is the runtime mode the MMBT-S is running in. It is selected by
// the hardware switch next to the USB-C socket, read by the box at reset, and
// cannot be queried over the serial link: this value tells the driver what the
// switch is set to, it does not set it.
type MMBTSMode int

const (
	// MMBTSPulseMode is the factory setting, switch at "P": the firmware
	// clears the output port 8 ms after each byte, so every trigger is 8 ms
	// wide whatever duration the caller asks for, and triggers issued closer
	// together than that are queued and delayed 8 ms each.
	MMBTSPulseMode MMBTSMode = iota

	// MMBTSSimpleMode is the switch at "S": a byte latches on the output port
	// until the next one is written, and writing 0 pulls every line LOW. The
	// host controls the pulse width; the firmware's ceiling is about 5 kHz.
	MMBTSSimpleMode
)

// String returns "pulse" or "simple".
func (m MMBTSMode) String() string {
	switch m {
	case MMBTSPulseMode:
		return "pulse"
	case MMBTSSimpleMode:
		return "simple"
	}
	return fmt.Sprintf("MMBTSMode(%d)", int(m))
}

var (
	ErrMMBTSNotOpen = errors.New("mmbts: port not open")
	ErrMMBTSBadLine = errors.New("mmbts: line out of range (0–7)")
	ErrMMBTSBadMode = errors.New("mmbts: unknown runtime mode")
)

// MMBTS controls a NEUROSPEC MMBT-S interface box over its USB-CDC serial
// port. It implements [OutputTTLDevice]; the box has no inputs, so it does not
// implement [InputTTLDevice]. Construct it with [NewMMBTS].
//
// Two properties of the hardware leak into the API and are worth knowing
// before wiring an experiment to it:
//
//   - In Pulse mode (the factory setting) the pulse width is 8 ms, fixed in
//     firmware. [MMBTS.Pulse], [FireTrigger] and [FireTriggerSync] cannot
//     shorten or lengthen it; the duration they are given only controls how
//     long the call blocks. Set the switch to "S" and pass
//     [WithMMBTSMode](MMBTSSimpleMode) to control the width from the host.
//   - The driver cannot see the P/S switch. If [WithMMBTSMode] disagrees with
//     the switch, nothing fails — the lines simply behave differently from
//     what the code says, so check the switch before a session.
type MMBTS struct {
	port       serial.Port
	mode       MMBTSMode
	pulseWidth time.Duration // firmware auto-clear delay, Pulse mode only
	mask       byte          // shadow of the eight output lines
	maskAt     time.Time     // when mask was written, for the Pulse-mode expiry
}

// MMBTSOption configures an [MMBTS] at construction.
type MMBTSOption func(*MMBTS)

// WithMMBTSMode tells the driver which runtime mode the box's P/S switch is
// set to. The default is [MMBTSPulseMode], the factory setting.
func WithMMBTSMode(m MMBTSMode) MMBTSOption {
	return func(b *MMBTS) { b.mode = m }
}

// WithMMBTSPulseWidth overrides the 8 ms firmware pulse width assumed in
// [MMBTSPulseMode]. It changes what the driver believes, not what the box
// does: use it only with firmware that clears the port after some other delay.
func WithMMBTSPulseWidth(d time.Duration) MMBTSOption {
	return func(b *MMBTS) { b.pulseWidth = d }
}

// NewMMBTS opens the MMBT-S on the given serial port — "/dev/ttyACM0" or
// "/dev/tty.usbmodemXXXX" on Unix, "COM4" on Windows. [AvailablePorts] lists
// the candidates; the box enumerates as a generic Arduino Micro, so it cannot
// be told apart from another Arduino-based box by its USB identifiers.
//
// The output port of the box is inactive until a host opens it, so the
// constructor writes a zero to leave every line LOW and the box in the
// documented initial state.
func NewMMBTS(portPath string, opts ...MMBTSOption) (*MMBTS, error) {
	mode := &serial.Mode{
		BaudRate: mmbtsBaudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	p, err := serial.Open(portPath, mode)
	if err != nil {
		return nil, fmt.Errorf("mmbts: open %s: %w", portPath, err)
	}
	// DTR asserted, RTS deasserted — the settings NEUROSPEC documents for
	// Presentation (manual v2.3 §4.4.1). Unlike the Uno, the 32u4 does not
	// reset when DTR is asserted, so no settling delay is needed here.
	if err := p.SetDTR(true); err != nil {
		p.Close()
		return nil, fmt.Errorf("mmbts: set DTR on %s: %w", portPath, err)
	}
	if err := p.SetRTS(false); err != nil {
		p.Close()
		return nil, fmt.Errorf("mmbts: clear RTS on %s: %w", portPath, err)
	}

	b := &MMBTS{port: p, mode: MMBTSPulseMode, pulseWidth: mmbtsFirmwarePulse}
	for _, opt := range opts {
		opt(b)
	}
	if b.mode != MMBTSPulseMode && b.mode != MMBTSSimpleMode {
		p.Close()
		return nil, fmt.Errorf("mmbts: open %s: %w: %d", portPath, ErrMMBTSBadMode, int(b.mode))
	}
	if b.pulseWidth <= 0 {
		p.Close()
		return nil, fmt.Errorf("mmbts: open %s: pulse width %v is not positive", portPath, b.pulseWidth)
	}
	if err := b.AllLow(); err != nil {
		p.Close()
		return nil, fmt.Errorf("mmbts: open %s: %w", portPath, err)
	}
	return b, nil
}

// Mode returns the runtime mode the driver was told the box is in.
func (b *MMBTS) Mode() MMBTSMode { return b.mode }

// PulseWidth returns the firmware pulse width assumed in [MMBTSPulseMode]. It
// is meaningless in [MMBTSSimpleMode], where the host sets the width.
func (b *MMBTS) PulseWidth() time.Duration { return b.pulseWidth }

// current returns the mask the output lines are actually holding. In Pulse
// mode the firmware clears the port by itself, so a shadow older than the
// pulse width describes lines that have already gone LOW; reusing it would
// resurrect a trigger code that the recording never saw a second time.
func (b *MMBTS) current() byte {
	if b.mode == MMBTSPulseMode && time.Since(b.maskAt) >= b.pulseWidth {
		return 0
	}
	return b.mask
}

// Send writes mask to the eight output lines in one byte, bit N driving line
// N. Implements [OutputTTLDevice].
//
// In Pulse mode the firmware clears the lines 8 ms later, and a mask sent less
// than 8 ms after the previous one is queued behind it rather than replacing
// it: 8 ms is the shortest usable interval between codes.
func (b *MMBTS) Send(mask byte) error {
	if b.port == nil {
		return ErrMMBTSNotOpen
	}
	if _, err := b.port.Write([]byte{mask}); err != nil {
		return fmt.Errorf("mmbts.Send: writing 0x%02X: %w", mask, err)
	}
	b.mask = mask
	b.maskAt = time.Now()
	return nil
}

// SetHigh drives a single output line HIGH, leaving the others as they are.
// line is 0-indexed (0–7). Implements [OutputTTLDevice].
//
// The box takes a whole byte rather than per-line commands, so this rewrites
// the full mask from the driver's shadow of it.
func (b *MMBTS) SetHigh(line int) error {
	if line < 0 || line > 7 {
		return ErrMMBTSBadLine
	}
	return b.Send(b.current() | 1<<uint(line))
}

// SetLow drives a single output line LOW, leaving the others as they are.
// line is 0-indexed (0–7). Implements [OutputTTLDevice].
func (b *MMBTS) SetLow(line int) error {
	if line < 0 || line > 7 {
		return ErrMMBTSBadLine
	}
	return b.Send(b.current() &^ (1 << uint(line)))
}

// Pulse drives line HIGH then LOW, blocking for the whole pulse.
// Implements [OutputTTLDevice].
//
// In Simple mode dur is the width, timed by the host. In Pulse mode the width
// is the firmware's 8 ms and dur cannot change it; the call still blocks for
// at least that long, so that a following write cannot race the automatic
// clear, and for dur when dur is longer.
func (b *MMBTS) Pulse(line int, dur time.Duration) error {
	if line < 0 || line > 7 {
		return ErrMMBTSBadLine
	}
	if b.mode == MMBTSSimpleMode {
		return defaultPulse(b, line, dur)
	}
	if err := b.Send(1 << uint(line)); err != nil {
		return fmt.Errorf("mmbts.Pulse: %w", err)
	}
	time.Sleep(max(b.pulseWidth, dur))
	return nil
}

// AllLow drives all eight output lines LOW. Implements [OutputTTLDevice].
func (b *MMBTS) AllLow() error { return b.Send(0x00) }

// Close drives every line LOW and closes the serial port. Safe to call more
// than once. Implements [OutputTTLDevice].
func (b *MMBTS) Close() error {
	if b.port == nil {
		return nil
	}
	_ = b.AllLow()
	err := b.port.Close()
	b.port = nil
	return err
}
