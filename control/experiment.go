// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/results"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/sysinfo"
)

// calibrationFrames is how many frames Initialize presents to measure the
// actual refresh rate. Enough for a stable median without a visible delay:
// 60 frames is one second at 60 Hz, half of that at 120 Hz.
const calibrationFrames = 60

// EventState provides a convenient summary of the last processed input events.
// It is updated by Experiment.PollEvents.
type EventState struct {
	LastKey                    sdl.Keycode
	LastMouseButton            uint32
	LastKeyTimestamp           uint64 // SDL3 event timestamp in nanoseconds (same clock as TicksNS)
	LastMouseTimestamp         uint64 // SDL3 event timestamp in nanoseconds
	LastKeyUp                  sdl.Keycode
	LastKeyUpTimestamp         uint64 // SDL3 event timestamp in nanoseconds for KEY_UP
	LastMouseButtonUp          uint32
	LastMouseButtonUpTimestamp uint64 // SDL3 event timestamp in nanoseconds for MOUSE_BUTTON_UP
	QuitRequested              bool
}

// ---------------------------------------------------------------------------
// Experiment — facade that ties together the subsystems of a running experiment
// ---------------------------------------------------------------------------

// Experiment manages the global state of a behavioral or psychophysics experiment.
// It owns the SDL window/renderer (`Screen`), input devices (`Keyboard`, `Mouse`),
// audio device/manager, and the `DataFile` used for logging responses.
//
// It acts as a **facade**: most of its methods are thin delegations to the
// focused subsystem packages (apparatus.Screen, apparatus.Keyboard, design.Experiment, etc.).
// This keeps the user-facing API simple while the real logic lives in each
// subsystem.
//
// Typical usage:
//
//	exp := control.NewExperiment("My Experiment", 1368, 1024, false)
//	if err := exp.Initialize(); err != nil { log.Fatal(err) }
//	defer exp.End()
//
//	err := exp.Run(func() error {
//		// draw stimuli using exp.Screen / stimuli package
//		// collect input via exp.Keyboard / exp.HandleEvents
//		// log responses via exp.Data.Add(...)
//		// return control.EndLoop to terminate the run loop
//		return control.EndLoop
//	})
//	if err != nil && !control.IsEndLoop(err) { log.Fatal(err) }
type Experiment struct {
	Name            string
	Design          *design.Experiment
	Screen          *apparatus.Screen
	Keyboard        *apparatus.Keyboard
	Mouse           *apparatus.Mouse
	Data            *results.DataFile
	SubjectID       int
	BackgroundColor sdl.Color
	ForegroundColor sdl.Color
	DefaultFontSize float32
	DefaultFont     *ttf.Font
	AudioDevice     sdl.AudioDeviceID
	Audio           *AudioManager
	WindowWidth     int
	WindowHeight    int
	Fullscreen      bool
	// ScreenNumber selects the target monitor (0 = primary display).
	// Set before calling Initialize(). Use control.ListDisplays() to
	// enumerate available displays.
	ScreenNumber    int
	OutputDirectory string
	// Info holds the key→value map returned by GetParticipantInfo, if called.
	Info map[string]string
	// CursorVisible controls whether the mouse pointer is shown over the
	// experiment window. The zero value is false, so Initialize() hides the
	// cursor: an experiment window is a stimulus surface and a stray pointer on
	// it is an unintended distractor.
	//
	// Set it to true before Initialize(), or call ShowCursor() after it, for
	// mouse-driven paradigms. ShowCursor and HideCursor keep this field in sync,
	// so it always reflects the current state.
	CursorVisible bool
	// GammaCorrector, when non-nil, is applied by CorrectColor.
	// Set via SetGamma or by assigning apparatus.NewGammaCorrector(...) directly.
	GammaCorrector *apparatus.GammaCorrector
	// Microphone, when non-nil, is the audio recording device opened by
	// OpenMicrophone. Closed automatically by End().
	Microphone *apparatus.Microphone
	// RealTimePriority is the SCHED_FIFO priority Initialize() asks the OS for,
	// or 0 to not ask at all. NewExperiment sets it to DefaultRealTimePriority.
	//
	// It lives on the Experiment rather than inside NewExperimentFromFlags
	// because the elevation is not a property of how the program was launched.
	// It used to be requested only on the flag path, so a program built with
	// NewExperiment + Initialize ran at SCHED_OTHER while an otherwise identical
	// one built with NewExperimentFromFlags ran at SCHED_FIFO 50 — a difference
	// worth milliseconds on a loaded host, decided by which constructor the
	// author happened to pick, and invisible in the source. tests/Timing-Tests,
	// this project's own timing benchmark, was on the wrong side of it.
	//
	// Set it to 0 before Initialize() to decline — the escape hatch that
	// -no-realtime provides on the flag path, for programs that have no flags.
	// Under a debugger that matters: a breakpoint hit on a real-time thread can
	// leave the desktop unresponsive until the process is killed (see
	// docs/SettingPriorityUnderLinux.md).
	RealTimePriority int

	sdlLoader interface{ Unload() }
	imgLoader interface{ Unload() }
	ttfLoader interface{ Unload() }

	event    EventState
	quitFlag atomic.Int32 // set to 1 by signal handler goroutine; checked by pumpFrame
	endOnce  sync.Once    // guards finalizeData so the footer is written exactly once
	crashed  bool         // set by platformHandleCrash (js) on an unrecovered panic; suppresses the partial-data download
}

// finalizeData writes the end-time footer and flushes the data file exactly
// once, whether the experiment ends normally (End) or via the Ctrl-C/SIGTERM
// signal handler. The sync.Once guard prevents duplicate footer lines and a
// concurrent append to the output buffer from the two goroutines.
func (e *Experiment) finalizeData() {
	e.endOnce.Do(func() {
		if e.Data != nil {
			// On a browser crash (see platformHandleCrash), the buffered data is
			// partial and the participant has already been shown an error
			// overlay; skip the download so they are not handed a broken file.
			if e.crashed {
				return
			}
			e.Data.WriteEndTime()
			if err := e.Data.Finalize(); err == nil {
				log.Printf("Results saved in %s (info: %s)", e.Data.FullPath, e.Data.InfoFile.FullPath)
			}
		}
	})
}

// Do executes the given function on the current goroutine (which is the
// main SDL thread when called from inside exp.Run). It exists as a named
// wrapper so that code can be annotated as "this must run on the render
// thread" without needing a separate dispatch mechanism.
func (e *Experiment) Do(f func() error) error {
	return f()
}

