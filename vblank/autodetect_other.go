// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build !darwin && !linux

package vblank

func autoDetect() Timer {
	// No platform-specific backend implemented for this OS yet.
	// Stage 5 of media/Plan.md targets macOS and Linux first; Windows
	// (DXGI GetFrameStatistics) is a future addition.
	return NewFallback()
}
