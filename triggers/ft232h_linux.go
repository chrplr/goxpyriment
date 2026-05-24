// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

//go:build linux

package triggers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// FT232H USB device identifiers.
const (
	ft232hVendorID  = 0x0403
	ft232hProductID = 0x6014

	// Bulk endpoints for FT232H channel A.
	ft232hEPOut = 0x02 // host→device: MPSSE commands
	ft232hEPIn  = 0x81 // device→host: MPSSE responses + modem-status prefix

	// FTDI SIO vendor control-transfer request codes.
	ftdiSioReset      = 0x00
	ftdiSioSetBitmode = 0x0B
	ftdiSioSetLatency = 0x09

	ftdiResetSio     = 0x0000
	ftdiResetPurgeRX = 0x0001
	ftdiResetPurgeTX = 0x0002
	ftdiMpsseBitmode = 0x0002 // MPSSE mode value for wValue low byte

	// bmRequestType: vendor, device, host→device.
	ftdiReqOut = 0x40

	// wIndex for interface 1 (A channel of FT232H).
	ft232hIfaceIndex = 0x0001

	ft232hBulkTimeoutMs = 100 // ms for bulk transfers
	ft232hCtrlTimeoutMs = 500 // ms for control transfers

	// MPSSE GPIO commands from FTDI application note AN_108.
	mpsseSetBitsLow  = 0x80 // SET_BITS_LOW  <value> <dir> — AD0–AD7
	mpsseSetBitsHigh = 0x82 // SET_BITS_HIGH <value> <dir> — AC0–AC7
	mpsseGetBitsHigh = 0x83 // GET_BITS_HIGH              — read AC0–AC7
	mpsseLoopbackOff = 0x85 // LOOPBACK_END — disable internal loopback

	// MPSSE sync: send 0xAA (bad command), device echoes 0xFA 0xAA.
	mpsseSyncByte    = 0xAA
	mpsseEchoFail    = 0xFA
	ft232hModemBytes = 2 // modem-status prefix bytes on every bulk IN packet
)

// usbdevfs ioctl numbers on 64-bit Linux.
//
// Encoding: _IOWR(type, nr, size) = (3<<30)|(size<<16)|(type<<8)|nr
//
// struct usbdevfs_ctrltransfer layout (24 bytes on 64-bit):
//
//	bRequestType uint8   offset  0
//	bRequest     uint8   offset  1
//	wValue       uint16  offset  2
//	wIndex       uint16  offset  4
//	wLength      uint16  offset  6
//	timeout      uint32  offset  8
//	_pad         [4]byte offset 12  ← natural alignment for 8-byte pointer
//	data         uintptr offset 16
//
// struct usbdevfs_bulktransfer layout (24 bytes on 64-bit):
//
//	ep      uint32  offset  0
//	len     uint32  offset  4
//	timeout uint32  offset  8
//	_pad    [4]byte offset 12  ← natural alignment for 8-byte pointer
//	data    uintptr offset 16
const (
	usbdevfsControl        uintptr = 0xC0185500 // _IOWR('U',  0, 24)
	usbdevfsBulk           uintptr = 0xC0185502 // _IOWR('U',  2, 24)
	usbdevfsClaimInterface uintptr = 0x8004550F // _IOR('U', 15,  4)
	usbdevfsDisconnect     uintptr = 0x5516     // _IO('U', 22) — takes *uint32 despite _IO
)

type usbdevfsCtrltransfer struct {
	bRequestType uint8
	bRequest     uint8
	wValue       uint16
	wIndex       uint16
	wLength      uint16
	timeout      uint32
	_            [4]byte // padding to align data pointer to 8 bytes
	data         uintptr
}

type usbdevfsBulktransfer struct {
	ep      uint32
	len     uint32
	timeout uint32
	_       [4]byte // padding to align data pointer to 8 bytes
	data    uintptr
}

// ft232hHandle holds the raw USB file descriptor and a mutex for concurrent access.
type ft232hHandle struct {
	fd int // 0 = not opened; >0 = open; <0 = closed
	mu sync.Mutex
}

// open locates the first FT232H, claims the USB interface, and initialises MPSSE.
func (t *FT232HTrigger) open() error {
	devPath, err := ft232hFindDevice()
	if err != nil {
		return fmt.Errorf("ft232h: %w", err)
	}
	fd, err := syscall.Open(devPath, syscall.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("ft232h: open %s: %w", devPath, err)
	}
	if err := ft232hInitMPSSE(fd); err != nil {
		syscall.Close(fd)
		return fmt.Errorf("ft232h: init MPSSE: %w", err)
	}
	t.handle.fd = fd
	t.outputState = 0
	return nil
}

