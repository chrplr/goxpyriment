// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build darwin

package vblank

import (
	"log"
)

// The Target is unused: CVDisplayLink is created for a display id, and the one
// caller that has a display id does not have a CGDirectDisplayID to give. The
// parameter is kept so the platform signature is one signature.
func autoDetect(_ Target) Timer {
	backend, err := newCVDisplayLinkBackend()
	if err != nil {
		log.Printf("present: macOS CVDisplayLink unavailable (%v); falling back to vsync-estimated", err)
		return NewFallback()
	}
	return backend
}
