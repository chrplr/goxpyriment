// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.
package stimuli

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// VisualStreamElement represents a single stimulus in a sequence with its timing.
type VisualStreamElement struct {
	Stimulus    VisualStimulus
	DurationOn  time.Duration
	DurationOff time.Duration
}

// UserEvent captures input data during the stream presentation.
type UserEvent struct {
	Event       sdl.Event     // The raw SDL event (Keyboard or Mouse)
	Timestamp   time.Duration // Time relative to the start of the stream (Go clock, millisecond precision)
	TimestampNS uint64        // SDL3 hardware event timestamp in nanoseconds (same clock as Screen.FlipNS)
}

// TimingLog provides post-hoc verification of the actual presentation times.
type TimingLog struct {
	Index        int
	TargetOn     time.Duration
	ActualOnset  time.Duration // Go-clock time of first-frame draw (stream-relative)
	ActualOffset time.Duration // Go-clock time after last on-frame (stream-relative)
	OnsetNS      uint64        // SDL3 nanosecond timestamp of the VSYNC flip that turned the stimulus on
	OffsetNS     uint64        // SDL3 nanosecond timestamp of the VSYNC flip that turned the stimulus off
}

// PresentStreamOfImages displays a sequence of stimuli with high precision.
// It preloads textures, disables GC, and aligns presentation to the monitor's VSYNC.
// Each stimulus is centered on (x, y) in screen-center coordinates.
//
// # Timing accuracy
//
// Onset jitter and compositor latency depend on the platform:
//
//   - Linux (no compositor): < 1 ms jitter; VSYNC blocks directly in the driver.
//   - Linux (Wayland / compositing WM): 1–3 ms jitter; compositor may add one
//     frame (~17 ms at 60 Hz) of fixed latency.
//   - macOS (Metal): WindowServer compositor is always active; 2–5 ms jitter;
//     0–1 frames of fixed compositor latency on top of TimingLog.OnsetNS.
//   - Windows exclusive fullscreen: < 1 ms jitter; DWM bypassed.
//   - Windows windowed (DWM): 1–3 ms jitter; one frame of compositor latency.
//
// TimingLog.OnsetNS is the SDL3 nanosecond timestamp captured immediately
// after Present() returns — it reflects GPU submission time, not photon
// emission. Hardware pipeline latency (scan-out + panel response) adds a
// further 0–2 frames that is constant across trials.
//
// Durations are rounded to the nearest whole frame. A 50 ms stimulus on a
// 60 Hz display is shown for exactly 3 frames (50.0 ms); 60 ms becomes 4
// frames (66.7 ms).
func PresentStreamOfImages(screen *apparatus.Screen, elements []VisualStreamElement, x, y float32) ([]UserEvent, []TimingLog, error) {
	// 1. Pre-load all stimuli into GPU memory (Textures)
	for _, el := range elements {
		if err := PreloadVisualOnScreen(screen, el.Stimulus); err != nil {
			return nil, nil, fmt.Errorf("failed to preload stimulus: %w", err)
		}
	}

	// 2. Timing Setup: query the display's actual refresh rate
	var refreshRate float32 = 60.0
	displayID := sdl.GetDisplayForWindow(screen.Window)
	if mode, err := displayID.CurrentDisplayMode(); err == nil && mode != nil && mode.RefreshRate > 0 {
		refreshRate = mode.RefreshRate
	}
	frameDuration := time.Duration(float64(time.Second) / float64(refreshRate))

	var userEvents []UserEvent
	var timingLogs []TimingLog

	// 3. Performance Optimization: Disable GC to prevent jitter during presentation
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	streamStartTime := time.Now()

	// 4. Presentation Loop
	for i, el := range elements {
		// Round duration to the nearest frame count
		framesOn := int((el.DurationOn + frameDuration/2) / frameDuration)
		framesOff := int((el.DurationOff + frameDuration/2) / frameDuration)

		// Center the stimulus on (x, y) before drawing
		el.Stimulus.SetPosition(sdl.FPoint{X: x, Y: y})

		actualOnset := time.Since(streamStartTime)
		var onsetNS uint64

		// --- STIMULUS ON ---
		for f := 0; f < framesOn; f++ {
			if err := screen.Clear(); err != nil {
				return userEvents, timingLogs, err
			}
			if err := el.Stimulus.Draw(screen); err != nil {
				return userEvents, timingLogs, err
			}
			if f == 0 {
				// Capture the SDL nanosecond timestamp of the actual VSYNC flip.
				ts, err := screen.FlipNS()
				if err != nil {
					return userEvents, timingLogs, err
				}
				onsetNS = ts
			} else {
				if err := screen.Update(); err != nil { // VSYNC blocks here
					return userEvents, timingLogs, err
				}
			}
			userEvents = collectEvents(streamStartTime, userEvents)
		}

		actualOffset := time.Since(streamStartTime)
		var offsetNS uint64

		// --- STIMULUS OFF (ISI / Blank screen) ---
		for f := 0; f < framesOff; f++ {
			if err := screen.Clear(); err != nil {
				return userEvents, timingLogs, err
			}
			if f == 0 {
				ts, err := screen.FlipNS()
				if err != nil {
					return userEvents, timingLogs, err
				}
				offsetNS = ts
			} else {
				if err := screen.Update(); err != nil {
					return userEvents, timingLogs, err
				}
			}
			userEvents = collectEvents(streamStartTime, userEvents)
		}

		timingLogs = append(timingLogs, TimingLog{
			Index:        i,
			TargetOn:     el.DurationOn,
			ActualOnset:  actualOnset,
			ActualOffset: actualOffset,
			OnsetNS:      onsetNS,
			OffsetNS:     offsetNS,
		})
	}

	return userEvents, timingLogs, nil
}

