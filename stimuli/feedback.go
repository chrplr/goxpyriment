// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/Zyko0/go-sdl3/sdl"
)

// PlayBuzzer plays the embedded buzzer sound synchronously on the given audio device.
func PlayBuzzer(audioDevice sdl.AudioDeviceID) error {
	return PlaySoundFromMemory(audioDevice, assets_embed.BuzzerWav)
}

// PlayPing plays the embedded "correct" ping sound synchronously on the given audio device.
func PlayPing(audioDevice sdl.AudioDeviceID) error {
	return PlaySoundFromMemory(audioDevice, assets_embed.CorrectWav)
}

