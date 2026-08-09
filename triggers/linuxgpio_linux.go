// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build linux

package triggers

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// Linux GPIO character device v2 API (kernel ≥ 5.10).
// Reference: include/uapi/linux/gpio.h
//
// Ioctl numbers use the standard Linux encoding:
//
//	_IOWR(type, nr, size) = (3<<30) | (size<<16) | (type<<8) | nr
//
// Struct sizes on all Linux architectures (no pointers, only fixed-width types):
//
//	sizeof(gpio_v2_line_values)  = 16
//	sizeof(gpio_v2_line_request) = 592
//
// So:
//
//	GPIO_V2_GET_LINE_IOCTL        = _IOWR(0xB4, 0x07, 592) = 0xC250B407
//	GPIO_V2_LINE_GET_VALUES_IOCTL = _IOWR(0xB4, 0x0E,  16) = 0xC010B40E
//	GPIO_V2_LINE_SET_VALUES_IOCTL = _IOWR(0xB4, 0x0F,  16) = 0xC010B40F
//
// GET is 0x0E and SET is 0x0F, per include/uapi/linux/gpio.h. These two were
// swapped here until 2026-08-09, so every SetHigh/SetLow issued GET_VALUES
// instead: a read of the line, which succeeds, returns no error, and leaves the
// pin untouched. GPIO output therefore never worked, and failed silently — the
// caller saw a nil error and no signal on the wire. Verified against the header
// on a Raspberry Pi 4 (pinctrl-bcm2711) after a BBTK saw nothing on BCM 17 that
// `pinctrl 17 op dh` drove correctly.
const (
	gpioV2GetLineIoctl  uintptr = 0xC250B407
	gpioV2LineGetValues uintptr = 0xC010B40E
	gpioV2LineSetValues uintptr = 0xC010B40F

	// gpio_v2_line_flag bits (from enum gpio_v2_line_flag).
	gpioV2LineFlagInput  = uint64(1 << 2) // _BITULL(2)
	gpioV2LineFlagOutput = uint64(1 << 3) // _BITULL(3)

	// gpio_v2_line_attr_id values.
	gpioV2AttrIDOutputValues = uint32(2)

	// Struct dimension constants.
	gpioV2LinesMax         = 64
	gpioMaxNameSize        = 32
	gpioV2LineMaxLineAttrs = 10

	gpioConsumerOut = "goxpyriment-out"
	gpioConsumerIn  = "goxpyriment-in"
)

// gpioV2LineValues mirrors struct gpio_v2_line_values (16 bytes).
//
//	bits  __aligned_u64  offset 0
//	mask  __aligned_u64  offset 8
type gpioV2LineValues struct {
	bits uint64 // bit N = state of Nth line in handle
	mask uint64 // which lines to get/set (0xFF for all 8)
}

// gpioV2LineAttribute mirrors struct gpio_v2_line_attribute (16 bytes).
//
//	id       __u32         offset  0
//	padding  __u32         offset  4
//	<union>  __aligned_u64 offset  8  (flags / values / debounce_period_us)
type gpioV2LineAttribute struct {
	id      uint32
	padding uint32
	data    uint64 // flags, output values, or debounce period depending on id
}

// gpioV2LineConfigAttribute mirrors struct gpio_v2_line_config_attribute (24 bytes).
//
//	attr  gpio_v2_line_attribute  offset  0 (16 bytes)
//	mask  __aligned_u64           offset 16
type gpioV2LineConfigAttribute struct {
	attr gpioV2LineAttribute
	mask uint64
}

// gpioV2LineConfig mirrors struct gpio_v2_line_config (272 bytes).
//
//	flags     __aligned_u64                          offset   0
//	num_attrs __u32                                  offset   8
//	padding   __u32[5]                               offset  12
//	attrs     gpio_v2_line_config_attribute[10]      offset  32
type gpioV2LineConfig struct {
	flags    uint64
	numAttrs uint32
	padding  [5]uint32
	attrs    [gpioV2LineMaxLineAttrs]gpioV2LineConfigAttribute
}

// gpioV2LineRequest mirrors struct gpio_v2_line_request (592 bytes).
//
//	offsets          __u32[64]            offset   0  (256 bytes)
//	consumer         char[32]             offset 256
//	config           gpio_v2_line_config  offset 288  (272 bytes)
//	num_lines        __u32                offset 560
//	event_buffer_size __u32               offset 564
//	padding          __u32[5]             offset 568
//	fd               __s32                offset 588  ← kernel fills this in
type gpioV2LineRequest struct {
	offsets         [gpioV2LinesMax]uint32
	consumer        [gpioMaxNameSize]byte
	config          gpioV2LineConfig
	numLines        uint32
	eventBufferSize uint32
	padding         [5]uint32
	fd              int32
}

func init() {
	// Compile-time struct size guard: panic early rather than silently
	// corrupt kernel memory if the layout drifts on an unusual platform.
	if s := unsafe.Sizeof(gpioV2LineRequest{}); s != 592 {
		panic(fmt.Sprintf("linuxgpio: gpioV2LineRequest size %d ≠ 592 — check build target", s))
	}
}

// gpioHandle holds the OS-level line handle file descriptors.
type gpioHandle struct {
	outFd int // line handle for outputs; ≤0 = not configured
	inFd  int // line handle for inputs;  ≤0 = not configured
	mu    sync.Mutex
}