// exitPanic is a internal sentinel used to abort the experiment loop
// gracefully (e.g. on ESC or window close) without requiring manual
// error propagation in every line of user code.
type exitPanic struct {
	err error
}

// NewExperiment creates a new Experiment instance with the requested logical
// window size, fullscreen flag, background/foreground colors, and default font size.
//
// If width and height are non‑zero, they define the logical coordinate space
// used for drawing (even if the physical window is scaled).
//
// If width == 0 and height == 0, the experiment will automatically switch to
// exclusive fullscreen at the current desktop resolution during Initialize().
func NewExperiment(name string, width, height int, fullscreen bool, bg, fg sdl.Color, defaultFontSize float32) *Experiment {
	return &Experiment{
		Name:            name,
		Design:          design.NewExperiment(name),
		BackgroundColor: bg,
		ForegroundColor: fg,
		DefaultFontSize: defaultFontSize,
		SubjectID:       0, // Default subject ID
		WindowWidth:     width,
		WindowHeight:    height,
		Fullscreen:      fullscreen,
		OutputDirectory: "",
		// Requested by Initialize, not here: this is a constructor and filling a
		// struct should not change the calling thread's scheduling policy. See
		// the field's own comment.
		RealTimePriority: DefaultRealTimePriority,
	}
}

// DefaultRealTimePriority is the SCHED_FIFO priority requested at startup.
//
// 50 is the middle of the 1-99 range and matches the rtprio grant the setup in
// docs/SettingPriorityUnderLinux.md installs. Asking for more than the granted
// limit fails with the same error as having no grant at all, so the two numbers
// are kept equal deliberately.
const DefaultRealTimePriority = 50

// NewExperimentFromFlags creates and initializes an experiment using the
// standard command-line flags accepted by every goxpyriment program:
//
//   - `-w`    windowed mode: opens a 1024×768 window instead of fullscreen
//   - `-d N`  display ID: open the window / fullscreen on monitor N (0 = primary; default -1 = primary)
//   - `-s N`  subject ID (default 0)
//
// Any extra flags the caller registered with the flag package before calling
// this function are parsed at the same time, so register experiment-specific
// flags first, then call NewExperimentFromFlags.
//
// # Automatic session-setup dialog
//
// When -s is NOT given on the command line — for example when the binary is
// launched by double-clicking its icon — a small setup dialog opens so the
// experimenter can enter the subject code and confirm the monitor, windowed/
// fullscreen mode, and results folder. It seeds its defaults from any -w/-d
// that were passed and remembers the display/fullscreen/folder choices across
// sessions (the subject code is always asked fresh). The dialog is skipped when:
//
//   - -s N is passed (that value is used, no dialog — the historical behaviour);
//   - -headless is passed (field defaults are used with no window, for batch runs);
//   - the program already called GetParticipantInfo itself (no double dialog);
//   - running in a browser (GOOS=js), where flags come from the page URL's
//     query string instead (?s=3&w) and no dialog is possible.
//
// Cancelling or closing the dialog exits the program cleanly.
//
// The experiment is fully initialized (SDL, audio, font, data file) before
// being returned. If initialization fails the program exits via log.Fatal.
// The caller should defer exp.End() immediately after this call.
func NewExperimentFromFlags(name string, bg, fg sdl.Color, fontSize float32) *Experiment {
	windowed := flag.Bool("w", false, "Windowed mode (1024×768 window instead of fullscreen)")
	display := flag.Int("d", -1, "Display ID: monitor index where the window/fullscreen will open (-1 = primary)")
	exclusive := flag.String("exclusive-fullscreen", "auto",
		"Fullscreen presentation: auto | on (exclusive, bypasses the compositor where possible) | off (fullscreen-desktop)")
	subject := flag.Int("s", 0, "Subject ID")
	noRealtime := flag.Bool("no-realtime", false,
		"Do not request real-time scheduling priority (see -realtime-priority)")
	realtimePrio := flag.Int("realtime-priority", 50,
		"Real-time scheduling priority to request at startup (1-99); must not exceed the RLIMIT_RTPRIO granted to this user")
	// In the browser (GOOS=js) there is no command line: synthesize os.Args
	// from the page URL's query string (?s=3&w) now that all flags exist.
	platformPrepareFlags()
	flag.Parse()

	// Detect whether -s was set explicitly: the default (0) is a valid subject
	// ID, so the value alone cannot distinguish "not given" from "given as 0".
	sProvided := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "s" {
			sProvided = true
		}
	})

	// Applies to the window created inside Initialize below, so it must be set
	// before then. A bad value is fatal rather than ignored: silently falling
	// back to "auto" would change what the session measures without saying so.
	policy, policyErr := ParseFullscreenPolicy(*exclusive)
	if policyErr != nil {
		log.Fatalf("-exclusive-fullscreen: %v", policyErr)
	}
	SetFullscreenPolicy(policy)

	// Session settings, seeded from the command-line flags.
	windowedMode := *windowed
	screenNumber := 0
	if *display >= 0 {
		screenNumber = *display
	}
	subjectID := *subject
	outputDir := ""
	var info map[string]string

	// No subject ID on the command line (e.g. launched by clicking the icon):
	// pop the setup dialog. GetParticipantInfo handles its own SDL lifecycle and
	// self-skips under -headless (returning field defaults with no window), which
	// yields subject 0 exactly as before. In the browser the dialog cannot run
	// (platformInteractiveSetup is false there); URL parameters take its place.
	if !sProvided && !participantInfoCollected && platformInteractiveSetup() {
		fullscreenDefault := "true"
		if windowedMode {
			fullscreenDefault = "false"
		}
		fields := []InfoField{
			{Name: "subject_id", Label: "Subject ID"},
			{Name: "display_id", Label: "Display ID (0 = primary monitor)", Default: fmt.Sprintf("%d", screenNumber)},
			{Name: "fullscreen", Label: "Fullscreen mode", Default: fullscreenDefault, Type: FieldCheckbox},
			{Name: "output_dir", Label: "Results folder", Default: results.DefaultDataDir()},
		}
		gathered, err := GetParticipantInfo(name, fields)
		if errors.Is(err, ErrCancelled) {
			os.Exit(0)
		}
		if err != nil {
			log.Fatalf("session-setup dialog: %v", err)
		}
		info = gathered
		if id, convErr := strconv.Atoi(strings.TrimSpace(info["subject_id"])); convErr == nil {
			subjectID = id
		}
		screenNumber = DisplayIDFromInfo(info)
		windowedMode = info["fullscreen"] != "true"
		outputDir = strings.TrimSpace(info["output_dir"])
	}

	width, height, fullscreen := 0, 0, true
	if windowedMode {
		width, height, fullscreen = 1024, 768, false
	}

	exp := NewExperiment(name, width, height, fullscreen, bg, fg, fontSize)
	exp.SubjectID = subjectID
	exp.ScreenNumber = screenNumber
	// The flags now only choose what Initialize asks for; the request itself is
	// made there, so both constructors take the same path.
	if *noRealtime {
		exp.RealTimePriority = 0
	} else {
		exp.RealTimePriority = *realtimePrio
	}
	if outputDir != "" {
		exp.SetOutputDirectory(outputDir)
	}
	if info != nil {
		exp.Info = info
	}
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	// _ = exp.ShowSplash(true)
	return exp
}

