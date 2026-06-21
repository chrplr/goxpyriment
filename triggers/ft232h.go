// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package triggers

import (
	"context"
	"fmt"
	"log"
	"time"
)

// FT232HTrigger controls an Adafruit FT232H USB-to-MPSSE bridge.
//
// D-bus (AD0–AD7) pins are wired to 8 TTL output lines for sending trigger
// codes to recording equipment. C-bus (AC0–AC7) pins are wired to 8 TTL input
// lines for reading response-pad signals.
//
// Implements both [OutputTTLDevice] and [InputTTLDevice].
//
// The device is accessed directly via the Linux usbfs kernel interface
// (/dev/bus/usb). No system libftdi or D2XX driver installation is required.
//
// Prerequisites (Linux):
//   - The ftdi_sio kernel module must not hold the interface. If it does,
//     unload it: sudo rmmod ftdi_sio
//   - The user needs rw permission to /dev/bus/usb/BBB/DDD, typically by
//     adding a udev rule or joining the plugdev group.
//
// Usage:
//
//	box, err := triggers.NewFT232H()
//	if err != nil { log.Fatal(err) }
//	defer box.Close()
//
//	box.Pulse(0, 5*time.Millisecond)
//	mask, rt, _ := box.WaitForInput(ctx)
type FT232HTrigger struct {
	handle       ft232hHandle
	outputState  byte
	pollInterval time.Duration
}

// FT232HOption configures an [FT232HTrigger] at construction time.
type FT232HOption func(*FT232HTrigger)

// WithFT232HPollInterval sets the polling interval used by [FT232HTrigger.WaitForInput]
// and [FT232HTrigger.DrainInputs]. Default: 5 ms.
func WithFT232HPollInterval(d time.Duration) FT232HOption {
	return func(t *FT232HTrigger) { t.pollInterval = d }
}

const ft232hDefaultPollInterval = 5 * time.Millisecond

// NewFT232H opens the first available Adafruit FT232H device, configures
// AD0–AD7 as TTL outputs (all LOW) and AC0–AC7 as TTL inputs.
func NewFT232H(opts ...FT232HOption) (*FT232HTrigger, error) {
	t := &FT232HTrigger{pollInterval: ft232hDefaultPollInterval}
	for _, opt := range opts {
		opt(t)
	}
	if err := t.open(); err != nil {
		return nil, fmt.Errorf("triggers.NewFT232H: %w", err)
	}
	return t, nil
}

// AutoDetectFT232H opens the first available FT232H device. If no device is
// found it returns a [NullOutputTTLDevice] and logs a warning; callers do not
// need to nil-check the returned [OutputTTLDevice].
func AutoDetectFT232H(opts ...FT232HOption) (OutputTTLDevice, error) {
	t, err := NewFT232H(opts...)
	if err != nil {
		log.Printf("ft232h: %v — trigger output disabled", err)
		return NullOutputTTLDevice{}, nil
	}
	return t, nil
}

// --- OutputTTLDevice ---

// Send sets all 8 AD0–AD7 output lines simultaneously from a bitmask.
// Bit N drives line N HIGH; zero drives it LOW. Implements [OutputTTLDevice].
func (t *FT232HTrigger) Send(mask byte) error {
	if err := t.ft232hWriteOutputs(mask); err != nil {
		return fmt.Errorf("ft232h.Send: %w", err)
	}
	t.outputState = mask
	return nil
}

// SetHigh drives a single AD0–AD7 output line HIGH. line is 0-indexed (0–7).
// Implements [OutputTTLDevice].
func (t *FT232HTrigger) SetHigh(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("ft232h: line %d out of range (0–7)", line)
	}
	return t.Send(t.outputState | (1 << uint(line)))
}

// SetLow drives a single AD0–AD7 output line LOW. line is 0-indexed (0–7).
// Implements [OutputTTLDevice].
func (t *FT232HTrigger) SetLow(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("ft232h: line %d out of range (0–7)", line)
	}
	return t.Send(t.outputState &^ (1 << uint(line)))
}

// Pulse drives line HIGH for dur, then LOW. Blocks for the full duration.
// Implements [OutputTTLDevice].
func (t *FT232HTrigger) Pulse(line int, dur time.Duration) error {
	return defaultPulse(t, line, dur)
}

// AllLow drives all 8 AD0–AD7 output lines LOW. Implements [OutputTTLDevice].
func (t *FT232HTrigger) AllLow() error { return t.Send(0x00) }

// Close sets all output lines LOW and releases the USB device.
// Safe to call multiple times.
func (t *FT232HTrigger) Close() error { return t.close() }

// --- InputTTLDevice ---

// ReadAll returns the current state of all 8 AC0–AC7 input lines as a bitmask.
// Bit N reflects line N. Implements [InputTTLDevice].
func (t *FT232HTrigger) ReadAll() (byte, error) {
	return t.ft232hReadInputs()
}

// ReadLine returns the state (0 or 1) of a single AC0–AC7 input line (0-indexed).
// Implements [InputTTLDevice].
func (t *FT232HTrigger) ReadLine(line int) (byte, error) {
	return readLineFromMask("ft232h", t.ReadAll, line)
}

// WaitForInput blocks until any AC0–AC7 input line becomes active or ctx is
// cancelled. Returns the active-line bitmask and the elapsed reaction time.
// Implements [InputTTLDevice].
func (t *FT232HTrigger) WaitForInput(ctx context.Context) (byte, time.Duration, error) {
	return pollWaitForInput(ctx, "ft232h", t.ReadAll, t.pollInterval)
}

// DrainInputs polls until all AC0–AC7 input lines are inactive or ctx is
// cancelled. Call before [FT232HTrigger.WaitForInput] to clear latched presses
// from a previous trial. Implements [InputTTLDevice].
func (t *FT232HTrigger) DrainInputs(ctx context.Context) error {
	return pollDrainInputs(ctx, "ft232h", t.ReadAll, t.pollInterval)
}
