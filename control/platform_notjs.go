// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build !js

package control

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/results"
)

func platformSDLInitFlags() sdl.InitFlags {
	return sdl.INIT_VIDEO | sdl.INIT_EVENTS | sdl.INIT_AUDIO | sdl.INIT_JOYSTICK | sdl.INIT_GAMEPAD
}

func (e *Experiment) platformInitAudio() error {
	dev, err := sdl.AUDIO_DEVICE_DEFAULT_PLAYBACK.OpenAudioDevice(nil)
	if err != nil {
		return fmt.Errorf("control: opening default audio device: %w", err)
	}
	e.AudioDevice = dev
	e.Audio = &AudioManager{Device: dev}
	return nil
}

// platformPrepareFlags is a no-op on desktop: flags come from the real
// command line.
func platformPrepareFlags() {}

// platformInteractiveSetup reports whether the session-setup dialog may be
// opened when -s is absent. Always true on desktop.
func platformInteractiveSetup() bool { return true }

// platformAudioDeviceName resolves a logical audio device to the physical one
// behind it, for the session metadata.
func platformAudioDeviceName(dev sdl.AudioDeviceID) (string, error) {
	return dev.Name()
}

// platformDataDestination names what Finalize just wrote, for the log line at
// the end of a session: on desktop, the two files it flushed to disk.
func platformDataDestination(d *results.DataFile) string {
	return fmt.Sprintf("%s (info: %s)", d.FullPath, d.InfoFile.FullPath)
}
