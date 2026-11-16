// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

import (
	"testing"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

func TestSoundDuration(t *testing.T) {
	cases := []struct {
		name string
		spec sdl.AudioSpec
		n    int // bytes of sample data
		want time.Duration
	}{
		{"mono s16 16 kHz, 1 s", sdl.AudioSpec{Format: sdl.AUDIO_S16LE, Channels: 1, Freq: 16000}, 32000, time.Second},
		{"stereo s16 24 kHz, 0.5 s", sdl.AudioSpec{Format: sdl.AUDIO_S16LE, Channels: 2, Freq: 24000}, 48000, 500 * time.Millisecond},
		{"mono f32 44.1 kHz, 2 s", sdl.AudioSpec{Format: sdl.AUDIO_F32LE, Channels: 1, Freq: 44100}, 4 * 88200, 2 * time.Second},
		{"not loaded", sdl.AudioSpec{}, 0, 0},
		{"zero rate", sdl.AudioSpec{Format: sdl.AUDIO_S16LE, Channels: 1}, 32000, 0},
	}
	for _, c := range cases {
		s := &Sound{Spec: c.spec, Data: make([]byte, c.n)}
		if got := s.Duration(); got != c.want {
			t.Errorf("%s: Duration() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSoundIsPlayingUnloaded(t *testing.T) {
	if (&Sound{}).IsPlaying() {
		t.Error("IsPlaying() on a sound with no stream = true, want false")
	}
}