// Show presents a visual stimulus on the experiment screen, clearing it first
// and flipping the backbuffer afterwards. It is a convenience wrapper around
// ShowTS for the common case where the onset timestamp is not needed:
//
//	exp.Show(stim)   // clear + draw + flip, no timing
//
// If the user requests to exit during presentation, this method will panic
// with an internal sentinel to abort the experiment loop gracefully.
func (e *Experiment) Show(v stimuli.VisualStimulus) error {
	_, err := e.ShowTS(v)
	return err
}

// ShowTS presents a visual stimulus (clear + draw + flip) and returns the
// SDL3 nanosecond timestamp captured immediately after the flip.
//
// The flip holds to the frame boundary (see Screen.Update), so consecutive
// ShowTS calls occupy exactly one display frame each even on drivers where
// SDL_RenderPresent does not block.
//
// The timestamp is on the same clock as SDL3 event timestamps, so the
// reaction time from this stimulus onset is simply:
//
//	onset, _ := exp.ShowTS(stim)
//	key, keyTS, _ := exp.Keyboard.GetKeyEventTS(keys, -1)
//	rtNS := int64(keyTS - onset)
func (e *Experiment) ShowTS(v stimuli.VisualStimulus) (uint64, error) {
	if err := v.Present(e.Screen, true, false); err != nil {
		if IsEndLoop(err) {
			panic(exitPanic{err: err})
		}
		return 0, fmt.Errorf("control.Experiment.ShowTS: presenting stimulus: %w", err)
	}
	ts, err := e.Screen.FlipTS()
	if err != nil {
		return 0, fmt.Errorf("control.Experiment.ShowTS: flipping display: %w", err)
	}
	return ts, nil
}

// ShowFrames presents a visual stimulus and holds it for exactly n display
// frames, returning the SDL3 nanosecond timestamp of the first flip — the
// stimulus onset.
//
// The stimulus is redrawn before every flip. That is not an optimisation
// detail but a requirement: SDL exposes no way to wait for a retrace without
// presenting, and a frame carrying no draw calls is not reliably scanned out
// under a compositor (see apparatus.Screen.fillWholeTarget). Redrawing also
// keeps the renderer's draw color consistent, which a "re-clear and flip" hold
// does not.
//
// Use it when the duration must be an exact number of frames:
//
//	onset, _ := exp.ShowFrames(stim, 10)   // 10 frames ≈ 166.5 ms at 60 Hz
//	exp.Screen.Clear()
//	offset, _ := exp.Screen.FlipTS()
//
// For a duration in milliseconds, use ShowTimed instead.
func (e *Experiment) ShowFrames(v stimuli.VisualStimulus, n int) (uint64, error) {
	if n < 1 {
		return 0, fmt.Errorf("control.Experiment.ShowFrames: n must be >= 1, got %d", n)
	}
	var onsetNS uint64
	for i := 0; i < n; i++ {
		ts, err := e.ShowTS(v)
		if err != nil {
			return 0, fmt.Errorf("control.Experiment.ShowFrames: frame %d: %w", i, err)
		}
		if i == 0 {
			onsetNS = ts
		}
	}
	return onsetNS, nil
}

// BlankFrames clears the screen and holds it blank for exactly n display
// frames, returning the SDL3 nanosecond timestamp of the first flip — the
// stimulus offset when it follows a ShowFrames.
//
// It is the frame-locked counterpart of Blank(ms), for inter-stimulus
// intervals that must be an exact number of frames. Every frame goes through
// Screen.Clear, so each one carries a real draw call (see ShowFrames).
func (e *Experiment) BlankFrames(n int) (uint64, error) {
	if n < 1 {
		return 0, fmt.Errorf("control.Experiment.BlankFrames: n must be >= 1, got %d", n)
	}
	var offsetNS uint64
	for i := 0; i < n; i++ {
		if err := e.Screen.Clear(); err != nil {
			return 0, fmt.Errorf("control.Experiment.BlankFrames: frame %d: %w", i, err)
		}
		ts, err := e.Screen.FlipTS()
		if err != nil {
			return 0, fmt.Errorf("control.Experiment.BlankFrames: frame %d: %w", i, err)
		}
		if i == 0 {
			offsetNS = ts
		}
	}
	return offsetNS, nil
}

// WaitAnyEventTS blocks until a matching input event arrives from any device
// and returns an InputEvent carrying the SDL3 hardware nanosecond timestamp.
//
// keys filters keyboard events: pass nil to accept any key.
// catchMouse controls whether mouse button presses are accepted.
// timeoutMS is the maximum wait in milliseconds; pass -1 for no timeout.
//
// On timeout, returns a zero InputEvent and nil error.
// On ESC or window-close, returns sdl.EndLoop.
//
// Because TimestampNS is on the same SDL3 nanosecond clock as ShowTS, reaction
// time is simply:
//
//	onset, _ := exp.ShowTS(stim)
//	ev, _ := exp.WaitAnyEventTS(keys, true, -1)
//	rtNS := int64(ev.TimestampNS - onset)
func (e *Experiment) WaitAnyEventTS(keys []sdl.Keycode, catchMouse bool, timeoutMS int) (apparatus.InputEvent, error) {
	start := sdl.Ticks()
	for {
		if timeoutMS >= 0 {
			if int(sdl.Ticks()-start) >= timeoutMS {
				return apparatus.InputEvent{}, nil
			}
		}

		state := e.PollEvents(nil)
		if state.QuitRequested {
			return apparatus.InputEvent{}, sdl.EndLoop
		}

		if state.LastKey != 0 {
			key := state.LastKey
			if key == sdl.K_ESCAPE {
				return apparatus.InputEvent{
					Device:      apparatus.DeviceKeyboard,
					Key:         sdl.K_ESCAPE,
					TimestampNS: state.LastKeyTimestamp,
				}, sdl.EndLoop
			}
			matched := keys == nil
			if !matched {
				for _, kc := range keys {
					if key == kc {
						matched = true
						break
					}
				}
			}
			if matched {
				return apparatus.InputEvent{
					Device:      apparatus.DeviceKeyboard,
					Key:         key,
					TimestampNS: state.LastKeyTimestamp,
				}, nil
			}
		}

		if catchMouse && state.LastMouseButton != 0 {
			return apparatus.InputEvent{
				Device:      apparatus.DeviceMouse,
				Button:      state.LastMouseButton,
				TimestampNS: state.LastMouseTimestamp,
			}, nil
		}

		time.Sleep(1 * time.Millisecond)
	}
}

