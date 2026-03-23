// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

//go:build linux

package triggers

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// Linux ppdev ioctl constants derived from <linux/ppdev.h>.
//
// Encoding: _IO(type,nr)  = (type<<8)|nr
//
//	_IOW(type,nr,T) = (1<<30)|(sizeof(T)<<16)|(type<<8)|nr
//	_IOR(type,nr,T) = (2<<30)|(sizeof(T)<<16)|(type<<8)|nr
//
// type = 'p' = 0x70, sizeof(uint8) = 1.
const (
	ppClaim   uintptr = 0x0000700B // _IO('p', 11)  — claim exclusive access
	ppRelease uintptr = 0x0000700C // _IO('p', 12)  — release
	ppwData   uintptr = 0x40017004 // _IOW('p', 4, uint8) — write data register
	pprStatus uintptr = 0x80017003 // _IOR('p', 3, uint8) — read status register
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
		return fmt.Errorf("parallel: claim %s: %w", p.Device, errno)
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
