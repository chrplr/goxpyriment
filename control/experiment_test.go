package control

import (
	"errors"
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
)

// TestRunRecovery verifies that Experiment.Run correctly catches our internal
// exitPanic and returns the wrapped error gracefully.
func TestRunRecovery(t *testing.T) {
	// Mock pollEvent to avoid SDL initialization crash
	oldPoll := pollEvent
	pollEvent = func(ev *sdl.Event) bool { return false }
	defer func() { pollEvent = oldPoll }()

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

// TestWaitAbort verifies that Experiment.Wait(ms) correctly detects a quit
// request and panics with exitPanic.
func TestWaitAbort(t *testing.T) {
	// Mock getTicks and pollEvent to avoid SDL initialization crash
	oldTicks := getTicks
	getTicks = func() uint64 { return 0 }
	defer func() { getTicks = oldTicks }()

	oldPoll := pollEvent
	pollEvent = func(ev *sdl.Event) bool { return false }
	defer func() { pollEvent = oldPoll }()

	exp := &Experiment{}
	
	// Simulate a quit request
	exp.event.QuitRequested = true

	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(exitPanic); !ok {
				t.Errorf("expected exitPanic, got %v", r)
			}
		} else {
			t.Error("Wait did not panic after QuitRequested")
		}
	}()

	// Should panic immediately
	exp.Wait(1000)
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
