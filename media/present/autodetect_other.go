// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build !darwin && !linux

package present

import "github.com/chrplr/goxpyriment/apparatus"

func autoDetect(_ *apparatus.Screen) Timer {
	// No platform-specific backend implemented for this OS yet.
	// Stage 5 of media/Plan.md targets macOS and Linux first; Windows
	// (DXGI GetFrameStatistics) is a future addition.
	return NewFallback()
}