// MakeVisualStream builds a []VisualStreamElement from parallel slices of
// stimuli, onset times, and on-durations (all in milliseconds).
// The off-duration (ISI) for each element is derived as the gap to the next
// onset; the last element gets an off-duration of zero.
// Returns an error if the slice lengths differ or any derived ISI is negative.
func MakeVisualStream(stims []VisualStimulus, onsetMs, durationMs []int) ([]VisualStreamElement, error) {
	n := len(stims)
	if len(onsetMs) != n || len(durationMs) != n {
		return nil, fmt.Errorf("MakeVisualStream: slices have different lengths (%d, %d, %d)",
			n, len(onsetMs), len(durationMs))
	}
	elements := make([]VisualStreamElement, n)
	for i, s := range stims {
		on := time.Duration(durationMs[i]) * time.Millisecond
		var off time.Duration
		if i < n-1 {
			gap := onsetMs[i+1] - onsetMs[i] - durationMs[i]
			if gap < 0 {
				return nil, fmt.Errorf("MakeVisualStream: negative ISI at index %d", i)
			}
			off = time.Duration(gap) * time.Millisecond
		}
		elements[i] = VisualStreamElement{Stimulus: s, DurationOn: on, DurationOff: off}
	}
	return elements, nil
}

// MakeRegularVisualStream builds a []VisualStreamElement where every element
// shares the same on-duration and off-duration (ISI). This covers the common
// RSVP case where all stimuli are shown for the same amount of time.
func MakeRegularVisualStream(stims []VisualStimulus, durationOn, durationOff time.Duration) []VisualStreamElement {
	elements := make([]VisualStreamElement, len(stims))
	for i, s := range stims {
		elements[i] = VisualStreamElement{Stimulus: s, DurationOn: durationOn, DurationOff: durationOff}
	}
	return elements
}

// MakeSoundStream builds a []SoundStreamElement from parallel slices of
// sounds, onset times, and on-durations (all in milliseconds).
// The off-duration (ISI) for each element is derived as the gap to the next
// onset; the last element gets an off-duration of zero.
// Returns an error if the slice lengths differ or any derived ISI is negative.
func MakeSoundStream(sounds []AudioPlayable, onsetMs, durationMs []int) ([]SoundStreamElement, error) {
	n := len(sounds)
	if len(onsetMs) != n || len(durationMs) != n {
		return nil, fmt.Errorf("MakeSoundStream: slices have different lengths (%d, %d, %d)",
			n, len(onsetMs), len(durationMs))
	}
	elements := make([]SoundStreamElement, n)
	for i, s := range sounds {
		on := time.Duration(durationMs[i]) * time.Millisecond
		var off time.Duration
		if i < n-1 {
			gap := onsetMs[i+1] - onsetMs[i] - durationMs[i]
			if gap < 0 {
				return nil, fmt.Errorf("MakeSoundStream: negative ISI at index %d", i)
			}
			off = time.Duration(gap) * time.Millisecond
		}
		elements[i] = SoundStreamElement{Sound: s, DurationOn: on, DurationOff: off}
	}
	return elements, nil
}

