// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/goburrow/modbus"
)

const (
	t4DefaultPollInterval      = 5 * time.Millisecond
	t4DefaultTimeout           = 1 * time.Second
	t4DefaultUnitID       byte = 1

	// 32-bit (2 Modbus registers each) DIO bitmask registers of the LabJack
	// T-series Modbus map. Bit N of each value refers to DIO N.
	t4RegDIOState        uint16 = 2800 // read/write the level of every DIO
	t4RegDIODirection    uint16 = 2850 // 0 = input, 1 = output
	t4RegDIOAnalogEnable uint16 = 2880 // T4 only: 1 = analog, 0 = digital
	t4RegDIOInhibit      uint16 = 2900 // 1 = ignore writes to that DIO

	// t4NumDIO is the width of the DIO bitmask registers (DIO0–DIO22).
	t4NumDIO     = 23
	t4DIOMask    = uint32(1)<<t4NumDIO - 1
	t4LinesPerIO = 8

	// Default line groups on a T4. DIO0–DIO3 do not exist as digital lines
	// (they are the dedicated AIN0–AIN3 screw terminals), so the outputs
	// start at DIO4.
	t4DefaultOutputBase = 4  // DIO4–DIO11  = FIO4–FIO7 + EIO0–EIO3
	t4DefaultInputBase  = 12 // DIO12–DIO19 = EIO4–EIO7 + CIO0–CIO3
)

// LabJackT4 controls a LabJack T4 DAQ device over Modbus TCP.
//
// Default wiring — the T4 has 16 digital lines, DIO4–DIO19:
//   - outputs: DIO4–DIO11 = FIO4–FIO7 (screw terminals) + EIO0–EIO3 (DB15)
//   - inputs:  DIO12–DIO19 = EIO4–EIO7 + CIO0–CIO3 (DB15)
//
// DIO0–DIO3 are *not* usable: on the T4 those are the dedicated analog inputs
// AIN0–AIN3. DIO4–DIO11 are "flexible I/O" that power up as analog inputs;
// [NewLabJackT4] switches them to digital mode via DIO_ANALOG_ENABLE.
// Use [WithT4OutputBase] / [WithT4InputBase] for a different pin assignment.
//
// Implements both [OutputTTLDevice] and [InputTTLDevice].
//
// Usage:
//
//	box, err := triggers.NewLabJackT4("192.168.1.100")
//	if err != nil { log.Fatal(err) }
//	defer box.Close()
//
//	box.Pulse(0, 5*time.Millisecond)
//	mask, rt, _ := box.WaitForInput(ctx)
type LabJackT4 struct {
	handler      *modbus.TCPClientHandler
	client       modbus.Client
	outputState  byte
	outputBase   int
	inputBase    int
	pollInterval time.Duration
}

// LabJackT4Option configures a [LabJackT4] at construction time.
type LabJackT4Option func(*modbus.TCPClientHandler, *LabJackT4)

// WithT4PollInterval sets the polling interval used by [LabJackT4.WaitForInput]
// and [LabJackT4.DrainInputs]. Default: 5 ms.
func WithT4PollInterval(d time.Duration) LabJackT4Option {
	return func(_ *modbus.TCPClientHandler, t *LabJackT4) { t.pollInterval = d }
}

// WithT4Timeout sets the Modbus TCP request timeout. Default: 1 s.
func WithT4Timeout(d time.Duration) LabJackT4Option {
	return func(h *modbus.TCPClientHandler, _ *LabJackT4) { h.Timeout = d }
}

// WithT4UnitID sets the Modbus unit/slave ID. Default: 1.
func WithT4UnitID(id byte) LabJackT4Option {
	return func(h *modbus.TCPClientHandler, _ *LabJackT4) { h.SlaveId = id }
}

// WithT4OutputBase sets the DIO number driven by output line 0; lines 0–7 then
// map to dio…dio+7. Default: 4 (FIO4–FIO7 + EIO0–EIO3).
func WithT4OutputBase(dio int) LabJackT4Option {
	return func(_ *modbus.TCPClientHandler, t *LabJackT4) { t.outputBase = dio }
}