// SetGamma installs a uniform inverse-gamma corrector on the experiment.
// Call once after Initialize(), before the trial loop.
// A gamma of 2.2 is typical for sRGB monitors.
//
// After calling SetGamma, pass all stimulus colors through CorrectColor so
// that linear luminance values are mapped to the physical digital values
// required by the monitor.
func (e *Experiment) SetGamma(gamma float64) {
	e.GammaCorrector = apparatus.NewGammaCorrectorUniform(gamma)
}

// CorrectColor applies the experiment's GammaCorrector (if set) to c and
// returns the corrected color. When GammaCorrector is nil (the default),
// c is returned unchanged, so callers can always call CorrectColor without
// first checking whether gamma correction is enabled.
func (e *Experiment) CorrectColor(c sdl.Color) sdl.Color {
	if e.GammaCorrector == nil {
		return c
	}
	return e.GammaCorrector.CorrectColor(c)
}

// ShowTimed presents a visual stimulus (clear + draw + flip) and then waits
// for the given duration. It replaces the common two-line pattern:
//
//	exp.Show(stim)
//	exp.Wait(durationMs)
func (e *Experiment) ShowTimed(v stimuli.VisualStimulus, durationMs int) error {
	if err := e.Show(v); err != nil {
		return fmt.Errorf("control.Experiment.ShowTimed: %w", err)
	}
	return e.Wait(durationMs)
}

// ShowAndGetRT presents a visual stimulus with hardware-precise onset timing,
// clears stale keyboard events, then waits for one of the given keys.
// It returns the keycode and the reaction time in milliseconds computed from
// the VSYNC flip timestamp.
//
// It replaces the common three-line pattern:
//
//	onset, _ := exp.ShowTS(stim)
//	key, eventTS, _ := exp.Keyboard.GetKeyEventTS(keys, timeoutMs)
//	rt := int64(eventTS-onset) / 1_000_000
//
// Pass timeoutMs = -1 for no timeout. On timeout, returns (0, 0, nil).
// On ESC or window-close, returns sdl.EndLoop.
func (e *Experiment) ShowAndGetRT(v stimuli.VisualStimulus, keys []Keycode, timeoutMs int) (Keycode, int64, error) {
	e.Keyboard.Clear()
	onsetNS, err := e.ShowTS(v)
	if err != nil {
		return 0, 0, fmt.Errorf("control.Experiment.ShowAndGetRT: %w", err)
	}
	key, eventTS, err := e.Keyboard.GetKeyEventTS(keys, timeoutMs)
	if err != nil {
		return key, 0, fmt.Errorf("control.Experiment.ShowAndGetRT: waiting for key: %w", err)
	}
	if key == 0 { // timeout
		return 0, 0, nil
	}
	return key, int64(eventTS-onsetNS) / 1_000_000, nil
}

// ShowEndMessage displays a centered completion message and waits for any key.
// It replaces the common end-of-experiment pattern:
//
//	box := stimuli.NewTextBox(message, width, control.Origin(), exp.ForegroundColor)
//	exp.Show(box)
//	exp.Keyboard.Wait()
func (e *Experiment) ShowEndMessage(message string) error {
	w := int32(float32(e.Screen.Width) * 0.80)
	if w < 400 {
		w = 400
	}
	tb := stimuli.NewTextBox(message, w, sdl.FPoint{}, e.ForegroundColor)
	if err := e.Show(tb); err != nil {
		return fmt.Errorf("control.Experiment.ShowEndMessage: %w", err)
	}
	_, err := e.Keyboard.Wait()
	return err
}

// ShowInstructions displays a centered text block and waits for the
// participant to press the spacebar before returning. This replaces the
// common three-line pattern:
//
//	tb := stimuli.NewTextBox(text, width, control.Origin(), exp.ForegroundColor)
//	exp.Show(tb)
//	exp.Keyboard.WaitKey(control.K_SPACE)
//
// The wrap width defaults to 80 % of the screen width (minimum 400 px).
func (e *Experiment) ShowInstructions(text string) error {
	w := int32(float32(e.Screen.Width) * 0.80)
	if w < 400 {
		w = 400
	}
	tb := stimuli.NewTextBox(text, w, sdl.FPoint{}, e.ForegroundColor)
	if err := e.Show(tb); err != nil {
		return fmt.Errorf("control.Experiment.ShowInstructions: %w", err)
	}
	return e.Keyboard.WaitKey(K_SPACE)
}

// Blank clears the screen and keeps it blank for the given number of
// milliseconds. It replaces the common three-line pattern:
//
//	exp.Screen.Clear()
//	exp.Screen.Update()
//	exp.Wait(ms)
func (e *Experiment) Blank(ms int) error {
	if err := e.Screen.ClearAndUpdate(); err != nil {
		return fmt.Errorf("control.Experiment.Blank: %w", err)
	}
	return e.Wait(ms)
}

// pumpFrame pumps OS events for one frame: keeps the window responsive on all
// platforms (including Wayland), updates input-device state (mouse position,
// keyboard state), and checks for quit/ESC — all without dequeuing events, so
// keyboard/mouse events remain in the SDL queue for GetKeyEventTS to consume
// with their original hardware timestamps.
//
// Returns sdl.EndLoop if quit or ESC was detected, nil otherwise.
func (e *Experiment) pumpFrame() error {
	// Honor a quit requested by the signal handler before issuing any SDL
	// calls, so a pending shutdown is detected even while SDL is mid-teardown
	// (and so this is checkable without a live SDL context).
	if e.quitFlag.Load() != 0 {
		return sdl.EndLoop
	}
	pumpEvents()
	if hasEvent(sdl.EVENT_QUIT) {
		return sdl.EndLoop
	}
	kbState := getKeyboardState()
	if len(kbState) > int(sdl.SCANCODE_ESCAPE) && kbState[sdl.SCANCODE_ESCAPE] {
		return sdl.EndLoop
	}
	return nil
}

