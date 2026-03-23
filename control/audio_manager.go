// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package control

import (
	"fmt"
	"sync"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/stimuli"
)

// SetAudioSampleFrames sets the SDL hint that controls how many sample frames
// the audio device uses per hardware callback (i.e. the hardware buffer size).
// Smaller values reduce output latency; larger values reduce underrun risk.
// Common values: 256, 512, 1024, 2048. The default is platform-dependent.
//
// This must be called BEFORE [NewExperiment] / [NewExperimentFromFlags] —
// the hint is consumed when the audio device is opened during initialization.
// To read back the actual value after init: exp.AudioDevice.GetAudioDeviceFormat().
func SetAudioSampleFrames(frames int) {
	sdl.SetHint(sdl.HINT_AUDIO_DEVICE_SAMPLE_FRAMES, fmt.Sprintf("%d", frames))
}

// AudioManager coordinates audio playback on top of a single SDL audio device.
// It provides synchronous and asynchronous helpers and ensures that all
// asynchronous playbacks are finished before shutdown.
type AudioManager struct {
	Device sdl.AudioDeviceID

	mu      sync.Mutex
	closing bool
	wg      sync.WaitGroup
}

// prepareAndPlay loads the sound onto the manager's device and starts playback.
// It is the common setup shared by PlaySync and PlayAsync.
// Returns the device ID (0 if unavailable) and any error from preload or play.
func (a *AudioManager) prepareAndPlay(s *stimuli.Sound) (sdl.AudioDeviceID, error) {
	if err := s.PreloadDevice(a.Device); err != nil {
		return a.Device, err
	}
	if err := s.Play(); err != nil {
		_ = s.Unload()
		return a.Device, err
	}
	return a.Device, nil
}

// PlaySync plays the provided Sound synchronously: it preloads the sound on the
// manager's device, starts playback, waits for it to finish, and then unloads
// resources before returning.
func (a *AudioManager) PlaySync(s *stimuli.Sound) error {
	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		return nil
	}
	device := a.Device
	a.mu.Unlock()

	if device == 0 {
		return nil
	}

	if _, err := a.prepareAndPlay(s); err != nil {
		return err
	}
	s.Wait()
	return s.Unload()
}

// PlayAsync plays the provided Sound asynchronously: it starts playback and
// returns immediately, while a background goroutine waits for completion and
// unloads resources. Shutdown waits for all async playbacks to finish.
func (a *AudioManager) PlayAsync(s *stimuli.Sound) error {
	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		return nil
	}
	device := a.Device
	a.wg.Add(1)
	a.mu.Unlock()

	if device == 0 {
		a.wg.Done()
		return nil
	}

	if _, err := a.prepareAndPlay(s); err != nil {
		a.wg.Done()
		return err
	}

	go func() {
		defer a.wg.Done()
		s.Wait()
		_ = s.Unload()
	}()

	return nil
}

// PlayMemorySync is a convenience wrapper to synchronously play a sound from
// an in-memory WAV byte slice.
func (a *AudioManager) PlayMemorySync(data []byte) error {
	s := stimuli.NewSoundFromMemory(data)
	return a.PlaySync(s)
}

// PlayMemoryAsync is a convenience wrapper to asynchronously play a sound from
// an in-memory WAV byte slice.
func (a *AudioManager) PlayMemoryAsync(data []byte) error {
	s := stimuli.NewSoundFromMemory(data)
	return a.PlayAsync(s)
}

// PlayBuzzer plays the embedded buzzer sound asynchronously (negative feedback).
func (a *AudioManager) PlayBuzzer() error {
	return a.PlayMemoryAsync(assets_embed.BuzzerWav)
}

// PlayCorrect plays the embedded correct sound asynchronously (positive feedback).
func (a *AudioManager) PlayCorrect() error {
	return a.PlayMemoryAsync(assets_embed.CorrectWav)
}

// Shutdown waits for all asynchronous playbacks to finish and prevents any new
// playback requests from being started.
func (a *AudioManager) Shutdown() {
	a.mu.Lock()
	a.closing = true
	a.mu.Unlock()
	a.wg.Wait()
}
