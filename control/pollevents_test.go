// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"testing"
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"
)

func keyEvent(typ sdl.EventType, key sdl.Keycode, ts uint64) sdl.Event {
	var ev sdl.Event
	*(*sdl.KeyboardEvent)(unsafe.Pointer(&ev)) = sdl.KeyboardEvent{Type: typ, Key: key, Timestamp: ts}
	return ev
}

func buttonEvent(button uint8, ts uint64) sdl.Event {
	var ev sdl.Event
	*(*sdl.MouseButtonEvent)(unsafe.Pointer(&ev)) = sdl.MouseButtonEvent{Type: sdl.EVENT_MOUSE_BUTTON_DOWN, Button: button, Timestamp: ts}
	return ev
}

// fakeQueue replaces the pollEvent seam with a queue of events.
func fakeQueue(t *testing.T, events ...sdl.Event) *[]sdl.Event {
	t.Helper()
	q := append([]sdl.Event(nil), events...)
	old := pollEvent
	pollEvent = func(ev *sdl.Event) bool {
		if len(q) == 0 {
			return false
		}
		*ev, q = q[0], q[1:]
		return true
	}
	t.Cleanup(func() { pollEvent = old })
	return &q
}

// Two key presses waiting in the queue must both reach the caller. PollEvents
// used to drain the whole queue and keep only the first, so a rejected key
// followed by an accepted one lost the accepted one.
func TestPollEventsDoesNotDropASecondKey(t *testing.T) {
	fakeQueue(t, keyEvent(sdl.EVENT_KEY_DOWN, sdl.K_G, 100), keyEvent(sdl.EVENT_KEY_DOWN, sdl.K_F, 200))
	e := &Experiment{}

	if s := e.PollEvents(nil); s.LastKey != sdl.K_G || s.LastKeyTimestamp != 100 {
		t.Fatalf("first poll: key %v ts %d, want G at 100", s.LastKey, s.LastKeyTimestamp)
	}
	if s := e.PollEvents(nil); s.LastKey != sdl.K_F || s.LastKeyTimestamp != 200 {
		t.Fatalf("second poll: key %v ts %d, want F at 200 — the second press was dropped", s.LastKey, s.LastKeyTimestamp)
	}
	if s := e.PollEvents(nil); s.LastKey != 0 {
		t.Errorf("third poll: key %v, want none", s.LastKey)
	}
}

// Events that fill no slot are still drained in the same cycle, so a key behind
// a burst of mouse motion is not held back, and a handler sees every event
// exactly once across calls.
func TestPollEventsDrainsMotionAndShowsEveryEventOnce(t *testing.T) {
	var motion sdl.Event
	motion.Type = sdl.EVENT_MOUSE_MOTION
	q := fakeQueue(t, motion, motion, keyEvent(sdl.EVENT_KEY_DOWN, sdl.K_J, 5), buttonEvent(1, 6), motion)
	e := &Experiment{}

	var seen []sdl.EventType
	handler := func(ev sdl.Event) bool { seen = append(seen, ev.Type); return false }

	s := e.PollEvents(handler)
	if s.LastKey != sdl.K_J {
		t.Fatalf("first poll: key %v, want J behind the motion events", s.LastKey)
	}
	if s.LastMouseButton != 0 {
		t.Errorf("first poll: button %d, want it left for the next cycle", s.LastMouseButton)
	}
	if s := e.PollEvents(handler); s.LastMouseButton != 1 || s.LastMouseTimestamp != 6 {
		t.Errorf("second poll: button %d ts %d, want 1 at 6", s.LastMouseButton, s.LastMouseTimestamp)
	}
	e.PollEvents(handler)
	if len(*q) != 0 {
		t.Errorf("%d events left in the queue", len(*q))
	}
	want := []sdl.EventType{sdl.EVENT_MOUSE_MOTION, sdl.EVENT_MOUSE_MOTION, sdl.EVENT_KEY_DOWN, sdl.EVENT_MOUSE_BUTTON_DOWN, sdl.EVENT_MOUSE_MOTION}
	if len(seen) != len(want) {
		t.Fatalf("handler saw %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("handler saw %v, want %v", seen, want)
		}
	}
}

// A signal caught by the handler goroutine must reach key waits as a quit, so
// they return EndLoop and End saves the data on the main goroutine.
func TestPollEventsReportsTheSignalQuitFlag(t *testing.T) {
	fakeQueue(t)
	e := &Experiment{}
	if e.PollEvents(nil).QuitRequested {
		t.Fatal("QuitRequested before any signal")
	}
	e.quitFlag.Store(1)
	if !e.PollEvents(nil).QuitRequested {
		t.Error("quit flag set: QuitRequested = false")
	}
}
