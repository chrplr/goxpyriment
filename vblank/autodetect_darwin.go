// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build darwin

package vblank

import (
	"log"
)

func autoDetect() Timer {
	backend, err := newCVDisplayLinkBackend()
	if err != nil {
		log.Printf("present: macOS CVDisplayLink unavailable (%v); falling back to vsync-estimated", err)
		return NewFallback()
	}
	return backend
}
