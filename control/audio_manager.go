// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package control

import (
	"sync"

	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/Zyko0/go-sdl3/sdl"
)

// AudioManager coordinates audio playback on top of a single SDL audio device.
// It provides synchronous and asynchronous helpers and ensures that all
// asynchronous playbacks are finished before shutdown.
type AudioManager struct {
	Device sdl.AudioDeviceID

	mu      sync.Mutex
	closing bool
	wg      sync.WaitGroup
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

	if err := s.PreloadDevice(device); err != nil {
		return err
	}
	if err := s.Play(); err != nil {
		_ = s.Unload()
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

	if err := s.PreloadDevice(device); err != nil {
		a.wg.Done()
		return err
	}
	if err := s.Play(); err != nil {
		_ = s.Unload()
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

// Shutdown waits for all asynchronous playbacks to finish and prevents any new
// playback requests from being started.
func (a *AudioManager) Shutdown() {
	a.mu.Lock()
	a.closing = true
	a.mu.Unlock()
	a.wg.Wait()
}