// WithT4InputBase sets the DIO number read as input line 0; lines 0–7 then map
// to dio…dio+7. Default: 12 (EIO4–EIO7 + CIO0–CIO3).
func WithT4InputBase(dio int) LabJackT4Option {
	return func(_ *modbus.TCPClientHandler, t *LabJackT4) { t.inputBase = dio }
}

// NewLabJackT4 connects to a LabJack T4 at host (e.g. "192.168.1.100" or
// "192.168.1.100:502"), configures the output group as digital outputs (all
// LOW) and the input group as digital inputs, and returns a ready device.
func NewLabJackT4(host string, opts ...LabJackT4Option) (*LabJackT4, error) {
	if !strings.Contains(host, ":") {
		host = fmt.Sprintf("%s:502", host)
	}

	handler := modbus.NewTCPClientHandler(host)
	handler.Timeout = t4DefaultTimeout
	handler.SlaveId = t4DefaultUnitID

	t := &LabJackT4{
		handler:      handler,
		outputBase:   t4DefaultOutputBase,
		inputBase:    t4DefaultInputBase,
		pollInterval: t4DefaultPollInterval,
	}
	for _, opt := range opts {
		opt(handler, t)
	}
	if err := t.checkBases(); err != nil {
		return nil, err
	}

	if err := handler.Connect(); err != nil {
		return nil, fmt.Errorf("labjackt4: connect %s: %w", host, err)
	}
	t.client = modbus.NewClient(handler)

	if err := t.configure(); err != nil {
		handler.Close()
		return nil, err
	}
	return t, nil
}

// checkBases validates the output/input DIO groups before any I/O is attempted.
func (t *LabJackT4) checkBases() error {
	for _, g := range []struct {
		name string
		base int
	}{{"output", t.outputBase}, {"input", t.inputBase}} {
		if g.base < 0 || g.base+t4LinesPerIO > t4NumDIO {
			return fmt.Errorf("labjackt4: %s base DIO%d out of range (0–%d)",
				g.name, g.base, t4NumDIO-t4LinesPerIO)
		}
	}
	if t.outputMask()&t.inputMask() != 0 {
		return fmt.Errorf("labjackt4: output group DIO%d–DIO%d overlaps input group DIO%d–DIO%d",
			t.outputBase, t.outputBase+7, t.inputBase, t.inputBase+7)
	}
	return nil
}

func (t *LabJackT4) outputMask() uint32 { return 0xFF << uint(t.outputBase) }
func (t *LabJackT4) inputMask() uint32  { return 0xFF << uint(t.inputBase) }

// configure puts the two line groups in digital mode, sets their directions and
// drives every output LOW.
//
// The order matters. DIO_INHIBIT filters writes to DIO_STATE, DIO_DIRECTION and
// DIO_ANALOG_ENABLE, so it is first cleared for the 16 lines we own (and left
// set for every other DIO, which on a T4 includes the analog-only DIO0–DIO3).
// The flexible lines DIO4–DIO11 power up as analog inputs and ignore digital
// writes until DIO_ANALOG_ENABLE is cleared, so that comes before the direction
// write. Finally the inhibit mask is narrowed to the outputs alone, so that a
// later Send cannot disturb the input lines.
func (t *LabJackT4) configure() error {
	owned := t.outputMask() | t.inputMask()

	steps := []struct {
		what  string
		reg   uint16
		value uint32
	}{
		{"clear DIO inhibit", t4RegDIOInhibit, ^owned & t4DIOMask},
		{"set digital mode", t4RegDIOAnalogEnable, 0},
		{"set DIO direction", t4RegDIODirection, t.outputMask()},
		{"init output state", t4RegDIOState, 0},
		{"protect input lines", t4RegDIOInhibit, ^t.outputMask() & t4DIOMask},
	}
	for _, s := range steps {
		if err := t.writeUint32(s.reg, s.value); err != nil {
			return fmt.Errorf("labjackt4: %s: %w", s.what, err)
		}
	}
	return nil
}

