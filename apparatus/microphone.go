// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
)

// Microphone wraps an SDL3 audio recording stream for capturing PCM audio input.
//
// Open with NewMicrophone (default device) or NewMicrophoneFromDevice.
// Call StartCapture before reading samples; call StopCapture when done.
// Always call Close when finished.
//
// Recording is initially paused. Call StartCapture to begin, StopCapture to pause.
type Microphone struct {
	DeviceID sdl.AudioDeviceID
	Stream   *sdl.AudioStream
	Spec     sdl.AudioSpec
}

// DefaultRecordingSpec returns a mono F32LE spec at 44100 Hz suitable for voice recording.
func DefaultRecordingSpec() sdl.AudioSpec {
	return sdl.AudioSpec{
		Format:   sdl.AUDIO_F32LE,
		Channels: 1,
		Freq:     44100,
	}
}

// GetRecordingDevices returns all currently connected audio recording devices.
func GetRecordingDevices() ([]sdl.AudioDeviceID, error) {
	return sdl.GetAudioRecordingDevices()
}

// NewMicrophone opens the system default recording device with the given spec.
// Pass nil for spec to use DefaultRecordingSpec.
func NewMicrophone(spec *sdl.AudioSpec) (*Microphone, error) {
	return NewMicrophoneFromDevice(sdl.AUDIO_DEVICE_DEFAULT_RECORDING, spec)
}

// NewMicrophoneFromDevice opens a specific recording device.
// Pass nil for spec to use DefaultRecordingSpec.
func NewMicrophoneFromDevice(devID sdl.AudioDeviceID, spec *sdl.AudioSpec) (*Microphone, error) {
	if spec == nil {
		s := DefaultRecordingSpec()
		spec = &s
	}
	stream := devID.OpenAudioDeviceStream(spec, 0)
	if stream == nil {
		return nil, fmt.Errorf("microphone: OpenAudioDeviceStream failed")
	}
	// Recording streams start paused; keep them that way until StartCapture is called.
	return &Microphone{
		DeviceID: devID,
		Stream:   stream,
		Spec:     *spec,
	}, nil
}

// StartCapture flushes the stream buffer, then resumes the recording device.
// It returns the SDL3 nanosecond timestamp immediately after unpausing.
//
// Use the returned timestamp to compute voice-onset RT:
//
//	captureStartNS, _ := mic.StartCapture()
//	// after reading N samples to detect onset:
//	onsetNS := captureStartNS + uint64(N)*1_000_000_000/uint64(mic.Spec.Freq)
func (m *Microphone) StartCapture() (uint64, error) {
	if err := m.Stream.Clear(); err != nil {
		return 0, fmt.Errorf("microphone StartCapture: clear: %w", err)
	}
	ts := sdl.TicksNS()
	if err := m.Stream.ResumeDevice(); err != nil {
		return 0, fmt.Errorf("microphone StartCapture: resume: %w", err)
	}
	return ts, nil
}

// StopCapture pauses the recording device.
func (m *Microphone) StopCapture() error {
	return m.Stream.PauseDevice()
}

// ReadSamples reads up to len(buf) bytes of captured PCM from the stream.
// Returns the number of bytes actually read (may be 0 if no data is ready).
func (m *Microphone) ReadSamples(buf []byte) (int32, error) {
	return m.Stream.Data(buf)
}

// Available returns the number of bytes of captured audio waiting to be read.
func (m *Microphone) Available() (int32, error) {
	return m.Stream.Available()
}

// Close destroys the audio stream and releases resources.
func (m *Microphone) Close() {
	if m.Stream != nil {
		_ = m.Stream.PauseDevice()
		m.Stream.Destroy()
		m.Stream = nil
	}
}
