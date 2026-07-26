// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

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
		ioStream, ioErr := sdl.IOFromBytes(s.Memory)
		if ioErr != nil {
			return fmt.Errorf("stimuli.Sound.PreloadDevice: reading WAV from memory: %w", ioErr)
		}
		data, err = sdl.LoadWAV_IO(ioStream, true, &spec)
		if err != nil {
			return fmt.Errorf("stimuli.Sound.PreloadDevice: decoding WAV from memory: %w", err)
		}
	} else {
		data, err = sdl.LoadWAV(s.FilePath, &spec)
		if err != nil {
			return fmt.Errorf("stimuli.Sound.PreloadDevice: loading WAV %q: %w", s.FilePath, err)
		}
	}
	s.Data = data
	s.Spec = spec

	// Create a stream that converts to the device's spec if needed.
	// We'll just create a stream matching the file's spec.
	stream, err := sdl.CreateAudioStream(&s.Spec, &s.Spec)
	if err != nil {
		return fmt.Errorf("stimuli.Sound.PreloadDevice: creating audio stream: %w", err)
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
		return fmt.Errorf("stimuli.Sound.Play: queueing audio data: %w", err)
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

// CurrentSampleFrame returns a best-effort estimate of the sample frame that
// is currently being played by the audio hardware.
//
// # THIS IS NOT THE EXACT CURRENT SAMPLE
//
// The SDL audio pipeline has two queues between your data and the speaker:
//
//  1. SDL's software stream buffer — drained by the audio thread.
//  2. The hardware DMA buffer — filled by the audio thread; the driver plays
//     from here.  Samples that have already left the software buffer but are
//     still in the hardware buffer have NOT been heard yet.
//
// SDL_GetAudioStreamQueued (called internally) only sees layer 1. The hardware
// buffer in layer 2 adds a constant lag that is subtracted using the device's
// reported buffer size.  The result is still an approximation: the returned
// value is accurate to within ±one hardware callback period, which at the
// default 512-frame buffer and 44100 Hz is roughly ±11.6 ms.  Calling
// [control.SetAudioSampleFrames](128) before initialization reduces the bound
// to ±2.9 ms at the cost of higher underrun risk.
//
// The reported position lags reality — it never jumps ahead.
//
// Returns 0 if the sound has not been loaded or playback has not started.
// Returns the total frame count once playback is complete.
func (s *Sound) CurrentSampleFrame() (int, error) {
	if s.Stream == nil || len(s.Data) == 0 {
		return 0, nil
	}

	bytesPerFrame := int(s.Spec.Channels) * sampleByteWidth(s.Spec.Format)
	if bytesPerFrame == 0 {
		return 0, fmt.Errorf("CurrentSampleFrame: invalid audio spec (channels=%d format=%d)",
			s.Spec.Channels, s.Spec.Format)
	}
	totalFrames := len(s.Data) / bytesPerFrame

	queued, err := s.Stream.Queued()
	if err != nil {
		return 0, fmt.Errorf("CurrentSampleFrame: %w", err)
	}
	if queued < 0 {
		queued = 0
	}

	// Estimate hardware buffer latency in source (WAV) frames.
	hwWavFrames := 0
	if deviceID := s.Stream.Device(); deviceID != 0 {
		if devSpec, hwFrames, fmtErr := deviceID.Format(); fmtErr == nil &&
			devSpec != nil && devSpec.Freq > 0 && hwFrames > 0 {
			hwLatencySec := float64(hwFrames) / float64(devSpec.Freq)
			hwWavFrames = int(hwLatencySec * float64(s.Spec.Freq))
		}
	}

	played := totalFrames - int(queued)/bytesPerFrame - hwWavFrames
	if played < 0 {
		return 0, nil
	}
	if played > totalFrames {
		return totalFrames, nil
	}
	return played, nil
}

// PlaySyncedWithFlip synchronises sound onset with the next VSYNC flip.
//
// It pauses the audio device, pre-fills the stream with the sound data (so
// the data is ready but silent), performs the display flip (which blocks until
// VSYNC), and immediately resumes the device.  Audio onset follows the flip by
// at most one audio callback period — approximately frames/sampleRate seconds,
// where frames is the hardware buffer size set via SetAudioSampleFrames.
//
// At the default 512-frame buffer (44100 Hz) the lag is ≤ 11.6 ms.
// Calling exp.SetAudioSampleFrames(128) before Initialize() reduces it to
// ≤ 2.9 ms at the cost of higher underrun risk on loaded systems.
//
// The lag is constant and measurable empirically once per setup; subtract it
// when reporting stimulus onset times.
func (s *Sound) PlaySyncedWithFlip(screen *apparatus.Screen) (uint64, error) {
	if s.Stream == nil {
		return 0, fmt.Errorf("PlaySyncedWithFlip: sound not loaded (call PreloadDevice first)")
	}
	return syncStreamWithFlip(s.Stream, s.Data, screen)
}

// syncStreamWithFlip is the shared pause-fill-flip-resume implementation used
// by both Sound.PlaySyncedWithFlip and Tone.PlaySyncedWithFlip.
func syncStreamWithFlip(stream *sdl.AudioStream, data []byte, screen *apparatus.Screen) (uint64, error) {
	if err := stream.PauseDevice(); err != nil {
		return 0, fmt.Errorf("stimuli: pausing audio device for synced playback: %w", err)
	}
	stream.Clear()
	if err := stream.PutData(data); err != nil {
		_ = stream.ResumeDevice()
		return 0, fmt.Errorf("stimuli: queueing audio data for synced playback: %w", err)
	}
	if err := stream.Flush(); err != nil {
		_ = stream.ResumeDevice()
		return 0, fmt.Errorf("stimuli: flushing audio stream for synced playback: %w", err)
	}
	flipNS, err := screen.FlipTS()
	resumeErr := stream.ResumeDevice()
	if err != nil {
		return flipNS, fmt.Errorf("stimuli: flipping display during synced playback: %w", err)
	}
	return flipNS, resumeErr
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
	if err := s.Stream.PutData(seg); err != nil {
		return fmt.Errorf("stimuli.Sound.PlaySegment: queueing audio data: %w", err)
	}
	// Flush so the resampler emits its lookahead frames; without it
	// Stream.Queued() never reaches 0 (so Wait() spins) and the tail is
	// truncated when the WAV rate differs from the device rate. Mirrors Play().
	return s.Stream.Flush()
}

// PlaySoundFromMemory is a helper to play a sound from a byte slice on a given audio device in the background.
func PlaySoundFromMemory(audioDevice sdl.AudioDeviceID, data []byte) error {
	s := NewSoundFromMemory(data)
	if err := s.PreloadDevice(audioDevice); err != nil {
		return fmt.Errorf("stimuli.PlaySoundFromMemory: %w", err)
	}
	if err := s.Play(); err != nil {
		_ = s.Unload()
		return fmt.Errorf("stimuli.PlaySoundFromMemory: %w", err)
	}
	// Synchronous behavior: wait for the sound to finish and then clean up.
	s.Wait()
	return s.Unload()
}
