// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/assets_embed"
)

// PlayBuzzer plays the embedded buzzer sound synchronously on the given audio device.
func PlayBuzzer(audioDevice sdl.AudioDeviceID) error {
	return PlaySoundFromMemory(audioDevice, assets_embed.BuzzerWav)
}

// PlayPing plays the embedded "correct" ping sound synchronously on the given audio device.
func PlayPing(audioDevice sdl.AudioDeviceID) error {
	return PlaySoundFromMemory(audioDevice, assets_embed.CorrectWav)
}
