// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build js

package control

import (
	"flag"
	"log"
	"net/url"
	"os"
	"strings"
	"syscall/js"

	"github.com/Zyko0/go-sdl3/sdl"
)

// In the browser, SDL_INIT_AUDIO requires a user gesture before the
// AudioContext can start, and SDL_INIT_JOYSTICK/GAMEPAD fail in most
// browser environments. Use minimal flags here.
func platformSDLInitFlags() sdl.InitFlags {
	return sdl.INIT_VIDEO | sdl.INIT_EVENTS
}

// platformInitAudio does not open an audio device in WASM: the browser
// requires a user gesture before an AudioContext may start, so opening is
// deferred (Phase 4 of the port). It still installs an AudioManager with a
// zero Device so playback calls (PlayBuzzer, PlaySync, …) degrade to silent
// no-ops instead of dereferencing a nil manager.
func (e *Experiment) platformInitAudio() error {
	e.Audio = &AudioManager{}
	return nil
}

// platformPrepareFlags synthesizes command-line arguments from the page URL's
// query string, so the standard flags — and any experiment-specific flags the
// program registered — can be set in the browser:
//
//	experiment.html?s=3&w      →  -s=3 -w
//
// A key without a value becomes a bare boolean flag. Keys that don't match a
// registered flag (e.g. tracking parameters) are skipped with a console note
// instead of aborting the program the way flag.Parse would. Call after all
// flags are registered and immediately before flag.Parse.
func platformPrepareFlags() {
	loc := js.Global().Get("location")
	if loc.IsUndefined() {
		return
	}
	raw := strings.TrimPrefix(loc.Get("search").String(), "?")
	if raw == "" {
		return
	}
	args := os.Args[:1:1]
	for _, pair := range strings.Split(raw, "&") {
		if pair == "" {
			continue
		}
		key, val, hasVal := strings.Cut(pair, "=")
		k, err := url.QueryUnescape(key)
		if err != nil || k == "" {
			continue
		}
		if flag.Lookup(k) == nil {
			log.Printf("control: ignoring unknown URL parameter %q", k)
			continue
		}
		if !hasVal || val == "" {
			args = append(args, "-"+k)
			continue
		}
		v, err := url.QueryUnescape(val)
		if err != nil {
			log.Printf("control: ignoring malformed URL parameter %q", pair)
			continue
		}
		args = append(args, "-"+k+"="+v)
	}
	os.Args = args
}

// platformInteractiveSetup reports whether the session-setup dialog may be
// opened when -s is absent. Always false in the browser: the dialog opens its
// own SDL window and shuts SDL down afterwards, neither of which works in a
// single-canvas page. Session settings come from URL parameters instead.
func platformInteractiveSetup() bool { return false }
