// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/chrplr/goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
)

// Sound represents an audio stimulus loaded from a WAV file or byte slice.
//
// Sound implements Stimulus but NOT VisualStimulus — it has no position or
// Draw method. Its Present method ignores the screen/clear/update parameters
// and simply plays the audio via the bound SDL audio stream.
//
// Before use, call PreloadDevice to bind to an SDL audio device. The no-arg
// Preload() from the Stimulus interface is a no-op because audio streams
// require a specific device ID that is not available at construction time.
type Sound struct {
	FilePath string
	Memory   []byte
	Data     []byte
	Spec     sdl.AudioSpec
	Stream   *sdl.AudioStream
}

// NewSound creates a new Sound stimulus from a WAV file.
func NewSound(filePath string) *Sound {
	return &Sound{
		FilePath: filePath,
	}
}

// NewSoundFromMemory creates a new Sound stimulus from embedded data.
func NewSoundFromMemory(data []byte) *Sound {
	return &Sound{
		Memory: data,
	}
}

// PreloadDevice loads the WAV file and prepares the audio stream.
func (s *Sound) PreloadDevice(audioDevice sdl.AudioDeviceID) error {
	var spec sdl.AudioSpec
	var data []byte
	var err error

	if s.Memory != nil {
		ioStream, err := sdl.IOFromBytes(s.Memory)
		if err != nil {
			return err
		}
		data, err = sdl.LoadWAV_IO(ioStream, true, &spec)
		if err != nil {
			return err
		}
	} else {
		data, err = sdl.LoadWAV(s.FilePath, &spec)
		if err != nil {
			return err
		}
	}
	s.Data = data
	s.Spec = spec

	// Create a stream that converts to the device's spec if needed.
	// We'll just create a stream matching the file's spec.
	stream, err := sdl.CreateAudioStream(&s.Spec, &s.Spec)
	if err != nil {
		return err
	}
	s.Stream = stream

	return audioDevice.BindAudioStream(s.Stream)
}

// Play plays the sound.
func (s *Sound) Play() error {
	if s.Stream == nil {
		return nil
	}
	// Clear any remaining data and put new data
	s.Stream.Clear()
	return s.Stream.PutData(s.Data)
}

// Wait blocks until the sound has finished playing.
func (s *Sound) Wait() {
	if s.Stream == nil {
		return
	}
	for {
		n, _ := s.Stream.Queued()
		if n <= 0 {
			break
		}
		sdl.Delay(10)
	}
}

// Present plays the sound (implements Stimulus interface).
func (s *Sound) Present(screen *io.Screen, clear, update bool) error {
	return s.Play()
}

func (s *Sound) Preload() error { return nil }

func (s *Sound) Unload() error {
	if s.Stream != nil {
		s.Stream.Destroy()
		s.Stream = nil
	}
	return nil
}

// PlaySoundFromMemory is a helper to play a sound from a byte slice on a given audio device in the background.
func PlaySoundFromMemory(audioDevice sdl.AudioDeviceID, data []byte) error {
	s := NewSoundFromMemory(data)
	if err := s.PreloadDevice(audioDevice); err != nil {
		return err
	}
	if err := s.Play(); err != nil {
		_ = s.Unload()
		return err
	}
	// Synchronous behavior: wait for the sound to finish and then clean up.
	s.Wait()
	return s.Unload()
}
