// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package control

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/results"
	"github.com/chrplr/goxpyriment/stimuli"
)

// EventState provides a convenient summary of the last processed input events.
// It is updated by Experiment.PollEvents.
type EventState struct {
	LastKey              sdl.Keycode
	LastMouseButton      uint32
	LastKeyTimestamp     uint64 // SDL3 event timestamp in nanoseconds (same clock as TicksNS)
	LastMouseTimestamp   uint64 // SDL3 event timestamp in nanoseconds
	LastKeyUp                sdl.Keycode
	LastKeyUpTimestamp       uint64 // SDL3 event timestamp in nanoseconds for KEY_UP
	LastMouseButtonUp        uint32
	LastMouseButtonUpTimestamp uint64 // SDL3 event timestamp in nanoseconds for MOUSE_BUTTON_UP
	QuitRequested            bool
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
	Info            map[string]string
	// GammaCorrector, when non-nil, is applied by CorrectColor.
	// Set via SetGamma or by assigning io.NewGammaCorrector(...) directly.
	GammaCorrector  *apparatus.GammaCorrector

	sdlLoader interface{ Unload() }
	imgLoader interface{ Unload() }
	ttfLoader interface{ Unload() }

	event    EventState
	quitFlag atomic.Int32 // set to 1 by signal handler goroutine; checked by pumpFrame
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
	}
}

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
// The experiment is fully initialized (SDL, audio, font, data file) before
// being returned. If initialization fails the program exits via log.Fatal.
// The caller should defer exp.End() immediately after this call.
func NewExperimentFromFlags(name string, bg, fg sdl.Color, fontSize float32) *Experiment {
	windowed := flag.Bool("w", false, "Windowed mode (1024×768 window instead of fullscreen)")
	display := flag.Int("d", -1, "Display ID: monitor index where the window/fullscreen will open (-1 = primary)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	width, height, fullscreen := 0, 0, true
	if *windowed {
		width, height, fullscreen = 1024, 768, false
	}

	exp := NewExperiment(name, width, height, fullscreen, bg, fg, fontSize)
	exp.SubjectID = *subject
	if *display >= 0 {
		exp.ScreenNumber = *display
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
// SDL3 nanosecond timestamp captured immediately after the VSYNC flip.
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
		return 0, err
	}
	ts, err := e.Screen.FlipTS()
	if err != nil {
		return 0, err
	}
	return ts, nil
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
		return err
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
		return 0, 0, err
	}
	key, eventTS, err := e.Keyboard.GetKeyEventTS(keys, timeoutMs)
	if err != nil {
		return key, 0, err
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
		return err
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
	e.Show(tb)
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
		return err
	}
	return e.Wait(ms)
}

// Wait blocks for the given number of milliseconds while keeping the OS
// responsive by pumping SDL events. If a quit event or ESC key is detected
// during the wait, it panics with an internal sentinel to exit gracefully.
// pumpFrame pumps OS events for one frame: keeps the window responsive on all
// platforms (including Wayland), updates input-device state (mouse position,
// keyboard state), and checks for quit/ESC — all without dequeuing events, so
// keyboard/mouse events remain in the SDL queue for GetKeyEventTS to consume
// with their original hardware timestamps.
//
// Returns sdl.EndLoop if quit or ESC was detected, nil otherwise.
func (e *Experiment) pumpFrame() error {
	sdl.PumpEvents()
	if sdl.HasEvent(sdl.EVENT_QUIT) {
		return sdl.EndLoop
	}
	kbState := sdl.GetKeyboardState()
	if len(kbState) > int(sdl.SCANCODE_ESCAPE) && kbState[sdl.SCANCODE_ESCAPE] {
		return sdl.EndLoop
	}
	if e.quitFlag.Load() != 0 {
		return sdl.EndLoop
	}
	return nil
}

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

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS | sdl.INIT_AUDIO | sdl.INIT_JOYSTICK | sdl.INIT_GAMEPAD); err != nil {
		return err
	}

	if err := ttf.Init(); err != nil {
		return err
	}

	// If no explicit window size was provided, we use the autodetect mode (0,0)
	// which apparatus.NewScreen handles by using native resolution and high pixel density.
	if e.WindowWidth == 0 && e.WindowHeight == 0 {
		e.Fullscreen = true
	}

	// Initialize Audio
	dev, err := sdl.AUDIO_DEVICE_DEFAULT_PLAYBACK.OpenAudioDevice(nil)
	if err != nil {
		return err
	}
	e.AudioDevice = dev
	e.Audio = &AudioManager{Device: dev}

	screen, err := apparatus.NewScreen(e.Name, e.WindowWidth, e.WindowHeight, e.BackgroundColor, e.Fullscreen, e.ScreenNumber)
	if err != nil {
		return err
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
		return err
	}
	e.Data = dataFile

	// Capture system metadata automatically so every data file has a complete
	// record of SDL, renderer, display, and audio configuration.
	sysInfo := e.Screen.GatherSystemInfo()
	sysInfo.AudioDriver = sdl.GetCurrentAudioDriver()
	if e.AudioDevice != 0 {
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
		if e.Data != nil {
			e.Data.WriteEndTime()
			if err := e.Data.Save(); err == nil {
				log.Printf("Results saved in %s (info: %s)", e.Data.FullPath, e.Data.InfoFile.FullPath)
			}
		}
		// Give pumpFrame up to 3 s to detect the flag so defer End() can run
		// from the main goroutine. If the main goroutine is blocked outside Run(),
		// force-exit as a last resort.
		time.Sleep(3 * time.Second)
		os.Exit(0)
	}()

	return nil
}

// pollEvent is a hook for sdl.PollEvent to allow unit testing without
// a live SDL context.
var pollEvent = sdl.PollEvent

// getTicks is a hook for sdl.Ticks to allow unit testing.
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
		return err
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
		return err
	}
	// Note: OpenFontIO with closeio=true will close the IOStream
	font, err := ttf.OpenFontIO(ioStream, true, size)
	if err != nil {
		return err
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
func (e *Experiment) End() {
	if e.Data != nil {
		e.Data.WriteEndTime()
		if err := e.Data.Save(); err == nil {
			log.Printf("Results saved in %s", e.Data.FullPath)
		}
	}
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
	ttf.Quit()
	sdl.Quit()
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
// The logic callback runs directly on the main (OS) thread so that all SDL
// calls inside it — screen rendering, event polling, etc. — are issued from
// the correct thread. This preserves compatibility with every example that
// calls exp.Screen.Clear(), stim.Draw(), and exp.Screen.Update() directly.
//
// For code that wants to compose rendering steps before presenting, use
// exp.Screen methods directly (they're on the main thread here).
// For code that wants automatic thread dispatch, use exp.Show / exp.Blank.
//
// If the callback (or any Experiment method called from it) panics with an
// internal sentinel, Run will recover and return the original error.
func (e *Experiment) Run(logic func() error) error {
	return sdl.RunLoop(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				if p, ok := r.(exitPanic); ok {
					err = p.err
				} else {
					panic(r)
				}
			}
		}()
		// Pump OS events every frame so the window stays responsive (Wayland,
		// X11, macOS) and input state is current, without draining the queue.
		if err := e.pumpFrame(); err != nil {
			return err
		}
		return logic()
	})
}

// HideCursor hides the mouse cursor. Call this after Initialize() to prevent
// the cursor from appearing over stimuli during the experiment.
func (e *Experiment) HideCursor() error {
	return sdl.HideCursor()
}

// ShowCursor makes the mouse cursor visible again.
func (e *Experiment) ShowCursor() error {
	return sdl.ShowCursor()
}
