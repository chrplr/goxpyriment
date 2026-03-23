// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

//go:build !linux

package triggers

import "fmt"

type parallelHandle struct{} // no hardware handle on non-Linux platforms

// Open returns an error on non-Linux platforms.
func (p *ParallelPort) Open() error {
	return fmt.Errorf("parallel port: not supported on this platform (Linux only)")
}

// Close is a no-op on non-Linux platforms.
func (p *ParallelPort) Close() error { return nil }

func (p *ParallelPort) writeData(_ byte) error {
	return fmt.Errorf("parallel port: not supported on this platform (Linux only)")
}

func availableParallelPorts() []string { return nil }
