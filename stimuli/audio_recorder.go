// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-developed by Álvaro Cabana <almadana@gmail.com> with Cursor (2026).
// Distributed under the GNU General Public License v3.

//go:build !js

package stimuli

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/Zyko0/go-sdl3/sdl"
)

// DefaultRecorderSpec returns mono 32-bit float PCM at 44.1 kHz, a common
// choice for microphone capture via SDL3.
func DefaultRecorderSpec() sdl.AudioSpec {
	return sdl.AudioSpec{
		Format:   sdl.AUDIO_F32LE,
		Channels: 1,
		Freq:     44100,
	}
}

// AudioRecorder captures PCM from an SDL recording device using a single
// [sdl.AudioStream] opened via [sdl.AUDIO_DEVICE_DEFAULT_RECORDING.OpenAudioDeviceStream].
// The device starts paused per SDL; call [AudioRecorder.Start] before reading.
//
// Call [AudioRecorder.Close] before the application shuts down SDL (e.g. before
// [github.com/chrplr/goxpyriment/control.Experiment.End] finishes), or assign
// the recorder with [github.com/chrplr/goxpyriment/control.Experiment.OpenAudioRecorder]
// so [github.com/chrplr/goxpyriment/control.Experiment.End] closes it automatically.
type AudioRecorder struct {
	mu     sync.Mutex
	stream *sdl.AudioStream
	out    sdl.AudioSpec
	closed bool
}

// NewAudioRecorder opens the default recording device. spec is the PCM layout
// you will read with [AudioRecorder.Read] (SDL's "app side" output format for
// capture). Pass nil to let SDL pick a format; use [AudioRecorder.OutputFormat]
// after construction to discover it.
func NewAudioRecorder(spec *sdl.AudioSpec) (*AudioRecorder, error) {
	return NewAudioRecorderOnDevice(sdl.AUDIO_DEVICE_DEFAULT_RECORDING, spec)
}

// NewAudioRecorderOnDevice opens a specific recording device id (from
// [sdl.GetAudioRecordingDevices]). spec may be nil for system default.
func NewAudioRecorderOnDevice(device sdl.AudioDeviceID, spec *sdl.AudioSpec) (*AudioRecorder, error) {
	st := device.OpenAudioDeviceStream(spec, 0)
	if st == nil {
		return nil, errors.New("stimuli: OpenAudioDeviceStream failed for recording device")
	}
	r := &AudioRecorder{stream: st}
	var src, dst sdl.AudioSpec
	if err := st.Format(&src, &dst); err != nil {
		st.Destroy()
		return nil, fmt.Errorf("stimuli: recording stream format: %w", err)
	}
	r.out = dst
	return r, nil
}

// OutputFormat returns the [sdl.AudioSpec] describing PCM returned by [AudioRecorder.Read].
func (r *AudioRecorder) OutputFormat() sdl.AudioSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.out
}

// Start resumes capture on the recording device.
func (r *AudioRecorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.stream == nil {
		return errors.New("stimuli: AudioRecorder is closed")
	}
	return r.stream.ResumeDevice()
}

// Stop pauses capture (no new samples are queued).
func (r *AudioRecorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.stream == nil {
		return errors.New("stimuli: AudioRecorder is closed")
	}
	return r.stream.PauseDevice()
}

// Available returns how many bytes can be read immediately without blocking.
func (r *AudioRecorder) Available() (int32, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.stream == nil {
		return 0, errors.New("stimuli: AudioRecorder is closed")
	}
	return r.stream.Available()
}

// Read copies up to len(p) captured bytes into p. It returns the number of
// bytes copied and any error. When no data is ready, it returns (0, nil).
func (r *AudioRecorder) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.stream == nil {
		return 0, errors.New("stimuli: AudioRecorder is closed")
	}
	avail, err := r.stream.Available()
	if err != nil {
		return 0, err
	}
	if avail <= 0 {
		return 0, nil
	}
	n := int(avail)
	if n > len(p) {
		n = len(p)
	}
	got, err := r.stream.Data(p[:n])
	if err != nil {
		return 0, err
	}
	return int(got), nil
}

// Drain appends all currently queued PCM into dest and returns the extended slice.
func (r *AudioRecorder) Drain(dest []byte) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.stream == nil {
		return dest, errors.New("stimuli: AudioRecorder is closed")
	}
	for {
		avail, err := r.stream.Available()
		if err != nil {
			return dest, err
		}
		if avail <= 0 {
			return dest, nil
		}
		chunk := make([]byte, avail)
		got, err := r.stream.Data(chunk)
		if err != nil {
			return dest, err
		}
		if got <= 0 {
			return dest, nil
		}
		dest = append(dest, chunk[:got]...)
	}
}

// Close stops capture and destroys the SDL stream (and closes the device).
// It is safe to call more than once.
func (r *AudioRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if r.stream != nil {
		_ = r.stream.PauseDevice()
		r.stream.Destroy()
		r.stream = nil
	}
	return nil
}

const waveFormatIEEEFloat = 3

// WriteFloat32WAV writes a Microsoft WAVE file containing IEEE float32 little-endian
// PCM. channels must be 1 or 2; sampleRate is Hz; pcm is raw interleaved frames
// (little-endian float32).
func WriteFloat32WAV(path string, pcm []byte, sampleRate int, channels int) error {
	if channels < 1 || channels > 2 {
		return fmt.Errorf("stimuli: WriteFloat32WAV: channels must be 1 or 2, got %d", channels)
	}
	if sampleRate <= 0 {
		return errors.New("stimuli: WriteFloat32WAV: invalid sample rate")
	}
	if len(pcm)%int(4*channels) != 0 {
		return errors.New("stimuli: WriteFloat32WAV: pcm length must be a multiple of 4*channels")
	}

	dataSize := uint32(len(pcm))
	blockAlign := uint16(channels * 4)
	byteRate := uint32(sampleRate) * uint32(blockAlign)
	subchunk1Size := uint32(16)
	chunkSize := uint32(36) + dataSize

	buf := make([]byte, 44+len(pcm))
	copy(buf[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(buf[4:8], chunkSize)
	copy(buf[8:12], []byte("WAVE"))
	copy(buf[12:16], []byte("fmt "))
	binary.LittleEndian.PutUint32(buf[16:20], subchunk1Size)
	binary.LittleEndian.PutUint16(buf[20:22], waveFormatIEEEFloat)
	binary.LittleEndian.PutUint16(buf[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], byteRate)
	binary.LittleEndian.PutUint16(buf[32:34], blockAlign)
	binary.LittleEndian.PutUint16(buf[34:36], 32)
	copy(buf[36:40], []byte("data"))
	binary.LittleEndian.PutUint32(buf[40:44], dataSize)
	copy(buf[44:], pcm)

	return os.WriteFile(path, buf, 0o644)
}
