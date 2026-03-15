package stimuli

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/chrplr/goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
)

// StreamElement represents a single stimulus in a sequence with its timing.
type StreamElement struct {
	Stimulus    VisualStimulus
	DurationOn  time.Duration
	DurationOff time.Duration
}

// UserEvent captures input data during the stream presentation.
type UserEvent struct {
	Event     sdl.Event     // The raw SDL event (Keyboard or Mouse)
	Timestamp time.Duration // Time relative to the start of the stream
}

// TimingLog provides post-hoc verification of the actual presentation times.
type TimingLog struct {
	Index        int
	TargetOn     time.Duration
	ActualOnset  time.Duration
	ActualOffset time.Duration
}

// PresentStreamOfImages displays a sequence of stimuli with high precision.
// It preloads textures, disables GC, and aligns presentation to the monitor's VSYNC.
// Each stimulus is centered on (x, y) in screen-center coordinates.
func PresentStreamOfImages(screen *io.Screen, elements []StreamElement, x, y float32) ([]UserEvent, []TimingLog, error) {
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

		// --- STIMULUS ON ---
		for f := 0; f < framesOn; f++ {
			if err := screen.Clear(); err != nil {
				return userEvents, timingLogs, err
			}
			if err := el.Stimulus.Draw(screen); err != nil {
				return userEvents, timingLogs, err
			}
			if err := screen.Update(); err != nil { // VSYNC blocks here
				return userEvents, timingLogs, err
			}
			userEvents = collectEvents(streamStartTime, userEvents)
		}

		actualOffset := time.Since(streamStartTime)

		// --- STIMULUS OFF (ISI / Blank screen) ---
		for f := 0; f < framesOff; f++ {
			if err := screen.Clear(); err != nil {
				return userEvents, timingLogs, err
			}
			if err := screen.Update(); err != nil {
				return userEvents, timingLogs, err
			}
			userEvents = collectEvents(streamStartTime, userEvents)
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

// PresentStreamOfText handles Rapid Serial Visual Presentation (RSVP).
// It converts a slice of strings into a stream of centered text stimuli.
func PresentStreamOfText(screen *io.Screen, words []string, durationOn, durationOff time.Duration, x, y float32, color sdl.Color) ([]UserEvent, []TimingLog, error) {
	elements := make([]StreamElement, len(words))
	for i, word := range words {
		elements[i] = StreamElement{
			Stimulus:    NewTextLine(word, 0, 0, color),
			DurationOn:  durationOn,
			DurationOff: durationOff,
		}
	}
	return PresentStreamOfImages(screen, elements, x, y)
}

// collectEvents drains the SDL event queue without blocking, appending any
// keyboard or mouse button events to logs with timestamps relative to baseTime.
func collectEvents(baseTime time.Time, logs []UserEvent) []UserEvent {
	var event sdl.Event
	for sdl.PollEvent(&event) {
		switch event.Type {
		case sdl.EVENT_KEY_DOWN, sdl.EVENT_KEY_UP,
			sdl.EVENT_MOUSE_BUTTON_DOWN, sdl.EVENT_MOUSE_BUTTON_UP:
			logs = append(logs, UserEvent{
				Event:     event,
				Timestamp: time.Since(baseTime),
			})
		}
	}
	return logs
}
