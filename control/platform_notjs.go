// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build !js

package control

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
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
