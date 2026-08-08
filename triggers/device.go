// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

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
//   - [DLPIO20]             — DLP-IO20 20-channel module over USB-CDC serial (output + input)
//   - [MEGTTLBox]           — NeuroSpin Arduino-based TTL box (output + input)
//   - [FT232HTrigger]       — Adafruit FT232H via MPSSE GPIO, Linux only (output + input)
//   - [LinuxGPIOTrigger]    — Linux GPIO char device (RPi, Rock Pi, …), Linux only (output + input)
//   - [LabJackT4]           — LabJack T4 via Modbus TCP, cross-platform (output + input)
//   - [ParallelPort]        — LPT parallel port, Linux only (output only)
//   - [NullOutputTTLDevice] — silent no-op output (safe default when no device present)
//   - [NullInputTTLDevice]  — silent no-op input
//
// Lines are 0-indexed (0–7) throughout. Bit N of a bitmask corresponds to line N.
package triggers

import (
	"context"
	"fmt"
	"log"
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

// FireTrigger pulses line HIGH for dur then LOW, silently discarding errors.
// It has no return value so it can be launched directly as a goroutine
// immediately after a VSYNC flip, keeping the frame loop free of blocking
// serial I/O:
//
//	go triggers.FireTrigger(trig, pin, 5*time.Millisecond)
//
// The pulse duration is controlled by precisionSleep, which busy-spins the
// last 500 µs to eliminate OS scheduling overshoot from time.Sleep alone.
func FireTrigger(d OutputTTLDevice, line int, dur time.Duration) {
	// Report a failure rather than discarding it. Silently doing nothing is the
	// worst behaviour available here: a trigger that never fires looks exactly
	// like a trigger the recording equipment missed, and the run is only found
	// to be worthless afterwards. An out-of-range line -- which is what passing
	// a 1-8 pin number to this 0-7 parameter produces at pin 8 -- used to fail
	// exactly that way.
	if err := d.SetHigh(line); err != nil {
		log.Printf("triggers.FireTrigger: raising line %d: %v", line, err)
		return
	}
	precisionSleep(dur)
	if err := d.SetLow(line); err != nil {
		log.Printf("triggers.FireTrigger: lowering line %d: %v", line, err)
	}
}

// precisionSleep sleeps for approximately dur with sub-millisecond accuracy.
// It delegates most of the wait to time.Sleep then busy-spins the final 500 µs
// to absorb OS scheduling jitter, which can otherwise cause time.Sleep to
// overshoot by several milliseconds on a loaded system.
func precisionSleep(dur time.Duration) {
	deadline := time.Now().Add(dur)
	if dur > 500*time.Microsecond {
		time.Sleep(dur - 500*time.Microsecond)
	}
	for time.Now().Before(deadline) {
		// busy-spin
	}
}

// defaultPulse is a shared Pulse implementation built on SetHigh/SetLow.
// Used by every OutputTTLDevice except MEGTTLBox, which pulses autonomously
// in firmware.
func defaultPulse(d OutputTTLDevice, line int, dur time.Duration) error {
	if err := d.SetHigh(line); err != nil {
		return fmt.Errorf("triggers: pulse SetHigh on line %d: %w", line, err)
	}
	time.Sleep(dur)
	return d.SetLow(line)
}

// readLineFromMask is a shared ReadLine implementation for devices whose
// per-line state is derived from the full input bitmask returned by readAll.
// name is the device prefix used in error messages (e.g. "ft232h"). Devices
// that read a single line directly from hardware (DLPIO8) or use a sentinel
// range error (MEGTTLBox) implement ReadLine themselves instead.
func readLineFromMask(name string, readAll func() (byte, error), line int) (byte, error) {
	if line < 0 || line > 7 {
		return 0, fmt.Errorf("%s: line %d out of range (0–7)", name, line)
	}
	mask, err := readAll()
	if err != nil {
		return 0, fmt.Errorf("%s.ReadLine: %w", name, err)
	}
	return (mask >> uint(line)) & 0x01, nil
}

// pollWaitForInput is the shared [InputTTLDevice.WaitForInput] loop: it polls
// readAll every poll interval until a nonzero mask appears or ctx is cancelled,
// returning the active-line mask and the elapsed reaction time. name is the
// device prefix used in error messages.
func pollWaitForInput(ctx context.Context, name string, readAll func() (byte, error), poll time.Duration) (byte, time.Duration, error) {
	start := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return 0, time.Since(start), err
		}
		mask, err := readAll()
		if err != nil {
			return 0, time.Since(start), fmt.Errorf("%s.WaitForInput: reading inputs: %w", name, err)
		}
		if mask != 0 {
			return mask, time.Since(start), nil
		}
		time.Sleep(poll)
	}
}

// pollDrainInputs is the shared [InputTTLDevice.DrainInputs] loop: it polls
// readAll every poll interval until all input lines are inactive or ctx is
// cancelled. name is the device prefix used in error messages.
func pollDrainInputs(ctx context.Context, name string, readAll func() (byte, error), poll time.Duration) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		mask, err := readAll()
		if err != nil {
			return fmt.Errorf("%s.DrainInputs: reading inputs: %w", name, err)
		}
		if mask == 0 {
			return nil
		}
		time.Sleep(poll)
	}
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
