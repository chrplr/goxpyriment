// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
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
//
// The OnsetNS / OffsetNS pair is the authoritative timing record. Both are SDL3
// nanosecond timestamps on the sdl.TicksNS() clock — the same clock that stamps
// input events (UserEvent.TimestampNS, Keyboard.GetKeyEventTS) and VSYNC flips
// (Screen.FlipTS). A reaction time or a displayed duration is therefore a plain
// subtraction within this one clock:
//
//	rtNS  := int64(event.TimestampNS - tl.OnsetNS)
//	durNS := int64(tl.OffsetNS - tl.OnsetNS)
//
// ActualOnset / ActualOffset are Go-clock (time.Now) DIAGNOSTICS ONLY, retained
// for coarse pacing checks. They live on a different timebase (different origin)
// from the NS fields and from input events, so they must never be subtracted
// from an event timestamp or logged as a canonical onset/offset. ActualOffset in
// particular is sampled one frame before the clearing flip, so using it as the
// offset reads ~1 frame early — use OffsetNS for any real timing.
type TimingLog struct {
	Index        int
	TargetOn     time.Duration
	ActualOnset  time.Duration // DIAGNOSTIC (Go clock): first-frame draw, stream-relative; not for RT
	ActualOffset time.Duration // DIAGNOSTIC (Go clock): after last on-frame, stream-relative; not for RT
	OnsetNS      uint64        // AUTHORITATIVE: SDL3 ns timestamp (sdl.TicksNS clock) of the stimulus onset
	OffsetNS     uint64        // AUTHORITATIVE: SDL3 ns timestamp (sdl.TicksNS clock) of the stimulus offset
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
// Note: on held (audio / non-visual) frames the stream re-renders the
// carried-over content (the last stream visual, or a blank) before invoking the
// callback, so an overlay drawn every frame does not accumulate.
//
// Content drawn outside the stream (e.g. an exp.Show before the call) is not
// carried over: the stream cannot reconstruct what it did not draw, so a held
// slot with no preceding stream visual shows the background. Put such content in
// the stream as a VisualStimulus if it needs to persist.
type FrameCallback func(ctx FrameContext) error

// OnsetCallback is invoked once for each stream element that has a stimulus
// onset, immediately AFTER the VSYNC flip that turns the element on (for a held
// audio element whose DurationOn rounds to zero frames, after the first-off-
// frame flip at which Play() is triggered). It is the post-flip counterpart to
// FrameCallback.
//
//   - index is the element's position in the elements slice (and in the returned
//     TimingLog slice).
//   - onsetNS is the SDL3 nanosecond timestamp of that flip — the exact value
//     recorded as TimingLog[index].OnsetNS, on the same clock as input-event
//     timestamps (UserEvent.TimestampNS, Keyboard.GetKeyEventTS).
//
// This is the hook for emitting a hardware TTL aligned to the *displayed* onset:
// unlike FrameCallback, which runs one frame earlier (pre-flip, at GPU-
// submission time), OnsetCallback runs on the photon side of the frame boundary,
// so the trigger and the logged OnsetNS coincide.
//
// It runs on the timing-critical path with GC disabled, between the onset flip
// and the next frame, so it MUST be short and non-blocking: emit an edge
// (device.SetHigh, or device.Send for a per-stimulus code) here and clear it
// from a later FrameCallback or after the stream — never time.Sleep or call a
// blocking Pulse, or a frame will be missed. Elements with a nil Stimulus (pure
// delay / hold slots) have no onset and do not fire it.
//
// Return nil to continue, sdl.EndLoop to stop the stream gracefully (reported
// via IsEndLoop), or any other error to abort.
type OnsetCallback func(index int, onsetNS uint64) error

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
//   - Audio / non-visual elements (and a nil Stimulus) hold the previous frame
//     for the whole slot: the last stream visual (or a blank, after an ISI) is
//     re-rendered every frame rather than relying on GPU backbuffer persistence,
//     which SDL leaves undefined after a present and which flickers on
//     double-buffered drivers. The stimulus is triggered once, immediately after
//     the first VSYNC flip of the slot, by calling its Present(screen, false,
//     false) method (Sound and Tone play on Present). This lets a visual placed
//     just before a sound remain on screen while the sound plays. Content drawn
//     outside the stream, with no intervening stream visual, cannot be
//     reconstructed and is instead held by re-presenting the front buffer.
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
//
// To also emit a hardware trigger aligned to each stimulus onset, use
// PresentStreamOfStimuliHooks with an OnsetCallback.
func PresentStreamOfStimuliFunc(screen *apparatus.Screen, elements []StreamElement, x, y float32, onFrame FrameCallback) ([]UserEvent, []TimingLog, error) {
	return PresentStreamOfStimuliHooks(screen, elements, x, y, onFrame, nil)
}

// PresentStreamOfStimuliHooks is PresentStreamOfStimuliFunc plus an optional
// post-flip onset hook. Either or both of onFrame and onOnset may be nil; with
// both nil it behaves exactly like PresentStreamOfStimuli.
//
// onFrame runs once per frame immediately before each flip (overlays, real-time
// feedback — see FrameCallback). onOnset runs once per element immediately after
// its onset flip, with that flip's SDL-clock timestamp, so a hardware TTL emitted
// there is aligned to the displayed onset and to the logged TimingLog.OnsetNS
// (see OnsetCallback). This is the function to use for per-stimulus triggering.
func PresentStreamOfStimuliHooks(screen *apparatus.Screen, elements []StreamElement, x, y float32, onFrame FrameCallback, onOnset OnsetCallback) ([]UserEvent, []TimingLog, error) {
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

	// Held-content tracking. A held (audio / non-visual) slot must keep the
	// last visual content on screen across many Present() calls. Relying on the
	// GPU backbuffer persisting between swaps is unsafe — SDL leaves the
	// backbuffer undefined after a present, so on strictly double-buffered
	// drivers repeated bare Present() calls flicker between the two buffers.
	// Instead we remember what was last drawn and re-render it into the
	// backbuffer every held frame, keeping both buffers identical.
	//
	//   - heldVisual != nil          → re-Clear + re-Draw that stimulus
	//   - otherwise                  → re-Clear to background
	//
	// The second case covers both an off-phase (deliberately blank) and content
	// drawn outside the stream before the call, e.g. an exp.Show. That external
	// content is NOT preserved, and cannot be: reading it back with
	// RenderReadPixels would read the backbuffer, which the caller's own present
	// already invalidated, so the capture would be undefined — on a
	// double-buffered driver it returns the frame before the caller's. The stream
	// has no way to reconstruct what it did not draw.
	//
	// This branch used to return nil, presenting a frame carrying no commands at
	// all in the hope that the front buffer would persist. That is worse than
	// undefined: a frame with no draw calls is not reliably scanned out under a
	// compositor (see apparatus.Screen.fillWholeTarget), so the display could
	// freeze on a stale frame for the whole held slot. Clearing is deterministic,
	// and Screen.Clear guarantees the frame carries real draw work.
	//
	// If you need content to survive into a held slot, put it in the stream as a
	// VisualStimulus rather than drawing it beforehand.
	var heldVisual VisualStimulus
	renderHeld := func() error {
		if heldVisual != nil {
			if err := screen.Clear(); err != nil {
				return err
			}
			return heldVisual.Draw(screen)
		}
		return screen.Clear()
	}

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
		// triggered guards the one-shot Present() of a held (audio / non-visual)
		// stimulus. The trigger normally fires on the first on-frame, but an
		// element whose DurationOn rounds to zero frames has no on-phase, so the
		// trigger must fall through to the first off-frame instead — otherwise
		// the sound would be silently dropped.
		triggered := false

		// --- STIMULUS ON ---
		for f := 0; f < framesOn; f++ {
			// Visual elements redraw the stimulus every frame; audio / non-visual
			// elements re-render the carried-over held content every frame (both
			// keep the two display buffers identical, avoiding flicker).
			if isVisual {
				if err := screen.Clear(); err != nil {
					return userEvents, timingLogs, err
				}
				if err := visual.Draw(screen); err != nil {
					return userEvents, timingLogs, err
				}
			} else {
				if err := renderHeld(); err != nil {
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
					triggered = true
				}
				// Post-flip onset hook: emit a hardware trigger aligned to the
				// displayed onset (== onsetNS == TimingLog.OnsetNS). Skipped for
				// nil-Stimulus hold/delay slots, which have no stimulus onset.
				if onOnset != nil && el.Stimulus != nil {
					if err := onOnset(i, onsetNS); err != nil {
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

		// After the on-phase, the last content drawn was this element's visual
		// (if any); record it so a following held slot can reproduce it.
		if isVisual && framesOn > 0 {
			heldVisual = visual
		}

		actualOffset := time.Since(streamStartTime)
		var offsetNS uint64

		// --- STIMULUS OFF (ISI) ---
		// Visual elements blank the screen; non-visual elements re-render the
		// carried-over held content (avoiding backbuffer-persistence flicker).
		for f := 0; f < framesOff; f++ {
			if isVisual {
				if err := screen.Clear(); err != nil {
					return userEvents, timingLogs, err
				}
			} else {
				if err := renderHeld(); err != nil {
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
				// A held stimulus whose DurationOn rounded to zero frames was not
				// triggered in the (empty) on-phase; fire it here at the slot's
				// first actual flip and anchor its onset to that VSYNC.
				if !isVisual && el.Stimulus != nil && !triggered {
					if err := el.Stimulus.Present(screen, false, false); err != nil {
						return userEvents, timingLogs, err
					}
					triggered = true
					onsetNS = ts
					// Post-flip onset hook for a held audio element with no
					// on-phase: its onset is this first off-frame flip.
					if onOnset != nil {
						if err := onOnset(i, onsetNS); err != nil {
							return userEvents, timingLogs, err
						}
					}
				}
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

		// A visual element with a non-zero ISI ends with the screen blanked, so a
		// following held slot should reproduce the blank rather than the stimulus.
		if isVisual && framesOff > 0 {
			heldVisual = nil
		}

		// When the ISI rounds to zero frames (e.g. the final element of a
		// contiguous stream) the off-phase never runs, so no clearing flip
		// occurs inside this call and offsetNS was never captured. The stimulus
		// is nonetheless held on screen for framesOn frames after its onset
		// VSYNC before the next flip takes it down; synthesise that takedown
		// time from the hardware onset so the offset stays on the SDL clock and
		// symmetric with the onset, rather than leaving callers to fall back on
		// the ~1-frame-early Go-clock ActualOffset.
		if offsetNS == 0 && onsetNS != 0 && framesOn > 0 {
			offsetNS = onsetNS + uint64(framesOn)*uint64(frameDuration.Nanoseconds())
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
//
// To emit a hardware trigger at each sound onset, use PlayStreamOfSoundsHook.
func PlayStreamOfSounds(elements []SoundStreamElement) ([]UserEvent, []TimingLog, error) {
	return PlayStreamOfSoundsHook(elements, nil)
}

// PlayStreamOfSoundsHook is PlayStreamOfSounds with an optional onset hook.
//
// onOnset (when non-nil) fires once for each element whose Sound is non-nil,
// immediately after Play() is triggered, with the SDL-clock onset timestamp
// (== TimingLog[index].OnsetNS). It is the auditory counterpart of the visual
// stream's OnsetCallback — there is no VSYNC flip here, so the onset is the
// Play() instant. The same non-blocking, keep-it-short contract applies (see
// OnsetCallback): emit an edge and clear it later, never sleep or block. A
// silent slot (nil Sound) has no onset and does not fire it.
func PlayStreamOfSoundsHook(elements []SoundStreamElement, onOnset OnsetCallback) ([]UserEvent, []TimingLog, error) {
	defer disableGC()()

	var userEvents []UserEvent
	var timingLogs []TimingLog

	streamStart := time.Now()

	for i, el := range elements {
		actualOnset := time.Since(streamStart)
		// SDL-clock onset marker, on the same timebase as input-event and flip
		// timestamps (see TimingLog). This is the authoritative onset; the
		// Go-clock actualOnset above is only a diagnostic.
		onsetNS := sdl.TicksNS()

		// --- SOUND ON ---
		if el.Sound != nil {
			if err := el.Sound.Play(); err != nil {
				return userEvents, timingLogs, err
			}
			onsetNS = sdl.TicksNS() // re-anchor to the instant the sound was triggered
			// Post-trigger onset hook: emit a hardware trigger aligned to the
			// sound onset (== onsetNS == TimingLog.OnsetNS). Silent slots (nil
			// Sound) have no onset and are skipped.
			if onOnset != nil {
				if err := onOnset(i, onsetNS); err != nil {
					return userEvents, timingLogs, err
				}
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
		offsetNS := sdl.TicksNS() // authoritative SDL-clock offset (end of the on-phase)

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
			OnsetNS:      onsetNS,
			OffsetNS:     offsetNS,
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
