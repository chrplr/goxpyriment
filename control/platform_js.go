// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build js

package control

import "github.com/Zyko0/go-sdl3/sdl"

// In the browser, SDL_INIT_AUDIO requires a user gesture before the
// AudioContext can start, and SDL_INIT_JOYSTICK/GAMEPAD fail in most
// browser environments. Use minimal flags here.
func platformSDLInitFlags() sdl.InitFlags {
	return sdl.INIT_VIDEO | sdl.INIT_EVENTS
}

// platformInitAudio is a no-op in WASM. Audio requires a user-gesture
// before the browser allows AudioContext creation; it must be deferred
// to after the first user interaction.
func (e *Experiment) platformInitAudio() error {
	return nil
}