// SDL seams for pumpFrame, swappable in unit tests that run Experiment.Run
// without a live SDL context (same pattern as pollEvent below).
var (
	pumpEvents       = sdl.PumpEvents
	hasEvent         = sdl.HasEvent
	getKeyboardState = sdl.GetKeyboardState
)

// SDL seams for End, so unit tests can exercise its panic backstop without a
// loaded SDL library (same pattern as the pump seams above).
var (
	ttfQuit = ttf.Quit
	sdlQuit = sdl.Quit
)

// Wait blocks for the given number of milliseconds while keeping the OS
// responsive by pumping SDL events each frame (see pumpFrame). If a quit
// request, quit event, or ESC key is detected during the wait, it panics with
// an internal sentinel so the run loop exits gracefully.
func (e *Experiment) Wait(ms int) error {
	start := getTicks()
	for {
		if elapsed := int(getTicks() - start); elapsed >= ms {
			return nil
		}
		if err := e.pumpFrame(); err != nil {
			panic(exitPanic{err: err})
		}
		clock.Wait(1)
	}
}

// SetOutputDirectory overrides the default folder used to store .csv result
// files. If not called, Initialize will use the user's home directory
// with the folder name defined by results.DataFileDirectory (default "goxpy_data").
func (e *Experiment) SetOutputDirectory(dir string) {
	e.OutputDirectory = dir
}

// Initialize loads the embedded SDL/TTF binaries, initializes SDL (video,
// events and audio), opens the default playback audio device, creates the
// main window/renderer (`apparatus.Screen`), and creates the default `DataFile`.
//
// It must be called exactly once before using the experiment, and `End`
// should be deferred immediately after successful initialization.
func (e *Experiment) Initialize() error {
	// Ask for real-time scheduling before anything timing-critical starts.
	//
	// Attempted by default rather than on request. There is no cost to trying:
	// if the privilege was never granted this fails and the run continues at
	// normal priority, exactly as it would have. Making it opt-in would mean the
	// common case -- launching by clicking the icon, where no chrt prefix is
	// possible -- silently gets the worse timing, which is the case that most
	// needs the help.
	//
	// Failure is reported and never fatal. An experiment that refused to run
	// because it could not get real-time priority would be worse than one that
	// runs slightly less precisely and says so. What it must not do is fail
	// silently: on a loaded host this is worth milliseconds, and the run's own
	// system report records what it ended up with (sysinfo.SchedulingInfo).
	//
	// It is here, and not in NewExperimentFromFlags where it used to live,
	// because every program reaches Initialize while only some are built from
	// flags. RealTimePriority carries the decision so the flag path keeps its
	// -no-realtime escape hatch and the plain path stops being a second, quieter
	// policy. On non-Linux this always logs a failure: the elevation is
	// deliberately Linux-only (sysinfo/realtime_other.go), because Windows
	// priority classes and Darwin thread policies are not the same guarantee.
	//
	// This runs on the main goroutine, which init() has locked to its OS thread,
	// so the elevation lands on the thread the experiment loop will use.
	if e.RealTimePriority > 0 {
		if err := sysinfo.RaiseToRealTime(e.RealTimePriority); err != nil {
			log.Printf("real-time scheduling not obtained, continuing at normal priority: %v", err)
		}
	}

	// Reuse loaders cached by GetParticipantInfo (if it was called first) to
	// avoid loading a second copy of the SDL dylib on macOS, which triggers
	// duplicate Objective-C class registrations and a silent crash.
	cachedSDL, cachedTTF := consumeSharedLoaders()
	if cachedSDL != nil {
		e.sdlLoader = cachedSDL
	} else {
		e.sdlLoader = loadSDL()
	}
	// imgLoader has no shared counterpart: GetParticipantInfo does not use images.
	e.imgLoader = loadIMG()
	if cachedTTF != nil {
		e.ttfLoader = cachedTTF
	} else {
		e.ttfLoader = loadTTF()
	}

	if err := sdl.Init(platformSDLInitFlags()); err != nil {
		return fmt.Errorf("sdl.Init: %w", err)
	}

	if err := ttf.Init(); err != nil {
		return fmt.Errorf("ttf.Init: %w", err)
	}

	// If no explicit window size was provided, we use the autodetect mode (0,0)
	// which apparatus.NewScreen handles by using native resolution and high pixel density.
	if e.WindowWidth == 0 && e.WindowHeight == 0 {
		e.Fullscreen = true
	}

	// Apply audio buffer hint before opening the audio device.
	if pendingAudioSampleFrames > 0 {
		sdl.SetHint(sdl.HINT_AUDIO_DEVICE_SAMPLE_FRAMES, fmt.Sprintf("%d", pendingAudioSampleFrames))
	}

	if err := e.platformInitAudio(); err != nil {
		return fmt.Errorf("platformInitAudio: %w", err)
	}

	screen, err := apparatus.NewScreen(e.Name, e.WindowWidth, e.WindowHeight, e.BackgroundColor, e.Fullscreen, e.ScreenNumber)
	if err != nil {
		return fmt.Errorf("NewScreen: %w", err)
	}
	e.Screen = screen
	e.Keyboard = &apparatus.Keyboard{
		PollKeys: func() (sdl.Keycode, bool) {
			state := e.PollEvents(nil)
			return state.LastKey, state.QuitRequested
		},
		PollKeysWithTS: func() (sdl.Keycode, uint64, bool) {
			state := e.PollEvents(nil)
			return state.LastKey, state.LastKeyTimestamp, state.QuitRequested
		},
		PollKeyUps: func() (sdl.Keycode, uint64, bool) {
			state := e.PollEvents(nil)
			return state.LastKeyUp, state.LastKeyUpTimestamp, state.QuitRequested
		},
	}
	// Hide the pointer by default: an experiment window is a stimulus surface,
	// and a stray cursor sitting on it is a distractor the experimenter did not
	// put there. apparatus.NewScreen deliberately shows the cursor (it also has
	// to install a cursor *shape*, which SDL does not supply on the KMS/DRM
	// backend), so this must come after the screen exists.
	//
	// Mouse-driven experiments opt back in with exp.ShowCursor() right after
	// Initialize. GetParticipantInfo is unaffected — it shows the cursor for the
	// lifetime of its own dialog window, whether it runs before or after this.
	if !e.CursorVisible {
		if err := e.HideCursor(); err != nil {
			// Non-fatal: a visible cursor is cosmetic, not a reason to abort a
			// session that is otherwise ready to run.
			log.Printf("Warning: could not hide the mouse cursor: %v", err)
		}
	}

	e.Mouse = &apparatus.Mouse{
		PollButtons: func() (uint32, bool) {
			state := e.PollEvents(nil)
			return state.LastMouseButton, state.QuitRequested
		},
		PollButtonsWithTS: func() (uint32, uint64, bool) {
			state := e.PollEvents(nil)
			return state.LastMouseButton, state.LastMouseTimestamp, state.QuitRequested
		},
		PollButtonUps: func() (uint32, uint64, bool) {
			state := e.PollEvents(nil)
			return state.LastMouseButtonUp, state.LastMouseButtonUpTimestamp, state.QuitRequested
		},
	}

	// Load default font if not already set
	if e.DefaultFont == nil {
		size := e.DefaultFontSize
		if size <= 0 {
			size = 32 // sensible library default
		}
		if err := e.LoadFontFromMemory(assets_embed.InconsolataFont, size); err != nil {
			// Non-fatal error, just warn
			log.Printf("Warning: failed to load default embedded font: %v", err)
		}
	}

	// Initialize DataFile
	outDir := e.OutputDirectory
	dataFile, err := results.NewDataFile(outDir, e.SubjectID, e.Name)
	if err != nil {
		return fmt.Errorf("control.Experiment: creating data file: %w", err)
	}
	e.Data = dataFile

	// Capture system metadata automatically so every data file has a complete
	// record of SDL, renderer, display, and audio configuration.
	sysInfo := e.Screen.GatherSystemInfo()

	// Measure the display before the experiment starts, and record it beside
	// the nominal rate. The two disagreeing is the single most useful signal
	// that a session's timing is not what the analysis will assume: below
	// nominal means the driver is not blocking on VSYNC, above means frames
	// are being dropped on the way to the panel (a compositor throttling an
	// unfocused window, typically). Costs calibrationFrames refresh periods,
	// on a screen that is still blank.
	if measured, err := e.Screen.CalibrateRefresh(calibrationFrames); err == nil && measured > 0 {
		sysInfo.MeasuredHz = float64(time.Second) / float64(measured)
		if sysInfo.NominalHz > 0 {
			if ratio := sysInfo.MeasuredHz / sysInfo.NominalHz; ratio < 0.9 || ratio > 1.1 {
				log.Printf("WARNING: measured refresh %.2f Hz differs from nominal %.2f Hz — "+
					"stimulus durations may not be what you asked for",
					sysInfo.MeasuredHz, sysInfo.NominalHz)
			}
		}
	}

	sysInfo.AudioDriver = sdl.GetCurrentAudioDriver()
	if e.AudioDevice != 0 {
		// WHICH device, not just which driver. The default playback device
		// follows the desktop's current selection, so two runs an hour apart can
		// go to different hardware with nothing in the data to say so — and the
		// difference is worth tens of milliseconds of audio latency. Diagnosed
		// 2026-08-17 on a machine whose USB interface was connected while the
		// desktop was still routing to the motherboard codec. Name() resolves a
		// logical device to the physical one behind it.
		if name, err := e.AudioDevice.Name(); err == nil {
			sysInfo.AudioDevice = name
		}
		if spec, frames, err := e.AudioDevice.Format(); err == nil && spec != nil {
			sysInfo.AudioFreq = spec.Freq
			sysInfo.AudioChannels = spec.Channels
			sysInfo.AudioFrames = frames
			sysInfo.AudioFormat = spec.Format.Name()
		}
	}
	e.Data.WriteSystemInfo(sysInfo)
	e.Data.WriteDisplayInfo(e.Screen.DisplayInfo())

	if len(e.Info) > 0 {
		e.Data.WriteParticipantInfo(e.Info)
	}

	// Handle Ctrl-C and SIGTERM: set quit flag so pumpFrame exits Run() cleanly
	// on the main goroutine. Never call End()/sdl.Quit() from here — concurrent
	// CGo calls from two OS threads cause SIGSEGV. Save data and fall back to
	// os.Exit after a timeout in case the main goroutine is blocked outside Run()
	// (e.g. apparatus.Keyboard.Wait).
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		e.quitFlag.Store(1)
		e.finalizeData()
		// Give pumpFrame up to 3 s to detect the flag so defer End() can run
		// from the main goroutine. If the main goroutine is blocked outside Run(),
		// force-exit as a last resort.
		time.Sleep(3 * time.Second)
		os.Exit(0)
	}()

	return nil
}