// MakeRegularSoundStream builds a []SoundStreamElement where every element
// shares the same on-duration and off-duration (ISI). This covers the common
// case of a regular sequence of tones or sounds with uniform timing.
func MakeRegularSoundStream(sounds []AudioPlayable, durationOn, durationOff time.Duration) []SoundStreamElement {
	elements := make([]SoundStreamElement, len(sounds))
	for i, s := range sounds {
		elements[i] = SoundStreamElement{Sound: s, DurationOn: durationOn, DurationOff: durationOff}
	}
	return elements
}

// PresentStreamOfText handles Rapid Serial Visual Presentation (RSVP).
// It converts a slice of strings into a stream of centered text stimuli.
func PresentStreamOfText(screen *apparatus.Screen, words []string, durationOn, durationOff time.Duration, x, y float32, color sdl.Color) ([]UserEvent, []TimingLog, error) {
	elements := make([]VisualStreamElement, len(words))
	for i, word := range words {
		elements[i] = VisualStreamElement{
			Stimulus:    NewTextLine(word, 0, 0, color),
			DurationOn:  durationOn,
			DurationOff: durationOff,
		}
	}
	return PresentStreamOfImages(screen, elements, x, y)
}

// AudioPlayable is implemented by any audio stimulus that can be triggered
// on a pre-bound SDL audio device. Both *Sound and *Tone satisfy this interface.
type AudioPlayable interface {
	Play() error
}

// SoundStreamElement represents a single sound in an auditory sequence,
// mirroring VisualStreamElement for visual streams.
// A nil Sound means silence for that slot (only DurationOn + DurationOff are waited).
type SoundStreamElement struct {
	Sound       AudioPlayable
	DurationOn  time.Duration // how long the sound is considered "on"
	DurationOff time.Duration // silence after the sound (ISI)
}

// PlayStreamOfSounds plays a sequence of audio stimuli with precise timing,
// mirroring PresentStreamOfImages for the auditory domain.
//
// For each element it triggers the sound, waits DurationOn while polling
// events, then waits DurationOff while polling events. Timing of actual
// onsets and offsets is recorded in the returned TimingLog slice.
//
// All sounds must be pre-loaded (bound to an audio device) before calling.
// GC is disabled during playback to reduce timing jitter.
// ESC causes early return with sdl.EndLoop.
func PlayStreamOfSounds(elements []SoundStreamElement) ([]UserEvent, []TimingLog, error) {
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	var userEvents []UserEvent
	var timingLogs []TimingLog

	streamStart := time.Now()

	for i, el := range elements {
		actualOnset := time.Since(streamStart)

		// --- SOUND ON ---
		if el.Sound != nil {
			if err := el.Sound.Play(); err != nil {
				return userEvents, timingLogs, err
			}
		}
		onDeadline := time.Now().Add(el.DurationOn)
		for time.Now().Before(onDeadline) {
			userEvents = collectEvents(streamStart, userEvents)
			time.Sleep(1 * time.Millisecond)
		}

		actualOffset := time.Since(streamStart)

		// --- SOUND OFF (ISI / silence) ---
		offDeadline := time.Now().Add(el.DurationOff)
		for time.Now().Before(offDeadline) {
			userEvents = collectEvents(streamStart, userEvents)
			time.Sleep(1 * time.Millisecond)
		}

		timingLogs = append(timingLogs, TimingLog{
			Index:        i,
			TargetOn:     el.DurationOn,
			ActualOnset:  actualOnset,
			ActualOffset: actualOffset,
		})
	}

	return userEvents, timingLogs, nil
}

// collectEvents drains the SDL event queue without blocking, appending any
// keyboard or mouse button events to logs. Each UserEvent carries both a
// Go-clock stream-relative timestamp (Timestamp) and the SDL3 hardware event
// timestamp in nanoseconds (TimestampNS), which is on the same clock as
// Screen.FlipNS() and can be used for sub-millisecond RT computation.
func collectEvents(baseTime time.Time, logs []UserEvent) []UserEvent {
	var event sdl.Event
	for sdl.PollEvent(&event) {
		switch event.Type {
		case sdl.EVENT_KEY_DOWN, sdl.EVENT_KEY_UP:
			logs = append(logs, UserEvent{
				Event:       event,
				Timestamp:   time.Since(baseTime),
				TimestampNS: event.KeyboardEvent().Timestamp,
			})
		case sdl.EVENT_MOUSE_BUTTON_DOWN, sdl.EVENT_MOUSE_BUTTON_UP:
			logs = append(logs, UserEvent{
				Event:       event,
				Timestamp:   time.Since(baseTime),
				TimestampNS: event.MouseButtonEvent().Timestamp,
			})
		}
	}
	return logs
}
