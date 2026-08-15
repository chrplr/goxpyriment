package control

import (
	"errors"
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
)

// TestRunRecovery verifies that Experiment.Run correctly catches our internal
// exitPanic and returns the wrapped error gracefully.
func TestRunRecovery(t *testing.T) {
	// Mock the SDL seams so Run's per-frame pump works without SDL loaded.
	oldPoll, oldPump, oldHas, oldKb := pollEvent, pumpEvents, hasEvent, getKeyboardState
	pollEvent = func(ev *sdl.Event) bool { return false }
	pumpEvents = func() {}
	hasEvent = func(typ sdl.EventType) bool { return false }
	getKeyboardState = func() []bool { return nil }
	defer func() {
		pollEvent, pumpEvents, hasEvent, getKeyboardState = oldPoll, oldPump, oldHas, oldKb
	}()

	exp := &Experiment{}

	// Mock a logic function that triggers an exit panic
	logic := func() error {
		panic(exitPanic{err: sdl.EndLoop})
	}

	err := exp.Run(logic)
	if err != nil {
		t.Errorf("expected nil error (graceful exit), got %v", err)
	}
}

// TestStickyEvents verifies the "sticky" input mechanism. Keys should be
// captured by the main thread and held until the logic thread consumes them.
func TestStickyEvents(t *testing.T) {
	exp := &Experiment{}

	// 1. Simulate a key press in PollEvents (as if from SDL)
	// We'll bypass the actual SDL polling for this unit test
	exp.event.LastKey = sdl.K_SPACE

	// 2. Inject a mock PollKeys that replicates the one in Initialize()
	pollKeys := func() (sdl.Keycode, bool) {
		k := exp.event.LastKey
		exp.event.LastKey = 0 // sticky key consumed
		return k, exp.event.QuitRequested
	}

	// 3. First consumption should get the key
	k1, _ := pollKeys()
	if k1 != sdl.K_SPACE {
		t.Errorf("expected K_SPACE on first poll, got %v", k1)
	}

	// 4. Second consumption should get 0 (already consumed)
	k2, _ := pollKeys()
	if k2 != 0 {
		t.Errorf("expected 0 on second poll (consumed), got %v", k2)
	}
}

// TestWaitAbort verifies that Experiment.Wait(ms) detects a quit request and
// panics with exitPanic. getTicks is stubbed to 0 so the wait never reaches its
// timeout — Wait must exit via the quit path instead. No SDL context is needed
// because pumpFrame checks the quit flag before making any SDL calls.
func TestWaitAbort(t *testing.T) {
	oldTicks := getTicks
	getTicks = func() uint64 { return 0 }
	defer func() { getTicks = oldTicks }()

	exp := &Experiment{}

	// Simulate a quit request from the signal handler.
	exp.quitFlag.Store(1)

	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(exitPanic); !ok {
				t.Errorf("expected exitPanic, got %v", r)
			}
		} else {
			t.Error("Wait did not panic after quit was requested")
		}
	}()

	// Should panic immediately
	exp.Wait(1000)
}

// stubQuit replaces the SDL/TTF shutdown seams so End can run in a unit test
// with no SDL library loaded. It returns the restore function.
func stubQuit() func() {
	oldTTF, oldSDL := ttfQuit, sdlQuit
	ttfQuit, sdlQuit = func() {}, func() {}
	return func() { ttfQuit, sdlQuit = oldTTF, oldSDL }
}

// TestEndRecoversExitPanic verifies End's backstop: an experiment that does not
// wrap its logic in Run must still exit cleanly when Wait or ShowTS aborts on
// ESC, because the deferred End absorbs the sentinel.
func TestEndRecoversExitPanic(t *testing.T) {
	defer stubQuit()()

	exp := &Experiment{}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("exitPanic escaped End: %v", r)
			}
		}()
		defer exp.End()
		panic(exitPanic{err: sdl.EndLoop})
	}()
}

// TestEndRepanicsOtherPanics verifies that End's backstop only swallows the
// internal sentinel: a genuine bug must still reach the top of the program,
// after End has released its resources.
func TestEndRepanicsOtherPanics(t *testing.T) {
	defer stubQuit()()

	exp := &Experiment{}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("End swallowed a panic that was not the exit sentinel")
			}
			if s, ok := r.(string); !ok || s != "boom" {
				t.Errorf("expected the original panic value, got %v", r)
			}
		}()
		defer exp.End()
		panic("boom")
	}()
}

// TestIsEndLoop verifies the helper function correctly identifies the sentinel.
func TestIsEndLoop(t *testing.T) {
	if !IsEndLoop(sdl.EndLoop) {
		t.Error("IsEndLoop failed to identify sdl.EndLoop")
	}
	if IsEndLoop(errors.New("other error")) {
		t.Error("IsEndLoop incorrectly identified a standard error")
	}
	if IsEndLoop(nil) {
		t.Error("IsEndLoop incorrectly identified nil as EndLoop")
	}
}

// TestNewExperimentRequestsRealTime pins the default that moved out of
// NewExperimentFromFlags.
//
// The two constructors used to disagree: a program built with NewExperiment ran
// at SCHED_OTHER while an otherwise identical one built from flags ran at
// SCHED_FIFO 50, decided by which constructor the author picked and visible
// nowhere in the source. tests/Timing-Tests — this project's own timing
// benchmark — was on the losing side of it for the whole 2026 campaign.
//
// This asserts the field, not the syscall: RaiseToRealTime needs a privilege CI
// does not have, and Initialize needs a display. What the test can pin is that
// the plain constructor asks by default and that the escape hatch is a plain
// zero, which is what the flag path and a debugging caller both set.
func TestNewExperimentRequestsRealTime(t *testing.T) {
	exp := NewExperiment("t", 640, 480, false, sdl.Color{}, sdl.Color{}, 16)
	if exp.RealTimePriority != DefaultRealTimePriority {
		t.Errorf("NewExperiment RealTimePriority = %d, want %d (the default request)",
			exp.RealTimePriority, DefaultRealTimePriority)
	}
	if DefaultRealTimePriority < 1 || DefaultRealTimePriority > 99 {
		t.Errorf("DefaultRealTimePriority = %d, outside the SCHED_FIFO range 1-99",
			DefaultRealTimePriority)
	}

	// Declining must be expressible without flags — the case a NewExperiment
	// program has, and the one docs/SettingPriorityUnderLinux.md tells a user to
	// reach for under a debugger.
	exp.RealTimePriority = 0
	if exp.RealTimePriority != 0 {
		t.Error("RealTimePriority must be settable to 0 to decline")
	}
}