// pollEvent and getTicks are test seams: indirection over sdl.PollEvent and
// sdl.Ticks so that event-loop and timing logic can be exercised without a live
// SDL context (which would crash in a headless CI environment). They are
// reassigned only by experiment_test.go, which restores them via defer. Do not
// mutate them in production code.
var pollEvent = sdl.PollEvent
var getTicks = sdl.Ticks

// ---------------------------------------------------------------------------
// Event handling
// ---------------------------------------------------------------------------

// PollEvents processes all pending SDL events, updates the experiment's
// aggregate `EventState`, and optionally forwards each SDL event to the
// provided handler callback.
//
// The handler can return true to stop processing further events for this
// polling cycle. The returned `EventState` summarizes the last keyboard and
// mouse button pressed and whether a quit/escape was requested.
func (e *Experiment) PollEvents(handle func(ev sdl.Event) bool) EventState {
	// Reset transient state for this polling cycle.
	// QuitRequested is intentionally sticky: once ESC or window-close is
	// received, it stays true so the experiment can unwind gracefully.
	e.event.LastKey = 0
	e.event.LastKeyTimestamp = 0
	e.event.LastMouseButton = 0
	e.event.LastMouseTimestamp = 0
	e.event.LastKeyUp = 0
	e.event.LastKeyUpTimestamp = 0
	e.event.LastMouseButtonUp = 0
	e.event.LastMouseButtonUpTimestamp = 0

	var ev sdl.Event
	for pollEvent(&ev) {
		switch ev.Type {
		case sdl.EVENT_QUIT:
			e.event.QuitRequested = true
		case sdl.EVENT_KEY_DOWN:
			ke := ev.KeyboardEvent()
			if ke.Key == sdl.K_ESCAPE {
				e.event.QuitRequested = true
			}
			if e.event.LastKey == 0 {
				e.event.LastKey = ke.Key
				e.event.LastKeyTimestamp = ke.Timestamp
			}
		case sdl.EVENT_KEY_UP:
			ke := ev.KeyboardEvent()
			if e.event.LastKeyUp == 0 {
				e.event.LastKeyUp = ke.Key
				e.event.LastKeyUpTimestamp = ke.Timestamp
			}
		case sdl.EVENT_MOUSE_BUTTON_DOWN:
			if e.event.LastMouseButton == 0 {
				me := ev.MouseButtonEvent()
				e.event.LastMouseButton = uint32(me.Button)
				e.event.LastMouseTimestamp = me.Timestamp
			}
		case sdl.EVENT_MOUSE_BUTTON_UP:
			if e.event.LastMouseButtonUp == 0 {
				me := ev.MouseButtonEvent()
				e.event.LastMouseButtonUp = uint32(me.Button)
				e.event.LastMouseButtonUpTimestamp = me.Timestamp
			}
		}

		if handle != nil {
			if stop := handle(ev); stop {
				break
			}
		}
	}

	return e.event
}

