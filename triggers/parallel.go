// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"fmt"
	"time"
)

// ParallelPort controls the 8 data lines of an LPT parallel port via the
// Linux ppdev kernel interface (/dev/parport0, /dev/parport1, …).
//
// On Linux, the real ioctl-based implementation is used (parallel_linux.go).
// On other platforms, [ParallelPort.Open] returns an unsupported error.
//
// The port must be opened before use (Open) and closed afterwards (Close).
// Lines are 0-indexed (line 0 = D0, line 7 = D7). [ParallelPort.Send] sets
// all 8 lines at once using a bitmask, which is the natural way to send EEG
// event codes.
//
// Prerequisites (Linux):
//   - Load the ppdev kernel module: modprobe ppdev
//   - The user must have rw access to /dev/parport0, e.g. by being in the
//     "lp" group: sudo usermod -aG lp $USER (re-login to take effect).
//   - Unload the "lp" kernel module: sudo rmmod lp
//
// Note that the last two both say "lp" and are unrelated: one is the Unix group
// that owns the device node, the other is the parallel *printer* driver. Both
// drivers can be registered on one port, and then [ParallelPort.Open]'s PPCLAIM
// goes through the kernel's parport_claim_or_block: if lp is holding the port,
// the ioctl blocks in uninterruptible sleep, and the process cannot be killed —
// not even with SIGKILL — until lp lets go. It is intermittent, because lp
// holds the port only some of the time.
//
// Look for this line in dmesg; it means lp is attached and the hang is possible:
//
//	lp0: using parport0 (interrupt-driven).
//
// To keep it away across reboots: echo 'blacklist lp' | sudo tee
// /etc/modprobe.d/blacklist-lp.conf. Unloading lp also leaves the port's IRQ
// unarmed, which costs nothing here — ppdev writes do not use the interrupt.
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

// SetHigh drives a single line HIGH. line is 0-indexed (0–7). Implements [OutputTTLDevice].
func (p *ParallelPort) SetHigh(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("parallel: line %d out of range (0–7)", line)
	}
	p.shadow |= 1 << uint(line)
	return p.writeData(p.shadow)
}

// SetLow drives a single line LOW. line is 0-indexed (0–7). Implements [OutputTTLDevice].
func (p *ParallelPort) SetLow(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("parallel: line %d out of range (0–7)", line)
	}
	p.shadow &^= 1 << uint(line)
	return p.writeData(p.shadow)
}

// Send sets all 8 data lines simultaneously from a bitmask.
// Bit N drives line N / D(N). Implements [OutputTTLDevice].
func (p *ParallelPort) Send(mask byte) error {
	p.shadow = mask
	return p.writeData(mask)
}

// Pulse drives line HIGH for dur, then LOW. Implements [OutputTTLDevice].
func (p *ParallelPort) Pulse(line int, dur time.Duration) error {
	return defaultPulse(p, line, dur)
}

// AllLow drives all 8 data lines LOW. Implements [OutputTTLDevice].
func (p *ParallelPort) AllLow() error {
	return p.Send(0x00)
}
