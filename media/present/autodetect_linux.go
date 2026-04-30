// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build linux

package present

import (
	"log"

	"github.com/chrplr/goxpyriment/apparatus"
)

func autoDetect(screen *apparatus.Screen) Timer {
	backend, err := newDRMBackend(screen)
	if err != nil {
		log.Printf("present: Linux DRM vblank unavailable (%v); falling back to vsync-estimated", err)
		return NewFallback()
	}
	return backend
}
