// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build darwin

package present

import (
	"log"

	"github.com/chrplr/goxpyriment/apparatus"
)

func autoDetect(screen *apparatus.Screen) Timer {
	backend, err := newCVDisplayLinkBackend(screen)
	if err != nil {
		log.Printf("present: macOS CVDisplayLink unavailable (%v); falling back to vsync-estimated", err)
		return NewFallback()
	}
	return backend
}
