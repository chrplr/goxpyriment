// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build !js

package apparatus

import "github.com/Zyko0/go-sdl3/sdl"

// present submits the backbuffer to the display. On desktop this is plain
// SDL_RenderPresent, which blocks on VSYNC when the driver honours it.
//
// Do not add SDL_FlushRenderer here. It was tried, on the theory that the
// clear-only-frame bug (see Screen.fillWholeTarget) came from the driver
// deferring command submission until it had real GPU work — flushing should then
// have forced the frame out regardless of what the caller drew. Measured with a
// photodiode against tests/test_clear_only_frames, it made no difference: a frame
// carrying no draw calls was still not scanned out. Whatever elides the frame
// sits below SDL's command batch, so flushing that batch cannot reach it. The
// only remedy found is to give the frame real draw work, which is what
// fillWholeTarget does.
func (s *Screen) present() error {
	return s.Renderer.Present()
}

// paceToFrame busy-waits until the expected frame boundary after a present,
// for drivers where SDL_RenderPresent returns immediately (triple/mailbox
// buffering: NVIDIA + compositor, Wayland mailbox). On well-behaved
// double-buffered VSYNC the wait runs zero iterations.
//
// The spin runs on the SDL clock (sdl.TicksNS) — the same clock PacedFlipTS
// stamps onsets with and that input events carry — so the frame boundary the
// spin waits for and the timestamp the caller records live on one timebase.
// It busy-waits rather than sleeps because sub-millisecond sleep is not
// reliable here.
func (s *Screen) paceToFrame() {
	// Cache the nominal frame duration on first use: it is fixed for the
	// session, and re-querying the SDL display mode every frame is avoidable
	// work on this timing-critical path.
	if s.frameDur == 0 {
		s.frameDur = s.FrameDuration()
	}
	now := sdl.TicksNS()
	if s.lastFlipNS != 0 {
		target := s.lastFlipNS + uint64(s.frameDur.Nanoseconds())
		for now < target {
			now = sdl.TicksNS()
		}
	}
	s.lastFlipNS = now
}
