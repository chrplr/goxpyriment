// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

// Package triggers provides hardware trigger interfaces for synchronising
// stimuli with external recording equipment (EEG amplifiers, oscilloscopes,
// photodiodes).
//
// The central abstraction is the [Trigger] interface, implemented by:
//   - [DLPIO8] — DLP-IO8 / DLP-IO8-G digital I/O via USB-CDC serial
//   - [ParallelPort] — LPT parallel port (Linux only; returns error elsewhere)
//   - [NullTrigger] — silent no-op, used when no device is present
//
// Also provided:
//   - [SerialPort] — generic serial device (moved from the io package)
//   - [AutoDetectDLPIO8] — scans available ports and returns the first DLP-IO8-G found
package triggers

import "time"

// Trigger is the common interface for all hardware trigger output devices.
//
// All methods are safe to call on a [NullTrigger] (no-op, no error returned).
type Trigger interface {
	// Send sets all 8 output lines simultaneously using a bitmask.
	// Bit 0 = pin 1, bit 7 = pin 8. Suitable for sending EEG event codes.
	Send(value byte) error

	// SetHigh drives a single pin HIGH. Pins are 1-indexed (1–8).
	SetHigh(pin int) error

	// SetLow drives a single pin LOW. Pins are 1-indexed (1–8).
	SetLow(pin int) error

	// Pulse drives pin HIGH for durationMs milliseconds, then LOW.
	// Blocks for the full duration.
	Pulse(pin int, durationMs int) error

	// Close sets all lines LOW and releases all resources.
	Close() error
}

// defaultPulse is a shared Pulse implementation in terms of SetHigh / SetLow.
// Used internally by DLPIO8 and ParallelPort.
func defaultPulse(t Trigger, pin int, durationMs int) error {
	if err := t.SetHigh(pin); err != nil {
		return err
	}
	time.Sleep(time.Duration(durationMs) * time.Millisecond)
	return t.SetLow(pin)
}

// NullTrigger is a no-op implementation of [Trigger]. It is returned by
// [AutoDetectDLPIO8] when no device is found, so callers never need to
// nil-check the result.
type NullTrigger struct{}

func (NullTrigger) Send(_ byte) error        { return nil }
func (NullTrigger) SetHigh(_ int) error      { return nil }
func (NullTrigger) SetLow(_ int) error       { return nil }
func (NullTrigger) Pulse(_ int, _ int) error { return nil }
func (NullTrigger) Close() error             { return nil }
