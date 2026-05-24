// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

//go:build !linux

package triggers

import "fmt"

// ft232hHandle is empty on non-Linux platforms.
type ft232hHandle struct{}

func (t *FT232HTrigger) open() error {
	return fmt.Errorf("ft232h: direct USB access not supported on this platform (Linux only)")
}

func (t *FT232HTrigger) close() error { return nil }

func (t *FT232HTrigger) ft232hWriteOutputs(_ byte) error {
	return fmt.Errorf("ft232h: not supported on this platform")
}

func (t *FT232HTrigger) ft232hReadInputs() (byte, error) {
	return 0, fmt.Errorf("ft232h: not supported on this platform")
}