// close sets all output lines LOW, releases the interface, and closes the fd.
func (t *FT232HTrigger) close() error {
	t.handle.mu.Lock()
	defer t.handle.mu.Unlock()
	if t.handle.fd <= 0 {
		return nil
	}
	_ = ft232hBulkWrite(t.handle.fd, []byte{mpsseSetBitsLow, 0x00, 0xFF})
	err := syscall.Close(t.handle.fd)
	t.handle.fd = -1
	return err
}

// ft232hWriteOutputs sends SET_BITS_LOW to drive AD0–AD7 from mask.
// Direction 0xFF keeps all D-bus pins as outputs.
func (t *FT232HTrigger) ft232hWriteOutputs(mask byte) error {
	t.handle.mu.Lock()
	defer t.handle.mu.Unlock()
	return ft232hBulkWrite(t.handle.fd, []byte{mpsseSetBitsLow, mask, 0xFF})
}

// ft232hReadInputs sends GET_BITS_HIGH and returns the AC0–AC7 input byte.
func (t *FT232HTrigger) ft232hReadInputs() (byte, error) {
	t.handle.mu.Lock()
	defer t.handle.mu.Unlock()
	if err := ft232hBulkWrite(t.handle.fd, []byte{mpsseGetBitsHigh}); err != nil {
		return 0, err
	}
	return ft232hReadOneByte(t.handle.fd)
}

// ft232hFindDevice locates the first FT232H in /sys/bus/usb/devices/ and
// returns its /dev/bus/usb/BBB/DDD path.
func ft232hFindDevice() (string, error) {
	const sysUSB = "/sys/bus/usb/devices"
	entries, err := os.ReadDir(sysUSB)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", sysUSB, err)
	}
	for _, e := range entries {
		dir := filepath.Join(sysUSB, e.Name())
		vid, err1 := ft232hReadHexFile(filepath.Join(dir, "idVendor"))
		pid, err2 := ft232hReadHexFile(filepath.Join(dir, "idProduct"))
		if err1 != nil || err2 != nil {
			continue
		}
		if vid != ft232hVendorID || pid != ft232hProductID {
			continue
		}
		busnum, err3 := ft232hReadIntFile(filepath.Join(dir, "busnum"))
		devnum, err4 := ft232hReadIntFile(filepath.Join(dir, "devnum"))
		if err3 != nil || err4 != nil {
			continue
		}
		return fmt.Sprintf("/dev/bus/usb/%03d/%03d", busnum, devnum), nil
	}
	return "", fmt.Errorf("no FT232H (VID=%04X PID=%04X) found", ft232hVendorID, ft232hProductID)
}

// ft232hInitMPSSE resets the FT232H, enables MPSSE mode, syncs the engine,
// configures AD0–AD7 as outputs (all LOW) and AC0–AC7 as inputs.
func ft232hInitMPSSE(fd int) error {
	// Detach ftdi_sio kernel driver if loaded; ignore error if not present.
	var ifnum uint32
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), usbdevfsDisconnect, uintptr(unsafe.Pointer(&ifnum)))

	// Claim interface 0 for userspace.
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), usbdevfsClaimInterface, uintptr(unsafe.Pointer(&ifnum))); errno != 0 {
		return fmt.Errorf("claim interface: %w (run: sudo rmmod ftdi_sio)", errno)
	}

	// Reset device and purge buffers.
	if err := ft232hControl(fd, ftdiReqOut, ftdiSioReset, ftdiResetSio, ft232hIfaceIndex, nil); err != nil {
		return fmt.Errorf("reset: %w", err)
	}
	if err := ft232hControl(fd, ftdiReqOut, ftdiSioReset, ftdiResetPurgeRX, ft232hIfaceIndex, nil); err != nil {
		return fmt.Errorf("purge RX: %w", err)
	}
	if err := ft232hControl(fd, ftdiReqOut, ftdiSioReset, ftdiResetPurgeTX, ft232hIfaceIndex, nil); err != nil {
		return fmt.Errorf("purge TX: %w", err)
	}

	// Set latency timer to 1 ms for fast MPSSE responses.
	if err := ft232hControl(fd, ftdiReqOut, ftdiSioSetLatency, 1, ft232hIfaceIndex, nil); err != nil {
		return fmt.Errorf("set latency: %w", err)
	}

	// Enable MPSSE mode. wValue = (pin_mask << 8) | mode; mask 0xFF = all pins.
	if err := ft232hControl(fd, ftdiReqOut, ftdiSioSetBitmode, (0xFF<<8)|ftdiMpsseBitmode, ft232hIfaceIndex, nil); err != nil {
		return fmt.Errorf("enable MPSSE: %w", err)
	}

	// Wait for MPSSE to stabilise.
	time.Sleep(5 * time.Millisecond)

	// Flush any leftover IN data.
	flush := make([]byte, 64)
	bt := usbdevfsBulktransfer{ep: ft232hEPIn, len: uint32(len(flush)), timeout: 10, data: uintptr(unsafe.Pointer(&flush[0]))}
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), usbdevfsBulk, uintptr(unsafe.Pointer(&bt)))

	// Sync: send bad command 0xAA; device echoes back 0xFA 0xAA to confirm MPSSE.
	if err := ft232hBulkWrite(fd, []byte{mpsseSyncByte}); err != nil {
		return fmt.Errorf("MPSSE sync write: %w", err)
	}
	resp, err := ft232hBulkReadRaw(fd, 16)
	if err != nil {
		return fmt.Errorf("MPSSE sync read: %w", err)
	}
	if !ft232hContainsSeq(resp, mpsseEchoFail, mpsseSyncByte) {
		return fmt.Errorf("MPSSE sync failed: unexpected response % X", resp)
	}

	// Disable internal loopback.
	if err := ft232hBulkWrite(fd, []byte{mpsseLoopbackOff}); err != nil {
		return fmt.Errorf("loopback off: %w", err)
	}

	// Configure AD0–AD7 all outputs, all LOW.
	if err := ft232hBulkWrite(fd, []byte{mpsseSetBitsLow, 0x00, 0xFF}); err != nil {
		return fmt.Errorf("set D-bus direction: %w", err)
	}
	// Configure AC0–AC7 all inputs.
	if err := ft232hBulkWrite(fd, []byte{mpsseSetBitsHigh, 0x00, 0x00}); err != nil {
		return fmt.Errorf("set C-bus direction: %w", err)
	}
	return nil
}