// writeUint32 writes a 32-bit LabJack register (2 consecutive Modbus registers,
// big-endian) with FC16.
func (t *LabJackT4) writeUint32(reg uint16, value uint32) error {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], value)
	_, err := t.client.WriteMultipleRegisters(reg, 2, b[:])
	return err
}

// readUint32 reads a 32-bit LabJack register (2 consecutive Modbus registers,
// big-endian) with FC3.
func (t *LabJackT4) readUint32(reg uint16) (uint32, error) {
	results, err := t.client.ReadHoldingRegisters(reg, 2)
	if err != nil {
		return 0, err
	}
	if len(results) < 4 {
		return 0, fmt.Errorf("short read: %d bytes, want 4", len(results))
	}
	return binary.BigEndian.Uint32(results), nil
}

// --- OutputTTLDevice ---

// Send sets all 8 output lines simultaneously from a bitmask. Bit N drives
// line N HIGH; zero drives it LOW. Implements [OutputTTLDevice].
func (t *LabJackT4) Send(mask byte) error {
	if err := t.writeUint32(t4RegDIOState, uint32(mask)<<uint(t.outputBase)); err != nil {
		return fmt.Errorf("labjackt4: Send: %w", err)
	}
	t.outputState = mask
	return nil
}

// SetHigh drives a single output line HIGH. line is 0-indexed (0–7).
// Implements [OutputTTLDevice].
func (t *LabJackT4) SetHigh(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("labjackt4: line %d out of range (0–7)", line)
	}
	return t.Send(t.outputState | (1 << uint(line)))
}

// SetLow drives a single output line LOW. line is 0-indexed (0–7).
// Implements [OutputTTLDevice].
func (t *LabJackT4) SetLow(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("labjackt4: line %d out of range (0–7)", line)
	}
	return t.Send(t.outputState &^ (1 << uint(line)))
}

// Pulse drives line HIGH for dur, then LOW. Blocks for the full duration.
// Implements [OutputTTLDevice].
func (t *LabJackT4) Pulse(line int, dur time.Duration) error {
	return defaultPulse(t, line, dur)
}

// AllLow drives all 8 output lines LOW. Implements [OutputTTLDevice].
func (t *LabJackT4) AllLow() error { return t.Send(0x00) }

// Close sets all output lines LOW and closes the Modbus TCP connection.
// Safe to call multiple times.
func (t *LabJackT4) Close() error {
	_ = t.AllLow()
	return t.handler.Close()
}

// --- InputTTLDevice ---

// ReadAll returns the current state of all 8 input lines as a bitmask.
// Bit N reflects line N. Implements [InputTTLDevice].
func (t *LabJackT4) ReadAll() (byte, error) {
	state, err := t.readUint32(t4RegDIOState)
	if err != nil {
		return 0, fmt.Errorf("labjackt4: ReadAll: %w", err)
	}
	return byte((state >> uint(t.inputBase)) & 0xFF), nil
}

// ReadLine returns the state (0 or 1) of a single input line (0-indexed).
// Implements [InputTTLDevice].
func (t *LabJackT4) ReadLine(line int) (byte, error) {
	return readLineFromMask("labjackt4", t.ReadAll, line)
}

// WaitForInput blocks until any input line becomes active or ctx is cancelled.
// Returns the active-line bitmask and the elapsed reaction time.
// Implements [InputTTLDevice].
func (t *LabJackT4) WaitForInput(ctx context.Context) (byte, time.Duration, error) {
	return pollWaitForInput(ctx, "labjackt4", t.ReadAll, t.pollInterval)
}

// DrainInputs polls until all input lines are inactive or ctx is cancelled.
// Call before [LabJackT4.WaitForInput] to clear latched presses from a previous
// trial. Implements [InputTTLDevice].
func (t *LabJackT4) DrainInputs(ctx context.Context) error {
	return pollDrainInputs(ctx, "labjackt4", t.ReadAll, t.pollInterval)
}
