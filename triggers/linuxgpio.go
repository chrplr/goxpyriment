// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"context"
	"fmt"
	"time"
)

// LinuxGPIOTrigger controls a set of Linux GPIO lines (via /dev/gpiochipN)
// as TTL trigger output and/or input. It works on any Linux single-board
// computer with GPIO — Raspberry Pi, Rock Pi, BeagleBone, Jetson, etc.
//
// Up to 8 output lines and 8 input lines are supported, each represented as a
// byte bitmask: bit N corresponds to the Nth pin in the configured pin array.
//
// The device is accessed directly via the Linux GPIO character device API v2
// (kernel ≥ 5.10). No external libraries or kernel modules are required; the
// user needs rw access to /dev/gpiochip0 (typically via the gpio group).
//
// Usage (Raspberry Pi, BCM pin numbers):
//
//	box, err := triggers.NewLinuxGPIOTrigger(
//	    triggers.WithGPIOOutputPins([8]int{17, 27, 22, 5, 6, 13, 19, 26}),
//	    triggers.WithGPIOInputPins([8]int{12, 16, 20, 21, 4, 25, 24, 23}),
//	)
//	if err != nil { log.Fatal(err) }
//	defer box.Close()
//
//	box.Pulse(0, 5*time.Millisecond)
//	mask, rt, _ := box.WaitForInput(ctx)
//
// At least one of [WithGPIOOutputPins] or [WithGPIOInputPins] must be given.
// Output-only and input-only configurations are both valid.
// Implements [OutputTTLDevice] and [InputTTLDevice].
type LinuxGPIOTrigger struct {
	chipPath     string
	outputPins   *[8]int // nil = output not configured
	inputPins    *[8]int // nil = input not configured
	handle       gpioHandle
	outputState  byte
	pollInterval time.Duration
}

// LinuxGPIOOption configures a [LinuxGPIOTrigger] at construction time.
type LinuxGPIOOption func(*LinuxGPIOTrigger)

// WithGPIOChip sets the GPIO chip device path. Default: "/dev/gpiochip0".
func WithGPIOChip(path string) LinuxGPIOOption {
	return func(t *LinuxGPIOTrigger) { t.chipPath = path }
}

// WithGPIOOutputPins sets the 8 GPIO pin numbers used as TTL output lines.
// Pin numbers are chip-relative offsets (BCM numbers on Raspberry Pi).
// Bit 0 of [LinuxGPIOTrigger.Send] bitmask drives pins[0]; bit 7 drives pins[7].
func WithGPIOOutputPins(pins [8]int) LinuxGPIOOption {
	return func(t *LinuxGPIOTrigger) { t.outputPins = &pins }
}

// WithGPIOInputPins sets the 8 GPIO pin numbers used as TTL input lines.
// Pin numbers are chip-relative offsets (BCM numbers on Raspberry Pi).
// Bit 0 of [LinuxGPIOTrigger.ReadAll] bitmask reflects pins[0]; bit 7 reflects pins[7].
func WithGPIOInputPins(pins [8]int) LinuxGPIOOption {
	return func(t *LinuxGPIOTrigger) { t.inputPins = &pins }
}

// WithGPIOPollInterval sets the polling interval for [LinuxGPIOTrigger.WaitForInput]
// and [LinuxGPIOTrigger.DrainInputs]. Default: 5 ms.
func WithGPIOPollInterval(d time.Duration) LinuxGPIOOption {
	return func(t *LinuxGPIOTrigger) { t.pollInterval = d }
}

const gpioDefaultPollInterval = 5 * time.Millisecond