// ft232hControl sends a USB vendor control transfer to the FT232H.
func ft232hControl(fd int, reqType, req uint8, value, index uint16, data []byte) error {
	ct := usbdevfsCtrltransfer{
		bRequestType: reqType,
		bRequest:     req,
		wValue:       value,
		wIndex:       index,
		wLength:      uint16(len(data)),
		timeout:      ft232hCtrlTimeoutMs,
	}
	if len(data) > 0 {
		ct.data = uintptr(unsafe.Pointer(&data[0]))
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), usbdevfsControl, uintptr(unsafe.Pointer(&ct)))
	if errno != 0 {
		return errno
	}
	return nil
}

// ft232hBulkWrite sends data to the FT232H MPSSE bulk OUT endpoint.
func ft232hBulkWrite(fd int, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	bt := usbdevfsBulktransfer{
		ep:      ft232hEPOut,
		len:     uint32(len(data)),
		timeout: ft232hBulkTimeoutMs,
		data:    uintptr(unsafe.Pointer(&data[0])),
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), usbdevfsBulk, uintptr(unsafe.Pointer(&bt)))
	if errno != 0 {
		return fmt.Errorf("bulk write: %w", errno)
	}
	return nil
}

// ft232hBulkReadRaw reads up to n bytes from the FT232H bulk IN endpoint.
// The FT232H prepends 2 modem-status bytes to every USB IN packet; callers
// are responsible for stripping them.
func ft232hBulkReadRaw(fd int, n int) ([]byte, error) {
	buf := make([]byte, n)
	bt := usbdevfsBulktransfer{
		ep:      ft232hEPIn,
		len:     uint32(n),
		timeout: ft232hBulkTimeoutMs,
		data:    uintptr(unsafe.Pointer(&buf[0])),
	}
	ret, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), usbdevfsBulk, uintptr(unsafe.Pointer(&bt)))
	if errno != 0 {
		return nil, fmt.Errorf("bulk read: %w", errno)
	}
	return buf[:int(ret)], nil
}

// ft232hReadOneByte reads the 1-byte MPSSE GPIO response.
// Every FT232H bulk IN packet starts with 2 modem-status bytes followed by
// the actual payload; the GPIO state byte is at index [ft232hModemBytes].
func ft232hReadOneByte(fd int) (byte, error) {
	data, err := ft232hBulkReadRaw(fd, 64)
	if err != nil {
		return 0, err
	}
	if len(data) < ft232hModemBytes+1 {
		return 0, fmt.Errorf("ft232h: short response (%d bytes)", len(data))
	}
	return data[ft232hModemBytes], nil
}

// ft232hContainsSeq reports whether buf contains the two-byte sequence [a, b].
func ft232hContainsSeq(buf []byte, a, b byte) bool {
	for i := 0; i+1 < len(buf); i++ {
		if buf[i] == a && buf[i+1] == b {
			return true
		}
	}
	return false
}

// ft232hReadHexFile reads a sysfs file containing a 4-digit hex number (e.g. "0403\n").
func ft232hReadHexFile(path string) (uint16, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 16, 16)
	if err != nil {
		return 0, err
	}
	return uint16(v), nil
}

// ft232hReadIntFile reads a sysfs file containing a decimal integer.
func ft232hReadIntFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return v, nil
}
