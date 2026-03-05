// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"math"
	"goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
)

// Tone represents a procedural audio stimulus.
type Tone struct {
	Frequency float64
	Duration  int // ms
	Amplitude float32
	Stream    *sdl.AudioStream
	Data      []byte
}

// NewTone creates a new sine wave tone.
func NewTone(frequency float64, duration int, amplitude float32) *Tone {
	return &Tone{
		Frequency: frequency,
		Duration:  duration,
		Amplitude: amplitude,
	}
}

// PreloadDevice generates the tone's PCM data.
func (t *Tone) PreloadDevice(audioDevice sdl.AudioDeviceID) error {
	sampleRate := 44100
	numSamples := (sampleRate * t.Duration) / 1000
	t.Data = make([]byte, numSamples*4) // 32-bit float
	
	for i := 0; i < numSamples; i++ {
		val := float32(math.Sin(2 * math.Pi * t.Frequency * float64(i) / float64(sampleRate))) * t.Amplitude
		// Store as float32 in little endian
		bits := math.Float32bits(val)
		t.Data[i*4] = byte(bits)
		t.Data[i*4+1] = byte(bits >> 8)
		t.Data[i*4+2] = byte(bits >> 16)
		t.Data[i*4+3] = byte(bits >> 24)
	}

	spec := &sdl.AudioSpec{
		Format:   sdl.AUDIO_F32LE,
		Channels: 1,
		Freq:     int32(sampleRate),
	}
	
	stream, err := sdl.CreateAudioStream(spec, spec)
	if err != nil {
		return err
	}
	t.Stream = stream
	
	return audioDevice.BindAudioStream(t.Stream)
}

func (t *Tone) Play() error {
	if t.Stream != nil {
		t.Stream.Clear()
		return t.Stream.PutData(t.Data)
	}
	return nil
}

func (t *Tone) Present(screen *io.Screen, clear, update bool) error {
	return t.Play()
}

func (t *Tone) Preload() error { return nil }

func (t *Tone) Unload() error {
	if t.Stream != nil {
		t.Stream.Destroy()
		t.Stream = nil
	}
	return nil
}