// NewLinuxGPIOTrigger opens the GPIO chip and claims the configured lines.
// At least one of [WithGPIOOutputPins] or [WithGPIOInputPins] must be supplied.
func NewLinuxGPIOTrigger(opts ...LinuxGPIOOption) (*LinuxGPIOTrigger, error) {
	t := &LinuxGPIOTrigger{
		chipPath:     "/dev/gpiochip0",
		pollInterval: gpioDefaultPollInterval,
	}
	for _, opt := range opts {
		opt(t)
	}
	if t.outputPins == nil && t.inputPins == nil {
		return nil, fmt.Errorf("linuxgpio: at least one of WithGPIOOutputPins or WithGPIOInputPins must be provided")
	}
	if err := t.open(); err != nil {
		return nil, fmt.Errorf("triggers.NewLinuxGPIOTrigger: %w", err)
	}
	return t, nil
}

// --- OutputTTLDevice ---

// Send sets all 8 output lines simultaneously from a bitmask.
// Bit N drives outputPins[N] HIGH; zero drives it LOW. Implements [OutputTTLDevice].
func (t *LinuxGPIOTrigger) Send(mask byte) error {
	if t.outputPins == nil {
		return fmt.Errorf("linuxgpio: output pins not configured")
	}
	if err := t.gpioWriteOutputs(mask); err != nil {
		return fmt.Errorf("linuxgpio.Send: %w", err)
	}
	t.outputState = mask
	return nil
}

// SetHigh drives a single output line HIGH. line is 0-indexed (0–7).
// Implements [OutputTTLDevice].
func (t *LinuxGPIOTrigger) SetHigh(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("linuxgpio: line %d out of range (0–7)", line)
	}
	return t.Send(t.outputState | (1 << uint(line)))
}

// SetLow drives a single output line LOW. line is 0-indexed (0–7).
// Implements [OutputTTLDevice].
func (t *LinuxGPIOTrigger) SetLow(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("linuxgpio: line %d out of range (0–7)", line)
	}
	return t.Send(t.outputState &^ (1 << uint(line)))
}

// Pulse drives line HIGH for dur, then LOW. Blocks for the full duration.
// Implements [OutputTTLDevice].
func (t *LinuxGPIOTrigger) Pulse(line int, dur time.Duration) error {
	return defaultPulse(t, line, dur)
}

// AllLow drives all 8 output lines LOW. Implements [OutputTTLDevice].
func (t *LinuxGPIOTrigger) AllLow() error { return t.Send(0x00) }

// Close drives all output lines LOW and releases the GPIO lines.
// Safe to call multiple times. Implements [OutputTTLDevice] and [InputTTLDevice].
func (t *LinuxGPIOTrigger) Close() error { return t.close() }

// --- InputTTLDevice ---

// ReadAll returns the current state of all 8 input lines as a bitmask.
// Bit N reflects inputPins[N]. Implements [InputTTLDevice].
func (t *LinuxGPIOTrigger) ReadAll() (byte, error) {
	if t.inputPins == nil {
		return 0, fmt.Errorf("linuxgpio: input pins not configured")
	}
	return t.gpioReadInputs()
}

// ReadLine returns the state (0 or 1) of a single input line (0-indexed).
// Implements [InputTTLDevice].
func (t *LinuxGPIOTrigger) ReadLine(line int) (byte, error) {
	return readLineFromMask("linuxgpio", t.ReadAll, line)
}

// WaitForInput blocks until any input line becomes active or ctx is cancelled.
// Returns the active-line bitmask and the elapsed reaction time.
// Implements [InputTTLDevice].
func (t *LinuxGPIOTrigger) WaitForInput(ctx context.Context) (byte, time.Duration, error) {
	return pollWaitForInput(ctx, "linuxgpio", t.ReadAll, t.pollInterval)
}

// DrainInputs polls until all input lines are inactive or ctx is cancelled.
// Call before [LinuxGPIOTrigger.WaitForInput] to clear latched presses.
// Implements [InputTTLDevice].
func (t *LinuxGPIOTrigger) DrainInputs(ctx context.Context) error {
	return pollDrainInputs(ctx, "linuxgpio", t.ReadAll, t.pollInterval)
}
