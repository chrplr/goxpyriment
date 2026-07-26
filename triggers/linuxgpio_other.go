// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build !linux

package triggers

import "fmt"

// gpioHandle is empty on non-Linux platforms.
type gpioHandle struct{}

func (t *LinuxGPIOTrigger) open() error {
	return fmt.Errorf("linuxgpio: GPIO character device not supported on this platform (Linux only)")
}

func (t *LinuxGPIOTrigger) close() error { return nil }

func (t *LinuxGPIOTrigger) gpioWriteOutputs(_ byte) error {
	return fmt.Errorf("linuxgpio: not supported on this platform")
}

func (t *LinuxGPIOTrigger) gpioReadInputs() (byte, error) {
	return 0, fmt.Errorf("linuxgpio: not supported on this platform")
}