// HandleEvents is a convenience wrapper around PollEvents.
// It processes pending SDL events and returns:
//   - the first key pressed since the last call (0 if none),
//   - the first mouse button pressed (0 if none),
//   - sdl.EndLoop if a quit or ESC key was detected.
//
// This mirrors the higher‑level event interface of the original Expyriment.
func (e *Experiment) HandleEvents() (sdl.Keycode, uint32, error) {
	state := e.PollEvents(nil)
	if state.QuitRequested {
		return 0, 0, sdl.EndLoop
	}
	// Note: HandleEvents from logic thread will return the sticky key,
	// but it won't clear it. Users should prefer Keyboard.Wait() or similar.
	return state.LastKey, state.LastMouseButton, nil
}

// ---------------------------------------------------------------------------
// Design delegation — thin wrappers forwarding to design.Experiment
// ---------------------------------------------------------------------------

// AddDataVariableNames registers column names for the experiment data file.
// It updates both the design metadata and the live DataFile (if already open).
func (e *Experiment) AddDataVariableNames(names []string) {
	e.Design.AddDataVariableNames(names)
	if e.Data != nil {
		e.Data.AddVariableNames(names)
	}
}

// AddBlock appends a trial block to the experiment design, optionally
// duplicating it `copies` times (useful for repeated-measures designs).
func (e *Experiment) AddBlock(b *design.Block, copies int) {
	e.Design.AddBlock(b, copies)
}

// AddExperimentInfo attaches free-form metadata (e.g. lab name, version)
// to the experiment design for inclusion in the data file header.
func (e *Experiment) AddExperimentInfo(text string) {
	e.Design.AddExperimentInfo(text)
}

// ShuffleBlocks randomizes the presentation order of blocks.
func (e *Experiment) ShuffleBlocks() {
	e.Design.ShuffleBlocks()
}

// AddBWSFactor registers a between-subjects factor with the given condition levels.
// Use GetPermutedBWSFactorCondition to retrieve the condition assigned to the
// current subject (determined by SubjectID via Latin-square permutation).
func (e *Experiment) AddBWSFactor(name string, conditions []interface{}) {
	e.Design.AddBWSFactor(name, conditions)
}

// GetPermutedBWSFactorCondition returns the condition assigned to the current
// subject for the named between-subjects factor, using the SubjectID to index
// into a Latin-square permutation of conditions.
func (e *Experiment) GetPermutedBWSFactorCondition(name string) interface{} {
	return e.Design.GetPermutedBWSFactorCondition(name, e.SubjectID)
}

// Summary returns a human-readable summary of the experiment design,
// including block structure, trial counts, and factor definitions.
func (e *Experiment) Summary() string {
	return e.Design.Summary()
}

// ---------------------------------------------------------------------------
// Screen / rendering delegation
// ---------------------------------------------------------------------------

// SetVSync toggles vertical synchronization on the screen.
// 1 to enable, 0 to disable.
func (e *Experiment) SetVSync(vsync int) error {
	if e.Screen == nil {
		return nil
	}
	return e.Do(func() error {
		return e.Screen.SetVSync(vsync)
	})
}

// SetLogicalSize sets a device-independent resolution for the experiment.
func (e *Experiment) SetLogicalSize(width, height int32) error {
	if e.Screen == nil {
		return nil
	}
	return e.Do(func() error {
		return e.Screen.SetLogicalSize(width, height)
	})
}

// Flip presents the backbuffer to the display using the experiment's screen.
// When VSync is enabled, this will typically block until the next vertical retrace.
func (e *Experiment) Flip() error {
	if e.Screen == nil {
		return nil
	}
	return e.Do(func() error {
		return e.Screen.Flip()
	})
}

// ---------------------------------------------------------------------------
// Font management
// ---------------------------------------------------------------------------

// LoadFont loads a TTF font from the specified path and sets it as the default for the experiment.
func (e *Experiment) LoadFont(path string, size float32) error {
	font, err := ttf.OpenFont(path, size)
	if err != nil {
		return fmt.Errorf("control.Experiment.LoadFont: %w", err)
	}
	e.DefaultFont = font
	if e.Screen != nil {
		e.Screen.DefaultFont = font
	}
	return nil
}

// LoadFontFromMemory loads a TTF font from a byte slice and sets it as the default.
func (e *Experiment) LoadFontFromMemory(data []byte, size float32) error {
	ioStream, err := sdl.IOFromBytes(data)
	if err != nil {
		return fmt.Errorf("control.Experiment.LoadFontFromMemory: %w", err)
	}
	// Note: OpenFontIO with closeio=true will close the IOStream
	font, err := ttf.OpenFontIO(ioStream, true, size)
	if err != nil {
		return fmt.Errorf("control.Experiment.LoadFontFromMemory: opening font: %w", err)
	}
	e.DefaultFont = font
	if e.Screen != nil {
		e.Screen.DefaultFont = font
	}
	return nil
}

