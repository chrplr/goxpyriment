// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package io

import (
	"github.com/Zyko0/go-sdl3/sdl"
)

// Keyboard provides blocking and non‑blocking helpers around SDL's keyboard
// events, mirroring the high‑level API of Expyriment.
type Keyboard struct {
	// PollKeys is injected by the control layer to avoid direct SDL polling
	// that discards non-keyboard events. It returns (firstKey, quitRequested).
	PollKeys func() (sdl.Keycode, bool)
}

// Wait blocks until any key is pressed and returns its SDL keycode.
// If the ESC key or a quit event is received, it returns sdl.EndLoop.
func (k *Keyboard) Wait() (sdl.Keycode, error) {
	return k.WaitKeys(nil, -1)
}

// WaitKeys blocks until one of the specified keys is pressed or a timeout
// occurs.
//
//   - If keys is nil, any key will trigger a return.
//   - If timeoutMS is -1, it waits indefinitely.
//   - On timeout, it returns keycode 0 and nil error.
//   - On ESC or quit, it returns sdl.EndLoop.
func (k *Keyboard) WaitKeys(keys []sdl.Keycode, timeoutMS int) (sdl.Keycode, error) {
	start := sdl.Ticks()

	// If a callback is injected (by control.Experiment), use it to avoid
	// discarding mouse events by directly draining the SDL queue.
	if k.PollKeys != nil {
		for {
			if timeoutMS >= 0 {
				elapsed := int(sdl.Ticks() - start)
				if elapsed >= timeoutMS {
					return 0, nil
				}
			}

			keycode, quit := k.PollKeys()
			if quit {
				return 0, sdl.EndLoop
			}

			if keycode != 0 {
				if keycode == sdl.K_ESCAPE {
					return sdl.K_ESCAPE, sdl.EndLoop
				}
				if keys == nil {
					return keycode, nil
				}
				for _, kc := range keys {
					if keycode == kc {
						return keycode, nil
					}
				}
			}

			sdl.Delay(1)
		}
	}

	// Fallback behavior if no callback is injected
	for {
		var event sdl.Event
		var hasEvent bool
		if timeoutMS < 0 {
			if sdl.WaitEvent(&event) == nil {
				hasEvent = true
			}
		} else {
			elapsed := int(sdl.Ticks() - start)
			remaining := timeoutMS - elapsed
			if remaining <= 0 {
				return 0, nil // Timeout
			}
			if sdl.WaitEventTimeout(&event, int32(remaining)) {
				hasEvent = true
			} else {
				// Possibly timeout or error, check again in the loop
				if int(sdl.Ticks()-start) >= timeoutMS {
					return 0, nil
				}
				continue
			}
		}

		if hasEvent {
			if event.Type == sdl.EVENT_KEY_DOWN {
				keycode := event.KeyboardEvent().Key
				if keycode == sdl.K_ESCAPE {
					return 0, sdl.EndLoop
				}
				if keys == nil {
					return keycode, nil
				}
				for _, k := range keys {
					if keycode == k {
						return keycode, nil
					}
				}
			}
			if event.Type == sdl.EVENT_QUIT {
				return 0, sdl.EndLoop
			}
		}
	}
}

// Check polls for keyboard events without blocking and returns the first key
// pressed since the last call (or 0 if none). ESC or a quit event yields
// sdl.EndLoop.
func (k *Keyboard) Check() (sdl.Keycode, error) {
	var event sdl.Event
	for sdl.PollEvent(&event) {
		if event.Type == sdl.EVENT_KEY_DOWN {
			keycode := event.KeyboardEvent().Key
			if keycode == sdl.K_ESCAPE {
				return 0, sdl.EndLoop
			}
			return keycode, nil
		}
		if event.Type == sdl.EVENT_QUIT {
			return 0, sdl.EndLoop
		}
	}
	return 0, nil
}

// WaitKey blocks until the given key is pressed and returns an error only on
// ESC / window close. It is a convenience wrapper around WaitKeys for the
// common "wait for SPACE to continue" pattern.
func (k *Keyboard) WaitKey(key sdl.Keycode) error {
	_, err := k.WaitKeys([]sdl.Keycode{key}, -1)
	return err
}

// WaitKeysRT blocks until one of the specified keys is pressed (or a timeout
// occurs) and also returns the reaction time in milliseconds measured from
// the moment WaitKeysRT was called.
//
// This bundles the common three-line pattern:
//
//	startTime := clock.GetTime()
//	key, err := kb.WaitKeys(keys, timeout)
//	rt := clock.GetTime() - startTime
func (k *Keyboard) WaitKeysRT(keys []sdl.Keycode, timeoutMS int) (sdl.Keycode, int64, error) {
	start := sdl.Ticks()
	key, err := k.WaitKeys(keys, timeoutMS)
	rt := int64(sdl.Ticks() - start)
	return key, rt, err
}

// Clear drains all pending keyboard (and other) events from SDL's event queue.
// This is useful between trials to avoid processing stale key presses.
func (k *Keyboard) Clear() {
	var event sdl.Event
	for sdl.PollEvent(&event) {
		// Just drain the queue
	}
}
