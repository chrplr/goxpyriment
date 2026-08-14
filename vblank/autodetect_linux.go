// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build linux

package vblank

import (
	"log"
)

func autoDetect() Timer {
	backend, err := newDRMBackend()
	if err != nil {
		log.Printf("present: Linux DRM vblank unavailable (%v); falling back to vsync-estimated", err)
		return NewFallback()
	}
	return backend
}
