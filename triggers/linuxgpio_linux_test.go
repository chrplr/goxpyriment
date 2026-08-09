// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build linux

package triggers

import (
	"testing"
	"unsafe"
)

// iowr reproduces the kernel's _IOWR(type, nr, size) encoding:
//
//	_IOC(dir, type, nr, size) = (dir << 30) | (size << 16) | (type << 8) | nr
//
// with dir = _IOC_READ|_IOC_WRITE = 3.
func iowr(typ, nr, size uintptr) uintptr {
	return (3 << 30) | (size << 16) | (typ << 8) | nr
}

// TestGPIOIoctlNumbers pins the three ioctl constants to the request numbers in
// include/uapi/linux/gpio.h, computed rather than copied.
//
// This exists because GET (0x0E) and SET (0x0F) were transposed, which is a
// silent fault rather than a loud one: SetHigh then issued GET_VALUES, the
// kernel happily read the line, no error was returned, and the pin never moved.
// Hardware showed nothing while the API reported success. A hand-written hex
// literal cannot catch that; deriving it from the formula can.
func TestGPIOIoctlNumbers(t *testing.T) {
	const gpioIoctlType = 0xB4

	sizeRequest := unsafe.Sizeof(gpioV2LineRequest{})
	sizeValues := unsafe.Sizeof(gpioV2LineValues{})

	tests := []struct {
		name string
		got  uintptr
		nr   uintptr
		size uintptr
	}{
		{"GPIO_V2_GET_LINE_IOCTL", gpioV2GetLineIoctl, 0x07, sizeRequest},
		{"GPIO_V2_LINE_GET_VALUES_IOCTL", gpioV2LineGetValues, 0x0E, sizeValues},
		{"GPIO_V2_LINE_SET_VALUES_IOCTL", gpioV2LineSetValues, 0x0F, sizeValues},
	}
	for _, tc := range tests {
		want := iowr(gpioIoctlType, tc.nr, tc.size)
		if tc.got != want {
			t.Errorf("%s = %#x, want %#x (_IOWR(0xB4, %#x, %d))",
				tc.name, tc.got, want, tc.nr, tc.size)
		}
	}

	// GET and SET differ only in the request number, so a transposition leaves
	// both constants individually plausible. Assert they are not equal, and
	// that they are the specific pair the header defines, in the right order.
	if gpioV2LineGetValues == gpioV2LineSetValues {
		t.Fatal("GET_VALUES and SET_VALUES must not be the same ioctl")
	}
	if gpioV2LineSetValues != gpioV2LineGetValues+1 {
		t.Errorf("SET_VALUES (%#x) must be GET_VALUES (%#x) + 1; they look transposed",
			gpioV2LineSetValues, gpioV2LineGetValues)
	}
}

// TestGPIOStructSizes guards the wire layout the ioctl numbers encode. The
// sizes are part of the ioctl number itself, so a layout drift silently changes
// which kernel handler is addressed.
func TestGPIOStructSizes(t *testing.T) {
	if s := unsafe.Sizeof(gpioV2LineRequest{}); s != 592 {
		t.Errorf("sizeof(gpio_v2_line_request) = %d, want 592", s)
	}
	if s := unsafe.Sizeof(gpioV2LineValues{}); s != 16 {
		t.Errorf("sizeof(gpio_v2_line_values) = %d, want 16", s)
	}
	if s := unsafe.Sizeof(gpioV2LineConfig{}); s != 272 {
		t.Errorf("sizeof(gpio_v2_line_config) = %d, want 272", s)
	}
	if s := unsafe.Sizeof(gpioV2LineAttribute{}); s != 16 {
		t.Errorf("sizeof(gpio_v2_line_attribute) = %d, want 16", s)
	}
	if s := unsafe.Sizeof(gpioV2LineConfigAttribute{}); s != 24 {
		t.Errorf("sizeof(gpio_v2_line_config_attribute) = %d, want 24", s)
	}
}
