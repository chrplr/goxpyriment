// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build js

package stimuli

import (
	"errors"

	"github.com/Zyko0/go-sdl3/sdl"
)

// DefaultRecorderSpec is a placeholder on the JS/WASM build.
func DefaultRecorderSpec() sdl.AudioSpec {
	return sdl.AudioSpec{}
}

// AudioRecorder is not supported in the JavaScript/WASM build.
type AudioRecorder struct{}

// NewAudioRecorder always fails on JS/WASM.
func NewAudioRecorder(spec *sdl.AudioSpec) (*AudioRecorder, error) {
	return nil, errors.New("stimuli: audio recording is not supported in the JS/WASM build")
}

// NewAudioRecorderOnDevice always fails on JS/WASM.
func NewAudioRecorderOnDevice(device sdl.AudioDeviceID, spec *sdl.AudioSpec) (*AudioRecorder, error) {
	return nil, errors.New("stimuli: audio recording is not supported in the JS/WASM build")
}

func (r *AudioRecorder) OutputFormat() sdl.AudioSpec { return sdl.AudioSpec{} }

func (r *AudioRecorder) Start() error {
	return errors.New("stimuli: audio recording is not supported in the JS/WASM build")
}

func (r *AudioRecorder) Stop() error {
	return errors.New("stimuli: audio recording is not supported in the JS/WASM build")
}

func (r *AudioRecorder) Available() (int32, error) { return 0, r.err() }

func (r *AudioRecorder) Read(p []byte) (int, error) { return 0, r.err() }

func (r *AudioRecorder) Drain(dest []byte) ([]byte, error) { return dest, r.err() }

func (r *AudioRecorder) Close() error { return nil }

func (r *AudioRecorder) err() error {
	return errors.New("stimuli: audio recording is not supported in the JS/WASM build")
}

// WriteFloat32WAV is not supported on JS/WASM.
func WriteFloat32WAV(path string, pcm []byte, sampleRate int, channels int) error {
	return errors.New("stimuli: WriteFloat32WAV is not supported in the JS/WASM build")
}