// open claims the configured output and/or input GPIO lines.
func (t *LinuxGPIOTrigger) open() error {
	chipFd, err := syscall.Open(t.chipPath, syscall.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("linuxgpio: open %s: %w", t.chipPath, err)
	}
	defer syscall.Close(chipFd)

	t.handle.outFd = -1
	t.handle.inFd = -1

	if t.outputPins != nil {
		fd, err := gpioRequestLines(chipFd, t.outputPins[:], true, gpioConsumerOut)
		if err != nil {
			return fmt.Errorf("linuxgpio: request output lines: %w", err)
		}
		t.handle.outFd = fd
	}

	if t.inputPins != nil {
		fd, err := gpioRequestLines(chipFd, t.inputPins[:], false, gpioConsumerIn)
		if err != nil {
			if t.handle.outFd > 0 {
				syscall.Close(t.handle.outFd)
				t.handle.outFd = -1
			}
			return fmt.Errorf("linuxgpio: request input lines: %w", err)
		}
		t.handle.inFd = fd
	}

	t.outputState = 0
	return nil
}

// close drives all output lines LOW and releases both line handles.
func (t *LinuxGPIOTrigger) close() error {
	t.handle.mu.Lock()
	defer t.handle.mu.Unlock()
	if t.handle.outFd > 0 {
		_ = gpioSetByte(t.handle.outFd, 0x00)
		syscall.Close(t.handle.outFd)
		t.handle.outFd = -1
	}
	if t.handle.inFd > 0 {
		syscall.Close(t.handle.inFd)
		t.handle.inFd = -1
	}
	return nil
}

// gpioWriteOutputs writes mask to the 8 output lines atomically.
//
// The descriptor is checked rather than handed to ioctl unconditionally: after
// close() it is -1, and the kernel then reports a bare EBADF ("bad file
// descriptor") that says nothing about which handle died or why. A trigger
// fired from a goroutine can outlive the Close that ends a run, so this is
// reachable in normal use.
func (t *LinuxGPIOTrigger) gpioWriteOutputs(mask byte) error {
	t.handle.mu.Lock()
	defer t.handle.mu.Unlock()
	if t.handle.outFd <= 0 {
		return fmt.Errorf("linuxgpio: output handle is closed or was never opened")
	}
	return gpioSetByte(t.handle.outFd, mask)
}

// gpioReadInputs reads the 8 input lines as a bitmask.
func (t *LinuxGPIOTrigger) gpioReadInputs() (byte, error) {
	t.handle.mu.Lock()
	defer t.handle.mu.Unlock()
	if t.handle.inFd <= 0 {
		return 0, fmt.Errorf("linuxgpio: input handle is closed or was never opened")
	}
	return gpioGetByte(t.handle.inFd)
}

// gpioRequestLines opens a GPIO_V2 line handle for the given set of pins.
// output=true configures them as outputs (all LOW initially); false = inputs.
func gpioRequestLines(chipFd int, pins []int, output bool, consumer string) (int, error) {
	req := gpioV2LineRequest{
		numLines: uint32(len(pins)),
	}
	copy(req.consumer[:gpioMaxNameSize-1], consumer)
	for i, p := range pins {
		req.offsets[i] = uint32(p)
	}

	if output {
		req.config.flags = gpioV2LineFlagOutput
		// Set an output-values attribute so all lines start LOW.
		req.config.numAttrs = 1
		req.config.attrs[0] = gpioV2LineConfigAttribute{
			attr: gpioV2LineAttribute{
				id:   gpioV2AttrIDOutputValues,
				data: 0x00, // all LOW
			},
			mask: 0xFF, // applies to all 8 requested lines
		}
	} else {
		req.config.flags = gpioV2LineFlagInput
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(chipFd), gpioV2GetLineIoctl, uintptr(unsafe.Pointer(&req)))
	if errno != 0 {
		return 0, fmt.Errorf("GPIO_V2_GET_LINE: %w", errno)
	}
	// A zero or negative fd means the kernel accepted the ioctl without handing
	// back a usable handle. Catching it here names the problem; passing it on
	// turns every later write into an unexplained EBADF instead.
	if req.fd <= 0 {
		return 0, fmt.Errorf("GPIO_V2_GET_LINE returned no usable descriptor (fd=%d)", req.fd)
	}
	return int(req.fd), nil
}

// gpioSetByte writes mask to all 8 lines held by lineFd atomically.
func gpioSetByte(lineFd int, mask byte) error {
	vals := gpioV2LineValues{bits: uint64(mask), mask: 0xFF}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(lineFd), gpioV2LineSetValues, uintptr(unsafe.Pointer(&vals)))
	if errno != 0 {
		return fmt.Errorf("GPIO_V2_LINE_SET_VALUES: %w", errno)
	}
	return nil
}

// gpioGetByte reads the current state of the 8 lines held by lineFd as a bitmask.
func gpioGetByte(lineFd int) (byte, error) {
	vals := gpioV2LineValues{mask: 0xFF}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(lineFd), gpioV2LineGetValues, uintptr(unsafe.Pointer(&vals)))
	if errno != 0 {
		return 0, fmt.Errorf("GPIO_V2_LINE_GET_VALUES: %w", errno)
	}
	return byte(vals.bits & 0xFF), nil
}