// ShowSplash displays a brief splash screen with the experiment name in the
// default font and "Goxpyriment <version>" in a smaller font below.
// When waitForKey is true, the screen stays up until any key is pressed.
// When waitForKey is false, it dismisses automatically after 5 seconds (or on
// any key). Non-fatal: errors during splash rendering are silently ignored so
// the experiment can continue.
func (e *Experiment) ShowSplash(waitForKey bool) error {
	if e.Screen == nil || e.DefaultFont == nil {
		return nil
	}
	smallSize := e.DefaultFontSize * 0.55
	if smallSize < 10 {
		smallSize = 10
	}
	ioStream, err := sdl.IOFromBytes(assets_embed.InconsolataFont)
	if err != nil {
		return nil
	}
	smallFont, err := ttf.OpenFontIO(ioStream, true, smallSize)
	if err != nil {
		return nil
	}
	defer smallFont.Close()
	subtitle := "Goxpyriment " + results.Version
	timeoutSec := 5.0
	if waitForKey {
		timeoutSec = 0
	}
	return stimuli.TwoLineSplash(e.Screen, assets_embed.IconPNG, e.DefaultFont, e.Name, smallFont, subtitle, timeoutSec, true)
}

// ---------------------------------------------------------------------------
// Lifecycle — cleanup and run loop
// ---------------------------------------------------------------------------

// End cleans up resources.
// OpenMicrophone opens an audio recording device and stores it in exp.Microphone.
// Pass nil for spec to use apparatus.DefaultRecordingSpec (F32LE mono 44100 Hz).
// Returns an error if no recording device is available or the device fails to open.
// The microphone is closed automatically by End().
func (e *Experiment) OpenMicrophone(spec *sdl.AudioSpec) error {
	mic, err := apparatus.NewMicrophone(spec)
	if err != nil {
		return fmt.Errorf("control.Experiment.OpenMicrophone: %w", err)
	}
	if e.Microphone != nil {
		e.Microphone.Close()
	}
	e.Microphone = mic
	return nil
}

// End releases everything the experiment holds — data file, font, screen, audio
// device, microphone, and the SDL/TTF libraries themselves. Defer it right after
// the experiment is created.
//
// End is also the backstop for the ESC/quit sentinel. Wait and ShowTS abort by
// panicking with the internal exitPanic value, which Run recovers; an experiment
// that does not wrap its logic in Run has nothing to catch it, and pressing ESC
// would surface as a crash. Since every experiment defers End, and a deferred
// function may call recover directly, absorbing the sentinel here makes ESC exit
// cleanly with or without Run.
//
// Any other panic is re-raised once the cleanup below has run, so genuine bugs
// still fail loudly. The re-raise costs the original stack trace — Go reports
// the panic as "[recovered]" and prints the frames of the re-panic instead —
// which is the same trade Run's recover has always made.
func (e *Experiment) End() {
	if r := recover(); r != nil {
		if _, ok := r.(exitPanic); !ok && !e.platformHandleCrash(r) {
			// Not our sentinel, and no browser error overlay took charge of it:
			// let the real crash propagate, but only after cleanup has run.
			defer panic(r)
		}
	}
	e.finalizeData()
	if e.DefaultFont != nil {
		e.DefaultFont.Close()
	}
	if e.Screen != nil {
		e.Screen.Destroy()
	}
	if e.Audio != nil {
		e.Audio.Shutdown()
	}
	if e.AudioDevice != 0 {
		e.AudioDevice.Close()
	}
	if e.Microphone != nil {
		e.Microphone.Close()
	}
	ttfQuit()
	sdlQuit()
	if e.ttfLoader != nil {
		e.ttfLoader.Unload()
	}
	if e.imgLoader != nil {
		e.imgLoader.Unload()
	}
	if e.sdlLoader != nil {
		e.sdlLoader.Unload()
	}
}

// Fatal cleans up SDL resources, then logs the formatted message and exits.
// Use this instead of log.Fatalf anywhere after exp.Initialize() succeeds, to
// ensure sdl.Quit() is called before the process exits.
func (e *Experiment) Fatal(format string, args ...any) {
	e.End()
	log.Fatalf(format, args...)
}

// Run executes the main experiment logic inside SDL's run loop.
//
// Threading contract: SDL3 video, rendering, and event calls must all happen on
// a single OS thread (on macOS, the main thread). The logic callback therefore
// runs synchronously on the goroutine that calls Run — there is no goroutine
// dispatch. All SDL/stimulus/event work (exp.Screen.*, stim.Draw, exp.Show,
// keyboard/mouse polling, …) must stay on this goroutine; never issue it from a
// goroutine spawned by experiment code, or SDL will crash or corrupt state.
//
// The required thread pinning is provided by the go-sdl3 sdl package, whose
// init() calls runtime.LockOSThread() on the main goroutine at startup. We
// reassert it here (idempotent) so the invariant is visible and self-documented
// in this codebase rather than relying solely on a dependency's init side effect.
//
// This preserves compatibility with every example that calls exp.Screen.Clear(),
// stim.Draw(), and exp.Screen.Update() directly. To compose rendering steps
// before presenting, use exp.Screen methods directly; for the common
// clear+draw+flip case use exp.Show / exp.Blank.
//
// If the callback (or any Experiment method called from it) panics with an
// internal sentinel, Run will recover and return the original error.
func (e *Experiment) Run(logic func() error) error {
	// Pin this goroutine to its OS thread for the lifetime of the run loop so
	// every SDL call is issued from the same thread. See the threading contract
	// above. Idempotent with go-sdl3's own init-time LockOSThread.
	runtime.LockOSThread()
	return sdl.RunLoop(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				if p, ok := r.(exitPanic); ok {
					err = p.err
				} else if e.platformHandleCrash(r) {
					// Browser only: an error overlay was shown and e.crashed set,
					// so swallow the panic and let the deferred End() run without
					// downloading partial data. On desktop this returns false and
					// the panic re-propagates with its stack trace, as before.
					err = fmt.Errorf("control.Experiment.Run: experiment crashed: %v", r)
				} else {
					panic(r)
				}
			}
		}()
		// Pump OS events every frame so the window stays responsive (Wayland,
		// X11, macOS) and input state is current, without draining the queue.
		if err := e.pumpFrame(); err != nil {
			return fmt.Errorf("control.Experiment.Run: %w", err)
		}
		return logic()
	})
}

// HideCursor hides the mouse cursor. Initialize() already does this, so it is
// only needed to hide the cursor again after a ShowCursor call.
func (e *Experiment) HideCursor() error {
	if err := sdl.HideCursor(); err != nil {
		return fmt.Errorf("control.Experiment.HideCursor: %w", err)
	}
	e.CursorVisible = false
	return nil
}

// ShowCursor makes the mouse cursor visible. Call it after Initialize() in
// mouse-driven paradigms — Initialize() hides the cursor by default.
func (e *Experiment) ShowCursor() error {
	if err := sdl.ShowCursor(); err != nil {
		return fmt.Errorf("control.Experiment.ShowCursor: %w", err)
	}
	e.CursorVisible = true
	return nil
}
