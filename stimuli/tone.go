// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// Tone is a procedural sine-wave tone with configurable frequency (Hz),
// duration (ms), and amplitude.
//
// Like Sound, Tone implements Stimulus but NOT VisualStimulus — it has no
// position or Draw method. Its Present method ignores the screen/clear/update
// parameters and simply plays the generated waveform.
//
// Before use, call PreloadDevice to generate the PCM data and bind to an SDL
// audio device. The no-arg Preload() from the Stimulus interface is a no-op.
type Tone struct {
	Frequency float64
	Duration  int // ms
	Amplitude float32
	Stream    *sdl.AudioStream
	Data      []byte
}

// NewTone creates a sine wave tone with the given frequency in Hz, duration in milliseconds, and amplitude (0–1).
func NewTone(frequency float64, duration int, amplitude float32) *Tone {
	return &Tone{
		Frequency: frequency,
		Duration:  duration,
		Amplitude: amplitude,
	}
}

// NewComplexTone creates a Tone by summing multiple sine-wave frequencies.
// The amplitudes are averaged so the result stays within [-amplitude, amplitude].
// A linear ramp of rampMs milliseconds is applied at start and end to avoid clicks.
// The returned Tone already has its PCM data set; call PreloadDevice to bind it
// to an SDL audio device before playing.
func NewComplexTone(frequencies []float64, durationMs, rampMs int, amplitude float32) *Tone {
	sampleRate := 44100
	numSamples := (sampleRate * durationMs) / 1000
	data := make([]byte, numSamples*4) // 32-bit float, mono

	n := float64(len(frequencies))
	if n == 0 {
		n = 1
	}
	rampSamples := (sampleRate * rampMs) / 1000
	if rampSamples > numSamples/2 {
		rampSamples = numSamples / 2
	}

	for i := 0; i < numSamples; i++ {
		var sum float64
		for _, f := range frequencies {
			sum += math.Sin(2 * math.Pi * f * float64(i) / float64(sampleRate))
		}
		val := float32(sum/n) * amplitude

		// Linear fade-in / fade-out
		if rampSamples > 0 {
			if i < rampSamples {
				val *= float32(i) / float32(rampSamples)
			} else if i >= numSamples-rampSamples {
				val *= float32(numSamples-1-i) / float32(rampSamples)
			}
		}

		bits := math.Float32bits(val)
		data[i*4] = byte(bits)
		data[i*4+1] = byte(bits >> 8)
		data[i*4+2] = byte(bits >> 16)
		data[i*4+3] = byte(bits >> 24)
	}

	return &Tone{
		Frequency: frequencies[0],
		Duration:  durationMs,
		Amplitude: amplitude,
		Data:      data,
	}
}

// PreloadDevice prepares the audio stream for playback.
// If t.Data is already set (e.g. by NewComplexTone), data generation is skipped
// and only the SDL stream is created and bound to the device.
func (t *Tone) PreloadDevice(audioDevice sdl.AudioDeviceID) error {
	if t.Data == nil {
		sampleRate := 44100
		numSamples := (sampleRate * t.Duration) / 1000
		t.Data = make([]byte, numSamples*4) // 32-bit float

		for i := 0; i < numSamples; i++ {
			val := float32(math.Sin(2*math.Pi*t.Frequency*float64(i)/float64(sampleRate))) * t.Amplitude
			// Store as float32 in little endian
			bits := math.Float32bits(val)
			t.Data[i*4] = byte(bits)
			t.Data[i*4+1] = byte(bits >> 8)
			t.Data[i*4+2] = byte(bits >> 16)
			t.Data[i*4+3] = byte(bits >> 24)
		}
	}

	spec := &sdl.AudioSpec{
		Format:   sdl.AUDIO_F32LE,
		Channels: 1,
		Freq:     44100,
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

func (t *Tone) Present(screen *apparatus.Screen, clear, update bool) error {
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
