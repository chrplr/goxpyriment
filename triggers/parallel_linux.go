// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build linux

package triggers

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// Linux ppdev ioctl numbers, from <linux/ppdev.h>.
//
// Encoding, from <asm-generic/ioctl.h>:
//
//	_IO (type,nr)   =            (type<<8) | nr
//	_IOW(type,nr,T) = (1<<30) | (sizeof(T)<<16) | (type<<8) | nr
//	_IOR(type,nr,T) = (2<<30) | (sizeof(T)<<16) | (type<<8) | nr
//
// The request numbers are written out below rather than as hex literals,
// because the literals were wrong for as long as this file existed and nothing
// caught it. They used nr = 11, 12, 4, 3 — plausible small numbers — where the
// header says 0x8b, 0x8c, 0x86, 0x81. PPCLAIM was therefore issued as 0x700b
// instead of 0x708b, an ioctl the kernel does not implement, and every attempt
// to open a port failed with:
//
//	parallel: claim /dev/parport0: invalid argument
//
// So TRIGGER_DEVICE=parallel never worked on Linux, on any machine, while the
// documentation recommended it over the USB devices for having no link in the
// path. Diagnosed 2026-08-17 on the first machine that actually had an LPT port
// to try it on. If this ever regresses, the symptom is EINVAL from Open and the
// check is one grep of /usr/include/linux/ppdev.h.
const (
	ppIOCType = 'p'

	ppClaim   uintptr = (ppIOCType << 8) | 0x8b                         // _IO ('p', 0x8b)
	ppRelease uintptr = (ppIOCType << 8) | 0x8c                         // _IO ('p', 0x8c)
	ppwData   uintptr = (1 << 30) | (1 << 16) | (ppIOCType << 8) | 0x86 // _IOW('p', 0x86, uint8)
	pprStatus uintptr = (2 << 30) | (1 << 16) | (ppIOCType << 8) | 0x81 // _IOR('p', 0x81, uint8)
	ppDataDir uintptr = (1 << 30) | (4 << 16) | (ppIOCType << 8) | 0x90 // _IOW('p', 0x90, int)
)

type parallelHandle struct {
	f *os.File
}

// Open claims exclusive access to the parallel port device.
func (p *ParallelPort) Open() error {
	f, err := os.OpenFile(p.Device, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("parallel: open %s: %w", p.Device, err)
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), ppClaim, 0); errno != 0 {
		f.Close()
		if errno == syscall.EBUSY {
			// Name the usual culprit. PPCLAIM contends with every other driver
			// registered on the port, and on a stock desktop that is the lp
			// printer driver -- which dmesg announces as "lp0: using parport0".
			// Refusing outright is the good case: when lp holds the port rather
			// than merely being attached, the claim blocks in uninterruptible
			// sleep instead, and the process cannot be killed at all.
			return fmt.Errorf("parallel: claim %s: %w "+
				"(another driver holds the port; if dmesg says \"lp0: using %s\", "+
				"unload the lp printer module: sudo rmmod lp)",
				p.Device, errno, filepath.Base(p.Device))
		}
		return fmt.Errorf("parallel: claim %s: %w", p.Device, errno)
	}
	// Force the data lines to OUTPUT. A port left in reverse direction by
	// whatever used it last reads the pins instead of driving them, so every
	// write would appear to succeed and no pin would move — the same silent
	// failure as an unplugged trigger. Not fatal if the driver refuses it:
	// compatibility mode is forward on every chipset seen, so this is a
	// belt-and-braces call, not a requirement.
	dir := int32(0) // 0 = forward (drive), 1 = reverse (read)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), ppDataDir,
		uintptr(unsafe.Pointer(&dir))); errno != 0 {
		log.Printf("parallel: could not set %s to output direction (%v); "+
			"continuing, but check the pins actually move", p.Device, errno)
	}
	p.handle.f = f
	p.shadow = 0
	return p.writeData(0) // ensure all lines start LOW
}

// Close sets all lines LOW, releases the port, and closes the device file.
func (p *ParallelPort) Close() error {
	if p.handle.f == nil {
		return nil
	}
	_ = p.writeData(0)
	syscall.Syscall(syscall.SYS_IOCTL, p.handle.f.Fd(), ppRelease, 0)
	err := p.handle.f.Close()
	p.handle.f = nil
	return err
}

// writeData sends a byte to the parallel port data register via PPWDATA ioctl.
func (p *ParallelPort) writeData(value byte) error {
	if p.handle.f == nil {
		return fmt.Errorf("parallel: port not open")
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		p.handle.f.Fd(),
		ppwData,
		uintptr(unsafe.Pointer(&value)),
	)
	if errno != 0 {
		return fmt.Errorf("parallel: PPWDATA: %w", errno)
	}
	return nil
}

// ReadStatus reads the 5 parallel port status lines (nACK, BUSY, PAPER-OUT,
// SELECT, nERROR) and returns the raw status register byte.
func (p *ParallelPort) ReadStatus() (byte, error) {
	if p.handle.f == nil {
		return 0, fmt.Errorf("parallel: port not open")
	}
	var v byte
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		p.handle.f.Fd(),
		pprStatus,
		uintptr(unsafe.Pointer(&v)),
	)
	if errno != 0 {
		return 0, fmt.Errorf("parallel: PPRSTATUS: %w", errno)
	}
	return v, nil
}

func availableParallelPorts() []string {
	var ports []string
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("/dev/parport%d", i)
		if f, err := os.OpenFile(name, os.O_RDWR, 0); err == nil {
			f.Close()
			ports = append(ports, name)
		}
	}
	return ports
}
