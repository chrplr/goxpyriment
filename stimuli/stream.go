// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.
package stimuli

import (
	"fmt"
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

// StreamElement is a single slot in a heterogeneous stream. Stimulus may be any
// visual or audio stimulus (anything satisfying the broad Stimulus interface).
// A nil Stimulus produces a pure delay: the previous frame is held for the
// combined DurationOn + DurationOff.
//
// See PresentStreamOfStimuli for the presentation semantics.
type StreamElement struct {
	Stimulus    Stimulus
	DurationOn  time.Duration
	DurationOff time.Duration
}

// UserEvent captures input data during the stream presentation.
type UserEvent struct {
	Event       sdl.Event     // The raw SDL event (Keyboard or Mouse)
	Timestamp   time.Duration // Time relative to the start of the stream (Go clock, millisecond precision)
	TimestampNS uint64        // SDL3 hardware event timestamp in nanoseconds (same clock as Screen.FlipTS)
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

// FrameContext carries per-frame state passed to a FrameCallback. It is built
// fresh every frame, just before the VSYNC flip.
type FrameContext struct {
	Screen     *apparatus.Screen // draw overlays here (rendered on top of the stimulus, before the flip)
	Index      int               // index of the element currently being presented
	Frame      int               // frame number within the current phase (0-based)
	OnPhase    bool              // true during the stimulus-on phase, false during the off/ISI phase
	FirstFrame bool              // true on the first frame of the current phase
	NowNS      uint64            // sdl.TicksNS() captured at callback time (pre-flip); use as "now" for deadlines
	Elapsed    time.Duration     // time since the stream started
	Events     []UserEvent       // all input events collected so far (through the previous frame)
}

// FrameCallback is invoked once per frame, immediately before the VSYNC flip,
// for every on- and off-phase frame of every element. Use it to:
//
//   - draw a persistent overlay (trial counter, frame border, fixation) on
//     ctx.Screen — it is rendered on top of the current stimulus and is visible
//     on the frame about to be flipped;
//   - run real-time logic against ctx.NowNS / ctx.Events (e.g. fire a feedback
//     sound the instant a response window lapses).
//
// Return nil to continue, sdl.EndLoop to stop the stream gracefully (handled
// like an ESC press, reported via IsEndLoop), or any other error to abort.
//
// Note: for held (audio / non-visual) frames the screen is not cleared, so an
// overlay drawn every frame will accumulate; overlays are intended for visual
// streams.
type FrameCallback func(ctx FrameContext) error

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
	// Visual streams are a special case of the generic heterogeneous stream:
	// every element is a VisualStimulus. Convert and delegate so the VSYNC-locked
	// loop lives in one place (PresentStreamOfStimuli).
	generic := make([]StreamElement, len(elements))
	for i, el := range elements {
		generic[i] = StreamElement{
			Stimulus:    el.Stimulus,
			DurationOn:  el.DurationOn,
			DurationOff: el.DurationOff,
		}
	}
	return PresentStreamOfStimuli(screen, generic, x, y)
}

// PresentStreamOfStimuli displays a sequence of heterogeneous stimuli with high
// precision, allowing visual and audio stimulus types to be mixed in a single
// stream (e.g. text → picture → tone → shape → sound). Elements are presented
// strictly sequentially: each occupies a slot of DurationOn + DurationOff.
//
// The loop is VSYNC-locked and GC is disabled, exactly as for the visual-only
// PresentStreamOfImages (which delegates here). Durations are rounded to whole
// frames; see that function's doc comment for timing-accuracy details.
//
// # Per-element behaviour
//
//   - Visual elements (those satisfying VisualStimulus) are centered on (x, y)
//     and redrawn every frame: the screen is cleared, the stimulus drawn, and
//     the backbuffer flipped, for DurationOn; then cleared (blank) for DurationOff.
//
//   - Audio / non-visual elements (and a nil Stimulus) do NOT clear the screen —
//     the previous frame is held for the whole slot. The stimulus is triggered
//     once, immediately after the first VSYNC flip of the slot, by calling its
//     Present(screen, false, false) method (Sound and Tone play on Present).
//     This lets a visual placed just before a sound remain on screen while the
//     sound plays.
//
// # Timing
//
// For every element, TimingLog.OnsetNS is the SDL3 nanosecond timestamp of the
// VSYNC flip that began the slot. For audio elements this is the known VSYNC
// reference at which Play() was triggered; the sound is audible at most one
// audio callback period later (see Sound.PlaySyncedWithFlip).
//
// # Preconditions
//
// Visual elements are GPU-preloaded automatically. Audio elements MUST already
// be bound to the audio device via PreloadDevice(exp.AudioDevice) before this
// call — the same precondition as PlayStreamOfSounds.
//
// ESC or a window-close event causes early return with sdl.EndLoop.
//
// To draw a per-frame overlay or run real-time feedback logic, use
// PresentStreamOfStimuliFunc with a FrameCallback.
func PresentStreamOfStimuli(screen *apparatus.Screen, elements []StreamElement, x, y float32) ([]UserEvent, []TimingLog, error) {
	return PresentStreamOfStimuliFunc(screen, elements, x, y, nil)
}

// PresentStreamOfStimuliFunc is PresentStreamOfStimuli with an optional
// per-frame callback. When onFrame is nil it behaves identically to
// PresentStreamOfStimuli; otherwise onFrame is invoked once per frame
// (immediately before each flip) so callers can draw persistent overlays or
// drive real-time feedback. See FrameCallback for the contract.
func PresentStreamOfStimuliFunc(screen *apparatus.Screen, elements []StreamElement, x, y float32, onFrame FrameCallback) ([]UserEvent, []TimingLog, error) {
	// 1. Pre-load visual stimuli into GPU memory. Audio stimuli are skipped:
	//    they are bound to the audio device by the caller via PreloadDevice, and
	//    PreloadVisualOnScreen's Draw fallback would not apply to them.
	for _, el := range elements {
		if v, ok := el.Stimulus.(VisualStimulus); ok {
			if err := PreloadVisualOnScreen(screen, v); err != nil {
				return nil, nil, fmt.Errorf("failed to preload stimulus: %w", err)
			}
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
	defer disableGC()()

	streamStartTime := time.Now()

	// 4. Presentation Loop
	for i, el := range elements {
		// Round duration to the nearest frame count. A positive but
		// sub-half-frame duration is clamped to a single frame so that a
		// requested stimulus (or ISI) is never silently dropped to 0 frames.
		framesOn := int((el.DurationOn + frameDuration/2) / frameDuration)
		if framesOn == 0 && el.DurationOn > 0 {
			framesOn = 1
		}
		framesOff := int((el.DurationOff + frameDuration/2) / frameDuration)
		if framesOff == 0 && el.DurationOff > 0 {
			framesOff = 1
		}

		visual, isVisual := el.Stimulus.(VisualStimulus)
		if isVisual {
			// Center the stimulus on (x, y) before drawing
			visual.SetPosition(sdl.FPoint{X: x, Y: y})
		}

		actualOnset := time.Since(streamStartTime)
		var onsetNS uint64

		// --- STIMULUS ON ---
		for f := 0; f < framesOn; f++ {
			// Visual elements redraw every frame; audio / non-visual elements
			// hold the previous frame (no clear, no draw).
			if isVisual {
				if err := screen.Clear(); err != nil {
					return userEvents, timingLogs, err
				}
				if err := visual.Draw(screen); err != nil {
					return userEvents, timingLogs, err
				}
			}
			// Per-frame callback (pre-flip): draw overlays / run feedback logic.
			if onFrame != nil {
				if err := onFrame(FrameContext{
					Screen: screen, Index: i, Frame: f, OnPhase: true, FirstFrame: f == 0,
					NowNS: sdl.TicksNS(), Elapsed: time.Since(streamStartTime), Events: userEvents,
				}); err != nil {
					return userEvents, timingLogs, err
				}
			}
			if f == 0 {
				// Capture the SDL nanosecond timestamp of the actual VSYNC flip.
				// PacedFlipTS busy-waits the frame boundary on drivers that do
				// not block in Present (NVIDIA+compositor, Wayland mailbox, Pi).
				ts, err := screen.PacedFlipTS()
				if err != nil {
					return userEvents, timingLogs, err
				}
				onsetNS = ts
				// Trigger a non-visual stimulus (e.g. Sound/Tone) once, right
				// after the flip, so its onset is anchored to a known VSYNC.
				if !isVisual && el.Stimulus != nil {
					if err := el.Stimulus.Present(screen, false, false); err != nil {
						return userEvents, timingLogs, err
					}
				}
			} else {
				if err := screen.PacedFlip(); err != nil { // VSYNC-paced frame
					return userEvents, timingLogs, err
				}
			}
			prev := len(userEvents)
			userEvents = collectEvents(streamStartTime, userEvents)
			if escOrQuit(userEvents[prev:]) {
				return userEvents, timingLogs, sdl.EndLoop
			}
		}

		actualOffset := time.Since(streamStartTime)
		var offsetNS uint64

		// --- STIMULUS OFF (ISI) ---
		// Visual elements blank the screen; non-visual elements keep holding
		// the previous frame.
		for f := 0; f < framesOff; f++ {
			if isVisual {
				if err := screen.Clear(); err != nil {
					return userEvents, timingLogs, err
				}
			}
			// Per-frame callback (pre-flip): draw overlays / run feedback logic.
			if onFrame != nil {
				if err := onFrame(FrameContext{
					Screen: screen, Index: i, Frame: f, OnPhase: false, FirstFrame: f == 0,
					NowNS: sdl.TicksNS(), Elapsed: time.Since(streamStartTime), Events: userEvents,
				}); err != nil {
					return userEvents, timingLogs, err
				}
			}
			if f == 0 {
				ts, err := screen.PacedFlipTS()
				if err != nil {
					return userEvents, timingLogs, err
				}
				offsetNS = ts
			} else {
				if err := screen.PacedFlip(); err != nil {
					return userEvents, timingLogs, err
				}
			}
			prev := len(userEvents)
			userEvents = collectEvents(streamStartTime, userEvents)
			if escOrQuit(userEvents[prev:]) {
				return userEvents, timingLogs, sdl.EndLoop
			}
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

// MakeStream builds a []StreamElement from parallel slices of (possibly mixed)
// stimuli, onset times, and on-durations (all in milliseconds). It is the
// heterogeneous analogue of MakeVisualStream: stims may freely mix visual and
// audio stimulus types for use with PresentStreamOfStimuli.
// The off-duration (ISI) for each element is derived as the gap to the next
// onset; the last element gets an off-duration of zero.
// Returns an error if the slice lengths differ or any derived ISI is negative.
func MakeStream(stims []Stimulus, onsetMs, durationMs []int) ([]StreamElement, error) {
	n := len(stims)
	if len(onsetMs) != n || len(durationMs) != n {
		return nil, fmt.Errorf("MakeStream: slices have different lengths (%d, %d, %d)",
			n, len(onsetMs), len(durationMs))
	}
	elements := make([]StreamElement, n)
	for i, s := range stims {
		on := time.Duration(durationMs[i]) * time.Millisecond
		var off time.Duration
		if i < n-1 {
			gap := onsetMs[i+1] - onsetMs[i] - durationMs[i]
			if gap < 0 {
				return nil, fmt.Errorf("MakeStream: negative ISI at index %d", i)
			}
			off = time.Duration(gap) * time.Millisecond
		}
		elements[i] = StreamElement{Stimulus: s, DurationOn: on, DurationOff: off}
	}
	return elements, nil
}

// MakeRegularStream builds a []StreamElement where every element shares the same
// on-duration and off-duration (ISI). It is the heterogeneous analogue of
// MakeRegularVisualStream.
func MakeRegularStream(stims []Stimulus, durationOn, durationOff time.Duration) []StreamElement {
	elements := make([]StreamElement, len(stims))
	for i, s := range stims {
		elements[i] = StreamElement{Stimulus: s, DurationOn: durationOn, DurationOff: durationOff}
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
	defer disableGC()()

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
			prev := len(userEvents)
			userEvents = collectEvents(streamStart, userEvents)
			if escOrQuit(userEvents[prev:]) {
				return userEvents, timingLogs, sdl.EndLoop
			}
			time.Sleep(1 * time.Millisecond)
		}

		actualOffset := time.Since(streamStart)

		// --- SOUND OFF (ISI / silence) ---
		offDeadline := time.Now().Add(el.DurationOff)
		for time.Now().Before(offDeadline) {
			prev := len(userEvents)
			userEvents = collectEvents(streamStart, userEvents)
			if escOrQuit(userEvents[prev:]) {
				return userEvents, timingLogs, sdl.EndLoop
			}
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

// escOrQuit reports whether any event in the slice is an ESC key-down or a quit event.
func escOrQuit(events []UserEvent) bool {
	for _, ue := range events {
		switch ue.Event.Type {
		case sdl.EVENT_QUIT:
			return true
		case sdl.EVENT_KEY_DOWN:
			if ue.Event.KeyboardEvent().Key == sdl.K_ESCAPE {
				return true
			}
		}
	}
	return false
}

// collectEvents drains the SDL event queue without blocking, appending any
// keyboard or mouse button events to logs. Each UserEvent carries both a
// Go-clock stream-relative timestamp (Timestamp) and the SDL3 hardware event
// timestamp in nanoseconds (TimestampNS), which is on the same clock as
// Screen.FlipTS() and can be used for sub-millisecond RT computation.
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
