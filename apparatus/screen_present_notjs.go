// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build !js

package apparatus

import "time"

// present submits the backbuffer to the display. On desktop this is plain
// SDL_RenderPresent, which blocks on VSYNC when the driver honours it.
func (s *Screen) present() error {
	return s.Renderer.Present()
}

// paceToFrame busy-waits until the expected frame boundary after a present,
// for drivers where SDL_RenderPresent returns immediately (triple/mailbox
// buffering: NVIDIA + compositor, Wayland mailbox). On well-behaved
// double-buffered VSYNC the wait runs zero iterations. The spin uses the
// wall clock deliberately: sub-millisecond sleep is not reliable here.
func (s *Screen) paceToFrame() {
	// Cache the nominal frame duration on first use: it is fixed for the
	// session, and re-querying the SDL display mode every frame is avoidable
	// work on this timing-critical path.
	if s.frameDur == 0 {
		s.frameDur = s.FrameDuration()
	}
	now := time.Now()
	if !s.lastFlipTime.IsZero() {
		target := s.lastFlipTime.Add(s.frameDur)
		for now.Before(target) {
			now = time.Now()
		}
	}
	s.lastFlipTime = now
}
