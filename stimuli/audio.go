// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"fmt"
	"math"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
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
	if err := s.Stream.PutData(s.Data); err != nil {
		return err
	}
	// Flush tells the resampler that no more input is coming so it emits its
	// lookahead frames.  Without this, SDL_GetAudioStreamQueued never reaches
	// zero when the WAV sample-rate differs from the device rate (e.g. 44100
	// Hz WAV on a 48000 Hz PipeWire device), causing Sound.Wait() to spin
	// forever.
	return s.Stream.Flush()
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
		time.Sleep(10 * time.Millisecond)
	}
}

// Present plays the sound (implements Stimulus interface).
func (s *Sound) Present(screen *apparatus.Screen, clear, update bool) error {
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

// ── Segment playback ──────────────────────────────────────────────────────────

// sampleByteWidth returns the number of bytes occupied by a single sample in
// the given SDL audio format. The SDL3 format word encodes the bit depth in
// its lowest 8 bits (e.g. AUDIO_F32LE = 0x8120 → 0x20 = 32 bits → 4 bytes).
func sampleByteWidth(format sdl.AudioFormat) int {
	bits := int(format & 0xFF)
	if bits == 0 {
		return 2 // safe fallback
	}
	return bits / 8
}

// applyLinearRamps scales the leading and trailing rampFrames of the audio
// buffer with a linear fade-in and fade-out respectively. The ramp is applied
// in-place; the format must be one of AUDIO_F32*, AUDIO_S16*, or AUDIO_U8.
// For unrecognised formats the buffer is left unchanged.
func applyLinearRamps(buf []byte, format sdl.AudioFormat, channels, rampFrames int) {
	if rampFrames <= 0 {
		return
	}
	sw := sampleByteWidth(format)
	frameBytes := sw * channels
	if frameBytes == 0 || len(buf) == 0 {
		return
	}
	totalFrames := len(buf) / frameBytes
	if rampFrames > totalFrames/2 {
		rampFrames = totalFrames / 2
	}

	isBigEndian := (format>>12)&1 != 0
	isFloat := (format>>8)&1 != 0

	scaleFrame := func(frameIdx int, scale float32) {
		for ch := 0; ch < channels; ch++ {
			base := frameIdx*frameBytes + ch*sw
			switch {
			case isFloat && sw == 4:
				// 32-bit float
				var u uint32
				if isBigEndian {
					u = uint32(buf[base])<<24 | uint32(buf[base+1])<<16 | uint32(buf[base+2])<<8 | uint32(buf[base+3])
				} else {
					u = uint32(buf[base]) | uint32(buf[base+1])<<8 | uint32(buf[base+2])<<16 | uint32(buf[base+3])<<24
				}
				v := math.Float32frombits(u) * scale
				u2 := math.Float32bits(v)
				if isBigEndian {
					buf[base], buf[base+1], buf[base+2], buf[base+3] = byte(u2>>24), byte(u2>>16), byte(u2>>8), byte(u2)
				} else {
					buf[base], buf[base+1], buf[base+2], buf[base+3] = byte(u2), byte(u2>>8), byte(u2>>16), byte(u2>>24)
				}
			case !isFloat && sw == 2:
				// 16-bit integer (signed)
				var raw uint16
				if isBigEndian {
					raw = uint16(buf[base])<<8 | uint16(buf[base+1])
				} else {
					raw = uint16(buf[base]) | uint16(buf[base+1])<<8
				}
				v := int16(float32(int16(raw)) * scale)
				if isBigEndian {
					buf[base], buf[base+1] = byte(uint16(v)>>8), byte(uint16(v))
				} else {
					buf[base], buf[base+1] = byte(uint16(v)), byte(uint16(v)>>8)
				}
			case sw == 1:
				// 8-bit unsigned: silence is 128, not 0
				dev := int(buf[base]) - 128
				dev = int(float32(dev) * scale)
				buf[base] = byte(dev + 128)
			}
		}
	}

	for f := 0; f < rampFrames; f++ {
		scale := float32(f) / float32(rampFrames)
		scaleFrame(f, scale)               // fade-in
		scaleFrame(totalFrames-1-f, scale) // fade-out
	}
}

// PlaySegment plays a time-delimited segment of the sound.
//
// onset and offset are expressed in seconds from the beginning of the loaded
// audio data (e.g. onset=1.5, offset=3.0 plays 1.5 seconds starting 1.5 s in).
// Both values are clamped to the valid range [0, duration].
//
// rampSec is the duration in seconds of a linear fade-in at the segment onset
// and a symmetric fade-out at the segment offset. Pass 0 for no ramp.
//
// PlaySegment queues only the extracted segment; the original Data is never
// modified. It replaces any audio currently queued on the stream (same
// behaviour as Play).
func (s *Sound) PlaySegment(onset, offset, rampSec float64) error {
	if s.Stream == nil {
		return fmt.Errorf("PlaySegment: sound not loaded (call PreloadDevice first)")
	}
	if onset < 0 {
		onset = 0
	}
	if offset <= onset {
		return fmt.Errorf("PlaySegment: offset (%.3fs) must be greater than onset (%.3fs)", offset, onset)
	}

	freq := int(s.Spec.Freq)
	channels := int(s.Spec.Channels)
	sw := sampleByteWidth(s.Spec.Format)
	frameBytes := sw * channels

	if freq == 0 || frameBytes == 0 {
		return fmt.Errorf("PlaySegment: invalid audio spec (freq=%d channels=%d)", freq, channels)
	}

	totalFrames := len(s.Data) / frameBytes
	maxSec := float64(totalFrames) / float64(freq)

	if onset > maxSec {
		onset = maxSec
	}
	if offset > maxSec {
		offset = maxSec
	}

	startFrame := int(onset * float64(freq))
	endFrame := int(offset * float64(freq))
	if endFrame > totalFrames {
		endFrame = totalFrames
	}
	if startFrame >= endFrame {
		return nil // nothing to play
	}

	startByte := startFrame * frameBytes
	endByte := endFrame * frameBytes

	// Copy the segment so we can apply ramps without modifying s.Data.
	seg := make([]byte, endByte-startByte)
	copy(seg, s.Data[startByte:endByte])

	if rampSec > 0 {
		rampFrames := int(rampSec * float64(freq))
		applyLinearRamps(seg, s.Spec.Format, channels, rampFrames)
	}

	s.Stream.Clear()
	return s.Stream.PutData(seg)
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
