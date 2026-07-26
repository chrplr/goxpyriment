// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

// ErrVoiceTimeout is returned by VoiceKey.WaitOnset when the participant does
// not produce a vocalization within the allowed response window.
var ErrVoiceTimeout = errors.New("voice key: response timeout")

// VoiceKey detects the onset of a vocal response by monitoring the amplitude
// of audio captured from a Microphone. Amplitude is measured as the RMS over
// non-overlapping windows of WindowSz samples; the first window whose RMS
// exceeds Threshold triggers onset.
//
// Recommended threshold: 0.02–0.05 (F32LE scale 0–1). Start low and raise if
// you get false triggers from breathing or room noise.
//
// Recommended WindowSz: 128 samples at 44100 Hz ≈ 2.9 ms resolution.
//
// OnOnset, if non-nil, is called on the calling goroutine immediately after
// onset is detected and before the post-onset recording window begins. Use it
// to update the display (e.g. blank the screen) at the moment of vocal onset.
type VoiceKey struct {
	Mic       *Microphone
	Threshold float32
	WindowSz  int
	OnOnset   func()
}

// NewVoiceKey constructs a VoiceKey for the given Microphone.
// threshold is in the range 0–1 (F32LE amplitude).
// windowSz is the number of samples per RMS window; 0 uses 128.
func NewVoiceKey(mic *Microphone, threshold float32, windowSz int) *VoiceKey {
	if windowSz <= 0 {
		windowSz = 128
	}
	return &VoiceKey{
		Mic:       mic,
		Threshold: threshold,
		WindowSz:  windowSz,
	}
}

// Arm starts audio capture and returns the capture start timestamp on the SDL3
// nanosecond clock (same clock as exp.ShowTS and keyboard event timestamps).
//
// Call Arm as close as possible to — or just before — showing the stimulus,
// then pass the returned timestamp to WaitOnset.
func (vk *VoiceKey) Arm() (captureStartNS uint64, err error) {
	return vk.Mic.StartCapture()
}

// WaitOnset polls the microphone until a vocal onset is detected or the timeout
// expires. captureStartNS must be the value returned by Arm.
//
// postOnsetMS controls how long recording continues after onset detection so
// that the returned pcm contains the actual vocal response, not just the
// pre-onset silence. Pass 0 to stop immediately on onset (the old behaviour).
// A value of 1000–1500 is suitable for single-word responses.
//
// On success: returns (onsetNS, pcm, nil) where onsetNS is the SDL3 nanosecond
// timestamp of the detected onset and pcm contains all raw PCM bytes captured
// from the start of capture through the end of the post-onset window.
//
// On timeout: returns (0, pcm, ErrVoiceTimeout). pcm still contains any audio
// that was captured before the timeout.
//
// timeoutMS < 0 means no timeout (wait indefinitely).
func (vk *VoiceKey) WaitOnset(captureStartNS uint64, timeoutMS int, postOnsetMS int) (onsetNS uint64, pcm []byte, err error) {
	const readBufBytes = 2048 // read up to 2048 bytes at a time (~11.6 ms at 44100 Hz F32LE mono)

	sampleRate := uint64(vk.Mic.Spec.Freq)
	bytesPerSample := 4 // F32LE = 4 bytes per sample
	windowBytes := vk.WindowSz * bytesPerSample

	readBuf := make([]byte, readBufBytes)
	var accumulated []byte // leftover bytes from previous read that haven't formed a full window
	var totalSamplesRead uint64

	var deadline time.Time
	if timeoutMS >= 0 {
		deadline = time.Now().Add(time.Duration(timeoutMS) * time.Millisecond)
	}

	for {
		if timeoutMS >= 0 && time.Now().After(deadline) {
			_ = vk.Mic.StopCapture()
			return 0, pcm, ErrVoiceTimeout
		}

		n, readErr := vk.Mic.ReadSamples(readBuf)
		if readErr != nil {
			_ = vk.Mic.StopCapture()
			return 0, pcm, readErr
		}
		if n <= 0 {
			// No data yet; yield briefly to avoid busy-spinning.
			time.Sleep(500 * time.Microsecond)
			continue
		}

		chunk := readBuf[:n]
		pcm = append(pcm, chunk...)
		accumulated = append(accumulated, chunk...)

		// Process complete windows from accumulated bytes.
		for len(accumulated) >= windowBytes {
			window := accumulated[:windowBytes]
			if rmsF32(window) > vk.Threshold {
				onsetNS = captureStartNS + totalSamplesRead*1_000_000_000/sampleRate
				if vk.OnOnset != nil {
					vk.OnOnset()
				}
				// Continue recording for postOnsetMS to capture the vocal response.
				if postOnsetMS > 0 {
					postDeadline := time.Now().Add(time.Duration(postOnsetMS) * time.Millisecond)
					for time.Now().Before(postDeadline) {
						m, rErr := vk.Mic.ReadSamples(readBuf)
						if rErr != nil {
							break
						}
						if m > 0 {
							pcm = append(pcm, readBuf[:m]...)
						} else {
							time.Sleep(500 * time.Microsecond)
						}
					}
				}
				_ = vk.Mic.StopCapture()
				return onsetNS, pcm, nil
			}
			totalSamplesRead += uint64(vk.WindowSz)
			accumulated = accumulated[windowBytes:]
		}
	}
}

// rmsF32 computes the RMS amplitude of a slice of F32LE PCM bytes.
// Returns 0 if buf is empty or not a multiple of 4 bytes.
func rmsF32(buf []byte) float32 {
	n := len(buf) / 4
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		bits := uint32(buf[i*4]) |
			uint32(buf[i*4+1])<<8 |
			uint32(buf[i*4+2])<<16 |
			uint32(buf[i*4+3])<<24
		v := float64(math.Float32frombits(bits))
		sum += v * v
	}
	return float32(math.Sqrt(sum / float64(n)))
}

// SampleOnsetNS converts a sample index to a nanosecond timestamp.
// Convenience helper for post-hoc onset recalculation from a saved PCM buffer.
func SampleOnsetNS(captureStartNS uint64, sampleIndex int, sampleRate int) uint64 {
	return captureStartNS + uint64(sampleIndex)*1_000_000_000/uint64(sampleRate)
}

// ScanOnset scans a PCM buffer for the first window exceeding threshold and
// returns its sample index (or -1 if not found). Useful for post-hoc analysis
// of saved WAV files.
func ScanOnset(pcm []byte, threshold float32, windowSz int) int {
	if windowSz <= 0 {
		windowSz = 128
	}
	bytesPerSample := 4
	windowBytes := windowSz * bytesPerSample
	for offset := 0; offset+windowBytes <= len(pcm); offset += windowBytes {
		if rmsF32(pcm[offset:offset+windowBytes]) > threshold {
			return offset / bytesPerSample
		}
	}
	return -1
}

// DeviceNames returns a map of recording device ID → human-readable name for
// all currently connected recording devices. Useful for device selection UIs.
func DeviceNames() (map[sdl.AudioDeviceID]string, error) {
	devices, err := GetRecordingDevices()
	if err != nil {
		return nil, fmt.Errorf("apparatus.DeviceNames: %w", err)
	}
	names := make(map[sdl.AudioDeviceID]string, len(devices))
	for _, d := range devices {
		name, nameErr := d.Name()
		if nameErr == nil {
			names[d] = name
		}
	}
	return names, nil
}
