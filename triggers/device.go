// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Package triggers provides hardware trigger interfaces for synchronising
// stimuli with external recording equipment (EEG/MEG amplifiers, oscilloscopes,
// photodiodes), and for reading TTL-level response signals.
//
// Two interfaces cover the two directions of TTL signalling:
//   - [OutputTTLDevice] — send trigger codes to recording equipment
//   - [InputTTLDevice]  — read TTL inputs from response hardware
//
// Implementations:
//   - [DLPIO8]              — DLP-IO8-G digital I/O over USB-CDC serial (output + input)
//   - [MEGTTLBox]           — NeuroSpin Arduino-based TTL box (output + input)
//   - [ParallelPort]        — LPT parallel port, Linux only (output only)
//   - [NullOutputTTLDevice] — silent no-op output (safe default when no device present)
//   - [NullInputTTLDevice]  — silent no-op input
//
// Lines are 0-indexed (0–7) throughout. Bit N of a bitmask corresponds to line N.
package triggers

import (
	"context"
	"time"
)

// OutputTTLDevice is the common interface for hardware devices that send TTL
// trigger signals to external recording equipment.
//
// Lines are 0-indexed (0–7). Bit N of a bitmask drives line N.
type OutputTTLDevice interface {
	// Send sets all 8 output lines simultaneously from a bitmask.
	// Bit N drives line N HIGH; a zero bit drives it LOW.
	Send(mask byte) error

	// SetHigh drives a single output line HIGH. Lines are 0-indexed (0–7).
	SetHigh(line int) error

	// SetLow drives a single output line LOW. Lines are 0-indexed (0–7).
	SetLow(line int) error

	// Pulse drives line HIGH for d, then LOW. Blocks for the full duration.
	Pulse(line int, d time.Duration) error

	// AllLow drives all 8 output lines LOW.
	AllLow() error

	// Close sets all lines LOW and releases the device.
	Close() error
}

// InputTTLDevice is the common interface for hardware devices that read TTL
// input signals (response buttons, FORP pads, etc.).
//
// Lines are 0-indexed (0–7). Bit N of a returned mask corresponds to line N.
type InputTTLDevice interface {
	// ReadAll returns the current state of all 8 input lines as a bitmask.
	ReadAll() (byte, error)

	// ReadLine returns the state (0 or 1) of a single input line (0-indexed).
	ReadLine(line int) (byte, error)

	// WaitForInput blocks until any input line becomes active or ctx is
	// cancelled. Returns the active-line bitmask and the reaction time
	// (elapsed from call to first detected input).
	WaitForInput(ctx context.Context) (mask byte, rt time.Duration, err error)

	// DrainInputs polls until all input lines are inactive or ctx is
	// cancelled. Use this to clear latched presses between trials.
	DrainInputs(ctx context.Context) error

	// Close releases the device.
	Close() error
}

// defaultPulse is a shared Pulse implementation built on SetHigh/SetLow.
// Used by DLPIO8 and ParallelPort.
func defaultPulse(d OutputTTLDevice, line int, dur time.Duration) error {
	if err := d.SetHigh(line); err != nil {
		return err
	}
	time.Sleep(dur)
	return d.SetLow(line)
}

// NullOutputTTLDevice is a no-op [OutputTTLDevice]. It is returned by
// [AutoDetectDLPIO8] when no device is found, so callers never need to
// nil-check the result.
type NullOutputTTLDevice struct{}

func (NullOutputTTLDevice) Send(_ byte) error                  { return nil }
func (NullOutputTTLDevice) SetHigh(_ int) error                { return nil }
func (NullOutputTTLDevice) SetLow(_ int) error                 { return nil }
func (NullOutputTTLDevice) Pulse(_ int, _ time.Duration) error { return nil }
func (NullOutputTTLDevice) AllLow() error                      { return nil }
func (NullOutputTTLDevice) Close() error                       { return nil }

// NullInputTTLDevice is a no-op [InputTTLDevice].
type NullInputTTLDevice struct{}

func (NullInputTTLDevice) ReadAll() (byte, error) { return 0, nil }
func (NullInputTTLDevice) ReadLine(_ int) (byte, error) {
	return 0, nil
}
func (NullInputTTLDevice) WaitForInput(_ context.Context) (byte, time.Duration, error) {
	return 0, 0, nil
}
func (NullInputTTLDevice) DrainInputs(_ context.Context) error { return nil }
func (NullInputTTLDevice) Close() error                        { return nil }
