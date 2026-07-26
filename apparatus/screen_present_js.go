// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build js

package apparatus

import "github.com/Zyko0/go-sdl3/sdl"

// present submits the backbuffer to the canvas and parks until the browser's
// next requestAnimationFrame tick — the browser's VSYNC equivalent.
//
// Two reasons this must block, unlike a bare SDL_RenderPresent (which returns
// immediately under Emscripten, vsync 0):
//
//  1. Correctness: canvas updates are only composited when the page yields to
//     the browser event loop. A draw/present loop that never parks would show
//     nothing and freeze the tab.
//  2. Pacing: RAF fires once per display refresh, right before the compositor
//     paints, so each present occupies exactly one display frame — the same
//     contract Update/PacedFlip have on a well-behaved desktop VSYNC driver.
func (s *Screen) present() error {
	if err := s.Renderer.Present(); err != nil {
		return err
	}
	sdl.WaitAnimationFrame()
	return nil
}

// paceToFrame is a no-op in the browser: present already parks until the
// next animation frame, and a busy-wait would never yield to the event loop
// (wedging the tab). The desktop implementation lives in
// screen_present_notjs.go.
func (s *Screen) paceToFrame() {}
