// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

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
	"github.com/chrplr/goxpyriment/results"
)

// SDL_INIT_JOYSTICK/GAMEPAD fail in most browser environments; video,
// events, and audio work.
func platformSDLInitFlags() sdl.InitFlags {
	return sdl.INIT_VIDEO | sdl.INIT_EVENTS | sdl.INIT_AUDIO
}

// platformInitAudio opens the default playback device, like on desktop.
// Browsers keep the underlying AudioContext suspended until the first user
// gesture; SDL's Emscripten backend resumes it automatically on the first
// click/keypress, so sounds triggered after e.g. the "press SPACE to start"
// screen play normally. Unlike desktop, failure is non-fatal: the experiment
// continues with a zero-Device AudioManager whose playback calls are silent
// no-ops, since browser audio support varies and a cognitive task should not
// crash over it.
func (e *Experiment) platformInitAudio() error {
	dev, err := sdl.AUDIO_DEVICE_DEFAULT_PLAYBACK.OpenAudioDevice(nil)
	if err != nil {
		log.Printf("control: browser audio unavailable, continuing silently: %v", err)
		e.Audio = &AudioManager{}
		return nil
	}
	e.AudioDevice = dev
	e.Audio = &AudioManager{Device: dev}
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

// platformAudioDeviceName reports the browser's audio path instead of querying
// SDL. There is no physical device to name — the Emscripten backend routes
// everything through one Web Audio context — and go-sdl3's js binding for
// SDL_GetAudioDeviceName is still a panic-stub, so calling AudioDeviceID.Name
// here aborts the program during Initialize rather than returning an error the
// caller can ignore. That panic killed every browser experiment before its
// first frame.
func platformAudioDeviceName(sdl.AudioDeviceID) (string, error) {
	return "Web Audio (browser)", nil
}

// platformDataDestination names what Finalize just wrote, for the log line at
// the end of a session: in the browser, the single archive the participant is
// handed, holding both files (see results/data_wasm.go).
func platformDataDestination(d *results.DataFile) string {
	return d.ZipFilename()
}
