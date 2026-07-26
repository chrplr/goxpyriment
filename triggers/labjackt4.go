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

	t4RegFIOState     uint16 = 2500
	t4RegEIOState     uint16 = 2501
	t4RegFIODirection uint16 = 2504
	t4RegEIODirection uint16 = 2505
)

// LabJackT4 controls a LabJack T4 DAQ device over Modbus TCP.
//
// Wiring:
//   - FIO0–FIO7 (8 digital I/O configured as outputs) → TTL trigger lines
//   - EIO0–EIO7 (8 extended I/O configured as inputs) → TTL response-pad lines
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

// NewLabJackT4 connects to a LabJack T4 at host (e.g. "192.168.1.100" or
// "192.168.1.100:502"), configures FIO0–FIO7 as outputs (all LOW) and
// EIO0–EIO7 as inputs, and returns a ready device.
func NewLabJackT4(host string, opts ...LabJackT4Option) (*LabJackT4, error) {
	if !strings.Contains(host, ":") {
		host = fmt.Sprintf("%s:502", host)
	}

	handler := modbus.NewTCPClientHandler(host)
	handler.Timeout = t4DefaultTimeout
	handler.SlaveId = t4DefaultUnitID

	t := &LabJackT4{
		handler:      handler,
		pollInterval: t4DefaultPollInterval,
	}
	for _, opt := range opts {
		opt(handler, t)
	}

	if err := handler.Connect(); err != nil {
		return nil, fmt.Errorf("labjackt4: connect %s: %w", host, err)
	}
	t.client = modbus.NewClient(handler)

	// Configure FIO0-FIO7 as outputs, EIO0-EIO7 as inputs
	if _, err := t.client.WriteSingleRegister(t4RegFIODirection, 0x00FF); err != nil {
		handler.Close()
		return nil, fmt.Errorf("labjackt4: set FIO direction: %w", err)
	}
	if _, err := t.client.WriteSingleRegister(t4RegEIODirection, 0x0000); err != nil {
		handler.Close()
		return nil, fmt.Errorf("labjackt4: set EIO direction: %w", err)
	}
	// All FIO outputs LOW
	if _, err := t.client.WriteSingleRegister(t4RegFIOState, 0x0000); err != nil {
		handler.Close()
		return nil, fmt.Errorf("labjackt4: init FIO state: %w", err)
	}
	return t, nil
}

// --- OutputTTLDevice ---

// Send sets all 8 FIO0–FIO7 output lines simultaneously from a bitmask.
// Bit N drives line N HIGH; zero drives it LOW. Implements [OutputTTLDevice].
func (t *LabJackT4) Send(mask byte) error {
	if _, err := t.client.WriteSingleRegister(t4RegFIOState, uint16(mask)); err != nil {
		return fmt.Errorf("labjackt4: Send: %w", err)
	}
	t.outputState = mask
	return nil
}

// SetHigh drives a single FIO output line HIGH. line is 0-indexed (0–7).
// Implements [OutputTTLDevice].
func (t *LabJackT4) SetHigh(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("labjackt4: line %d out of range (0–7)", line)
	}
	return t.Send(t.outputState | (1 << uint(line)))
}

// SetLow drives a single FIO output line LOW. line is 0-indexed (0–7).
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

// AllLow drives all 8 FIO0–FIO7 output lines LOW. Implements [OutputTTLDevice].
func (t *LabJackT4) AllLow() error { return t.Send(0x00) }

// Close sets all output lines LOW and closes the Modbus TCP connection.
// Safe to call multiple times.
func (t *LabJackT4) Close() error {
	_ = t.AllLow()
	return t.handler.Close()
}

// --- InputTTLDevice ---

// ReadAll returns the current state of all 8 EIO0–EIO7 input lines as a
// bitmask. Bit N reflects line N. Implements [InputTTLDevice].
func (t *LabJackT4) ReadAll() (byte, error) {
	results, err := t.client.ReadHoldingRegisters(t4RegEIOState, 1)
	if err != nil {
		return 0, fmt.Errorf("labjackt4: ReadAll: %w", err)
	}
	return byte(binary.BigEndian.Uint16(results)), nil
}

// ReadLine returns the state (0 or 1) of a single EIO input line (0-indexed).
// Implements [InputTTLDevice].
func (t *LabJackT4) ReadLine(line int) (byte, error) {
	return readLineFromMask("labjackt4", t.ReadAll, line)
}

// WaitForInput blocks until any EIO0–EIO7 input line becomes active or ctx is
// cancelled. Returns the active-line bitmask and the elapsed reaction time.
// Implements [InputTTLDevice].
func (t *LabJackT4) WaitForInput(ctx context.Context) (byte, time.Duration, error) {
	return pollWaitForInput(ctx, "labjackt4", t.ReadAll, t.pollInterval)
}

// DrainInputs polls until all EIO0–EIO7 input lines are inactive or ctx is
// cancelled. Call before [LabJackT4.WaitForInput] to clear latched presses
// from a previous trial. Implements [InputTTLDevice].
func (t *LabJackT4) DrainInputs(ctx context.Context) error {
	return pollDrainInputs(ctx, "labjackt4", t.ReadAll, t.pollInterval)
}
