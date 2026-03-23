// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package triggers

import "fmt"

// ParallelPort controls the 8 data lines of an LPT parallel port via the
// Linux ppdev kernel interface (/dev/parport0, /dev/parport1, …).
//
// On Linux, the real ioctl-based implementation is used (parallel_linux.go).
// On other platforms, [ParallelPort.Open] returns an unsupported error.
//
// The port must be opened before use (Open) and closed afterwards (Close).
// Individual pins are addressed by 1-indexed number (pin 1 = data bit D0,
// pin 8 = data bit D7). The Send method sets all 8 lines at once using a
// bitmask, which is the natural way to send EEG event codes.
//
// Prerequisites (Linux):
//   - Load the ppdev kernel module: modprobe ppdev
//   - The user must have rw access to /dev/parport0, e.g. by being in the
//     "lp" group: sudo usermod -aG lp $USER (re-login to take effect).
type ParallelPort struct {
	Device string // e.g. "/dev/parport0"
	shadow byte   // shadow register: current value of the 8 data lines
	handle parallelHandle
}

// NewParallelPort creates a ParallelPort for the given device path.
// Call [ParallelPort.Open] before using any I/O methods.
func NewParallelPort(device string) *ParallelPort {
	return &ParallelPort{Device: device}
}

// AvailableParallelPorts returns the device paths of parallel ports that
// appear to be accessible on the current system.
func AvailableParallelPorts() []string {
	return availableParallelPorts()
}

// SetHigh drives a single pin HIGH (1-indexed, 1–8). Implements [Trigger].
func (p *ParallelPort) SetHigh(pin int) error {
	if pin < 1 || pin > 8 {
		return fmt.Errorf("parallel: pin %d out of range (1–8)", pin)
	}
	p.shadow |= 1 << uint(pin-1)
	return p.writeData(p.shadow)
}

// SetLow drives a single pin LOW (1-indexed, 1–8). Implements [Trigger].
func (p *ParallelPort) SetLow(pin int) error {
	if pin < 1 || pin > 8 {
		return fmt.Errorf("parallel: pin %d out of range (1–8)", pin)
	}
	p.shadow &^= 1 << uint(pin-1)
	return p.writeData(p.shadow)
}

// Send sets all 8 data lines simultaneously from a bitmask.
// Bit 0 = pin 1 / D0, bit 7 = pin 8 / D7. Implements [Trigger].
func (p *ParallelPort) Send(value byte) error {
	p.shadow = value
	return p.writeData(value)
}

// Pulse drives pin HIGH for durationMs milliseconds, then LOW. Implements [Trigger].
func (p *ParallelPort) Pulse(pin int, durationMs int) error {
	return defaultPulse(p, pin, durationMs)
}
