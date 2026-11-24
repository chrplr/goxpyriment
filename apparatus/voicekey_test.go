// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
)

// The voice key measures F32 audio only; any other format used to be decoded
// as float and timed as mono. checkSpec is what Arm and WaitOnset refuse on.
func TestVoiceKeyCheckSpec(t *testing.T) {
	cases := []struct {
		name string
		spec sdl.AudioSpec
		ok   bool
	}{
		{"default mono f32", DefaultRecordingSpec(), true},
		{"stereo f32", sdl.AudioSpec{Format: sdl.AUDIO_F32LE, Channels: 2, Freq: 48000}, true},
		{"s16", sdl.AudioSpec{Format: sdl.AUDIO_S16LE, Channels: 1, Freq: 44100}, false},
		{"no channels", sdl.AudioSpec{Format: sdl.AUDIO_F32LE, Channels: 0, Freq: 44100}, false},
		{"no rate", sdl.AudioSpec{Format: sdl.AUDIO_F32LE, Channels: 1}, false},
	}
	for _, c := range cases {
		vk := NewVoiceKey(&Microphone{Spec: c.spec}, 0.02, 0)
		if err := vk.checkSpec(); (err == nil) != c.ok {
			t.Errorf("%s: checkSpec() = %v, want ok=%v", c.name, err, c.ok)
		}
	}
}
