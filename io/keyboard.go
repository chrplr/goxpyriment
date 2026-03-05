// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package io

import (
	"github.com/Zyko0/go-sdl3/sdl"
)

// Keyboard provides methods for handling keyboard input.
type Keyboard struct {
}

// Wait blocks until a key is pressed and returns the key.
func (k *Keyboard) Wait() (sdl.Keycode, error) {
	return k.WaitKeys(nil, -1)
}

// WaitKeys blocks until one of the specified keys is pressed or a timeout occurs.
// If keys is nil, any key will trigger a return.
// If timeoutMS is -1, it waits indefinitely.
// Returns the keycode and any error (e.g., sdl.EndLoop for quit).
func (k *Keyboard) WaitKeys(keys []sdl.Keycode, timeoutMS int) (sdl.Keycode, error) {
	start := sdl.Ticks()
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

// Check polls for keyboard events and returns the first key pressed since the last call, without blocking.
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

// Clear clears all keyboard events from the queue.
func (k *Keyboard) Clear() {
	var event sdl.Event
	for sdl.PollEvent(&event) {
		// Just drain the queue
	}
}
