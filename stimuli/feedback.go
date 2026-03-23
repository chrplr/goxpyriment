// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

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
