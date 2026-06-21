// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package apparatus

import (
	"fmt"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

// Keyboard provides blocking and non‑blocking helpers around SDL's keyboard
// events, mirroring the high‑level API of Expyriment.
type Keyboard struct {
	// PollKeys is injected by the control layer to avoid direct SDL polling
	// that discards non-keyboard events. It returns (firstKey, quitRequested).
	PollKeys func() (sdl.Keycode, bool)

	// PollKeysWithTS is like PollKeys but also returns the SDL3 event timestamp
	// (nanoseconds, same clock as sdl.TicksNS()). Injected by the control layer
	// alongside PollKeys; used by GetKeyEventRT.
	PollKeysWithTS func() (sdl.Keycode, uint64, bool)

	// PollKeyUps is like PollKeysWithTS but returns the first KEY_UP event
	// seen in the current polling cycle. Injected by the control layer.
	PollKeyUps func() (sdl.Keycode, uint64, bool)
}

// waitSDLKeyEvent is the shared fallback SDL event loop used by WaitKeys and
// GetKeyEventTS when no injected callback is available. It blocks until a
// matching key, ESC, quit, or timeout.
//
// Return values:
//   - (keycode, ts, nil)       — a key in keys was pressed (or any key if keys==nil)
//   - (K_ESCAPE, ts, EndLoop)  — ESC was pressed
//   - (0, 0, EndLoop)          — window-close quit event
//   - (0, 0, nil)              — timeout
//
// The hardware event timestamp ts is always populated when a key is returned.
func waitSDLKeyEvent(keys []sdl.Keycode, start uint64, timeoutMS int) (sdl.Keycode, uint64, error) {
	for {
		var event sdl.Event
		var hasEvent bool

		if timeoutMS < 0 {
			if sdl.WaitEvent(&event) == nil {
				hasEvent = true
			}
		} else {
			elapsed := int(sdl.Ticks() - start)
			remaining := timeoutMS - elapsed
			if remaining <= 0 {
				return 0, 0, nil
			}
			if sdl.WaitEventTimeout(&event, int32(remaining)) {
				hasEvent = true
			} else {
				if int(sdl.Ticks()-start) >= timeoutMS {
					return 0, 0, nil
				}
				continue
			}
		}

		if !hasEvent {
			continue
		}

		switch event.Type {
		case sdl.EVENT_QUIT:
			return 0, 0, sdl.EndLoop
		case sdl.EVENT_KEY_DOWN:
			ke := event.KeyboardEvent()
			keycode, ts := ke.Key, ke.Timestamp
			if keycode == sdl.K_ESCAPE {
				return sdl.K_ESCAPE, ts, sdl.EndLoop
			}
			if keys == nil {
				return keycode, ts, nil
			}
			for _, kc := range keys {
				if keycode == kc {
					return keycode, ts, nil
				}
			}
		}
	}
}

// Wait blocks until any key is pressed and returns its SDL keycode.
// If the ESC key or a quit event is received, it returns sdl.EndLoop.
func (k *Keyboard) Wait() (sdl.Keycode, error) {
	return k.WaitKeys(nil, -1)
}

// WaitKeys blocks until one of the specified keys is pressed or a timeout
// occurs.
//
//   - If keys is nil, any key will trigger a return.
//   - If timeoutMS is -1, it waits indefinitely.
//   - On timeout, it returns keycode 0 and nil error.
//   - On ESC or quit, it returns sdl.EndLoop.
func (k *Keyboard) WaitKeys(keys []sdl.Keycode, timeoutMS int) (sdl.Keycode, error) {
	start := sdl.Ticks()

	// Injected path: use the control-layer callback to avoid discarding
	// non-keyboard events while draining the SDL queue.
	if k.PollKeys != nil {
		for {
			if timeoutMS >= 0 {
				if int(sdl.Ticks()-start) >= timeoutMS {
					return 0, nil
				}
			}

			keycode, quit := k.PollKeys()
			if quit {
				return 0, sdl.EndLoop
			}
			if keycode != 0 {
				if keycode == sdl.K_ESCAPE {
					return sdl.K_ESCAPE, sdl.EndLoop
				}
				if keys == nil {
					return keycode, nil
				}
				for _, kc := range keys {
					if keycode == kc {
						return keycode, nil
					}
				}
			}

			time.Sleep(1 * time.Millisecond)
		}
	}

	// Fallback: direct SDL event polling when no callback is injected.
	key, _, err := waitSDLKeyEvent(keys, start, timeoutMS)
	return key, err
}

// Check polls for keyboard events without blocking and returns the first key
// pressed since the last call (or 0 if none). ESC or a quit event yields
// sdl.EndLoop.
func (k *Keyboard) Check() (sdl.Keycode, error) {
	var event sdl.Event
	for sdl.PollEvent(&event) {
		if event.Type == sdl.EVENT_KEY_DOWN {
			keycode := event.KeyboardEvent().Key
			if keycode == sdl.K_ESCAPE {
				return 0, sdl.EndLoop
			}
			return keycode, nil
		}
		if event.Type == sdl.EVENT_QUIT {
			return 0, sdl.EndLoop
		}
	}
	return 0, nil
}

// WaitKey blocks until the given key is pressed and returns an error only on
// ESC / window close. It is a convenience wrapper around WaitKeys for the
// common "wait for SPACE to continue" pattern.
func (k *Keyboard) WaitKey(key sdl.Keycode) error {
	_, err := k.WaitKeys([]sdl.Keycode{key}, -1)
	return err
}

// WaitKeysRT blocks until one of the specified keys is pressed (or a timeout
// occurs) and also returns the reaction time in milliseconds measured from
// the moment WaitKeysRT was called.
//
// The RT is a wall-clock elapsed time (sdl.Ticks delta), NOT a hardware event
// timestamp. For stimulus-onset-locked RT with nanosecond precision, use
// GetKeyEventTS instead, which returns the SDL3 KeyboardEvent.Timestamp
// directly and can be subtracted from a Screen.FlipTS() onset value.
//
// This bundles the common three-line pattern:
//
//	startTime := clock.GetTime()
//	key, err := kb.WaitKeys(keys, timeout)
//	rt := clock.GetTime() - startTime
func (k *Keyboard) WaitKeysRT(keys []sdl.Keycode, timeoutMS int) (sdl.Keycode, int64, error) {
	start := sdl.Ticks()
	key, err := k.WaitKeys(keys, timeoutMS)
	rt := int64(sdl.Ticks() - start)
	return key, rt, err
}

// GetKeyEventTS waits for one of the specified keys (or any key if keys is
// nil) and returns the keycode and SDL3 event timestamp in nanoseconds (same
// reference clock as sdl.TicksNS() and Screen.FlipTS()).
//
// Unlike WaitKeysRT — which measures elapsed time from the function call —
// the returned timestamp comes directly from the SDL3 KeyboardEvent.Timestamp
// field, which is set at hardware-interrupt time. This makes it suitable for
// computing reaction times relative to a stimulus onset captured with
// Screen.FlipTS():
//
//	onset, _ := screen.FlipTS()
//	key, keyTS, _ := kb.GetKeyEventTS(keys, -1)
//	rtNS := int64(keyTS - onset)
//
// If an event matching keys is already in the SDL queue, it is returned
// immediately without blocking. Pass timeoutMS = -1 for no timeout.
// On timeout, returns (0, 0, nil). On ESC or quit, returns sdl.EndLoop.
//
// Use GetKeyEventsTS to retrieve all events that arrived, not just the first.
func (k *Keyboard) GetKeyEventTS(keys []sdl.Keycode, timeoutMS int) (sdl.Keycode, uint64, error) {
	start := sdl.Ticks()

	// Injected path: use the control-layer callback which carries the SDL3
	// hardware event timestamp.
	if k.PollKeysWithTS != nil {
		for {
			if timeoutMS >= 0 {
				if int(sdl.Ticks()-start) >= timeoutMS {
					return 0, 0, nil
				}
			}
			keycode, ts, quit := k.PollKeysWithTS()
			if quit {
				return 0, 0, sdl.EndLoop
			}
			if keycode != 0 {
				if keycode == sdl.K_ESCAPE {
					return sdl.K_ESCAPE, ts, sdl.EndLoop
				}
				if keys == nil {
					return keycode, ts, nil
				}
				for _, kc := range keys {
					if keycode == kc {
						return keycode, ts, nil
					}
				}
			}
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Fallback: direct SDL event polling when no callback is injected.
	return waitSDLKeyEvent(keys, start, timeoutMS)
}

// simultaneityWindowMS is the time after the first key event during which
// GetKeyEventsTS continues collecting additional key events. Human
// "simultaneous" bilateral presses are typically 10–50 ms apart; 50 ms
// captures nearly all of them without adding noticeable delay to single-key
// trials.
const simultaneityWindowMS = 50

// GetKeyEventsTS waits for one or more matching key events and returns ALL
// that arrived, ordered by hardware timestamp (earliest first).
//
// The function operates in two phases:
//
//  1. It blocks until the first matching key arrives, respecting timeoutMS
//     (pass -1 for no timeout). This is identical to GetKeyEventTS.
//
//  2. After the first key, it waits up to simultaneityWindowMS (50 ms) for
//     any additional matching keys. This second phase is necessary because
//     human "simultaneous" bilateral presses (e.g. both hands at once) are
//     rarely truly simultaneous — the two KEY_DOWN events typically arrive
//     10–50 ms apart. A non-blocking drain after phase 1 would miss the
//     second key almost every time.
//
// In the common single-response case only one event is returned, with at most
// 50 ms of extra latency before the function returns. In the bilateral case
// both events are returned with their exact hardware timestamps, so inter-key
// lag is simply events[1].TimestampNS - events[0].TimestampNS.
//
// On timeout (phase 1), returns (nil, nil). On ESC or quit, returns
// sdl.EndLoop with any events collected so far.

func (k *Keyboard) GetKeyEventsTS(keys []sdl.Keycode, timeoutMS int) ([]InputEvent, error) {
	// Wait for the first matching key using the raw SDL event loop (not the
	// injected callback, which would discard simultaneous events via PollEvents).
	start := sdl.Ticks()
	firstKey, firstTS, err := waitSDLKeyEvent(keys, start, timeoutMS)
	if err != nil {
		return nil, fmt.Errorf("apparatus.Keyboard.GetKeyEventsTS: %w", err)
	}
	if firstKey == 0 {
		return nil, nil // timeout
	}

	all := []InputEvent{{Device: DeviceKeyboard, Key: firstKey, TimestampNS: firstTS}}

	// After the first key, wait up to simultaneityWindowMS for additional
	// matching keys. A non-blocking drain is not enough: the second key event
	// may still be in transit from the OS when the first one is returned.
	windowEnd := sdl.Ticks() + simultaneityWindowMS
	for {
		remaining := int(windowEnd - sdl.Ticks())
		if remaining <= 0 {
			break
		}
		var ev sdl.Event
		if !sdl.WaitEventTimeout(&ev, int32(remaining)) {
			break // window expired with no further events
		}
		switch ev.Type {
		case sdl.EVENT_QUIT:
			return all, sdl.EndLoop
		case sdl.EVENT_KEY_DOWN:
			ke := ev.KeyboardEvent()
			if ke.Key == sdl.K_ESCAPE {
				return all, sdl.EndLoop
			}
			matched := keys == nil
			for _, kc := range keys {
				if ke.Key == kc {
					matched = true
					break
				}
			}
			if matched {
				all = append(all, InputEvent{Device: DeviceKeyboard, Key: ke.Key, TimestampNS: ke.Timestamp})
			}
		}
	}

	// Sort by hardware timestamp (typically already in order).
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].TimestampNS < all[j-1].TimestampNS; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}

	return all, nil
}

// CollectKeyEventsTS records all matching key events that occur during a fixed
// time window and returns them ordered by hardware timestamp (earliest first).
//
// Unlike GetKeyEventsTS — which returns shortly after the first key arrives —
// CollectKeyEventsTS always runs for the full durationMS regardless of how
// many keys are pressed. Use it when you need a complete record of all presses
// within a known period, for example:
//
//   - Finger-tapping tasks (count and time every tap over N seconds)
//   - Free-response windows where the participant may press multiple keys
//   - Logging all responses during a stimulus stream
//
// Pass keys = nil to accept any key. Pass durationMS = 0 to do a
// non-blocking drain of whatever is already in the SDL queue. Returns an
// empty (non-nil) slice if no matching key was pressed. Returns sdl.EndLoop
// on ESC or window close, with any events collected up to that point.
func (k *Keyboard) CollectKeyEventsTS(keys []sdl.Keycode, durationMS int) ([]InputEvent, error) {
	var all []InputEvent
	start := sdl.Ticks()

	for {
		elapsed := int(sdl.Ticks() - start)
		remaining := durationMS - elapsed
		if remaining < 0 {
			remaining = 0
		}
		var ev sdl.Event
		if !sdl.WaitEventTimeout(&ev, int32(remaining)) {
			break // duration elapsed (or durationMS==0 and queue empty)
		}
		switch ev.Type {
		case sdl.EVENT_QUIT:
			return all, sdl.EndLoop
		case sdl.EVENT_KEY_DOWN:
			ke := ev.KeyboardEvent()
			if ke.Key == sdl.K_ESCAPE {
				return all, sdl.EndLoop
			}
			matched := keys == nil
			for _, kc := range keys {
				if ke.Key == kc {
					matched = true
					break
				}
			}
			if matched {
				all = append(all, InputEvent{Device: DeviceKeyboard, Key: ke.Key, TimestampNS: ke.Timestamp})
			}
		}
	}

	return all, nil
}

// Clear drains the entire SDL event queue — keyboard, mouse, gamepad, and all
// other event types — because SDL uses a single shared queue. Call it before
// a new trial to discard stale presses from any device. Do not call it after
// ShowTS/FlipTS: the participant may have already responded and the event
// would be silently discarded before GetKeyEventTS can read it.
func (k *Keyboard) Clear() {
	var event sdl.Event
	for sdl.PollEvent(&event) {
		// Just drain the queue
	}
}

// IsPressed reports whether the given key is physically held down at the
// moment of the call. It uses SDL's scancode state array (sdl.GetKeyboardState),
// which is updated by sdl.PumpEvents — no event queue involvement.
//
// Typical use: polling a held key during a stimulus loop, or checking that the
// participant has released a key before starting the next trial.
//
// Note: call sdl.PumpEvents (or exp.PollEvents) in the same loop to keep the
// window responsive and handle ESC/quit. IsPressed itself calls PumpEvents so
// the state snapshot is always fresh.
func (k *Keyboard) IsPressed(key sdl.Keycode) bool {
	sdl.PumpEvents()
	scancode := key.ScancodeFromKey(nil)
	state := sdl.GetKeyboardState()
	return int(scancode) < len(state) && state[scancode]
}

// waitSDLKeyUpEvent is the fallback SDL event loop for WaitKeyReleaseTS when
// no injected PollKeyUps callback is available. It blocks until the specified
// key generates a KEY_UP event, ESC/quit, or the timeout expires.
//
// Return values:
//   - (ts, nil)       — KEY_UP for key was received; ts is its hardware timestamp
//   - (0, EndLoop)    — ESC KEY_DOWN or window-close quit event
//   - (0, nil)        — timeout
func waitSDLKeyUpEvent(key sdl.Keycode, start uint64, timeoutMS int) (uint64, error) {
	for {
		var event sdl.Event
		var hasEvent bool

		if timeoutMS < 0 {
			if sdl.WaitEvent(&event) == nil {
				hasEvent = true
			}
		} else {
			elapsed := int(sdl.Ticks() - start)
			remaining := timeoutMS - elapsed
			if remaining <= 0 {
				return 0, nil
			}
			if sdl.WaitEventTimeout(&event, int32(remaining)) {
				hasEvent = true
			} else {
				if int(sdl.Ticks()-start) >= timeoutMS {
					return 0, nil
				}
				continue
			}
		}

		if !hasEvent {
			continue
		}

		switch event.Type {
		case sdl.EVENT_QUIT:
			return 0, sdl.EndLoop
		case sdl.EVENT_KEY_DOWN:
			if event.KeyboardEvent().Key == sdl.K_ESCAPE {
				return 0, sdl.EndLoop
			}
		case sdl.EVENT_KEY_UP:
			ke := event.KeyboardEvent()
			if ke.Key == key {
				return ke.Timestamp, nil
			}
		}
	}
}

// WaitKeyReleaseTS blocks until the given key is released (KEY_UP event) and
// returns the SDL3 hardware event timestamp in nanoseconds.
//
// Combined with the KEY_DOWN timestamp from GetKeyEventTS, this gives
// nanosecond-precision keypress duration:
//
//	key, downTS, _ := kb.GetKeyEventTS(keys, -1)
//	upTS, _        := kb.WaitKeyReleaseTS(key, -1)
//	durationNS     := upTS - downTS
//
// Pass timeoutMS = -1 for no timeout. On timeout returns (0, nil).
// On ESC or quit returns (0, sdl.EndLoop).
func (k *Keyboard) WaitKeyReleaseTS(key sdl.Keycode, timeoutMS int) (uint64, error) {
	start := sdl.Ticks()

	if k.PollKeyUps != nil {
		for {
			if timeoutMS >= 0 {
				if int(sdl.Ticks()-start) >= timeoutMS {
					return 0, nil
				}
			}
			keyUp, ts, quit := k.PollKeyUps()
			if quit {
				return 0, sdl.EndLoop
			}
			if keyUp == key {
				return ts, nil
			}
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Fallback: direct SDL event polling when no callback is injected.
	return waitSDLKeyUpEvent(key, start, timeoutMS)
}
