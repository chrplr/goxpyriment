// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package control

import (
	"flag"
	"log"

	"github.com/Zyko0/go-sdl3/bin/binimg"
	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/bin/binttf"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/io"
	"github.com/chrplr/goxpyriment/stimuli"
)

// EventState provides a convenient summary of the last processed input events.
// It is updated by Experiment.PollEvents.
type EventState struct {
	LastKey         sdl.Keycode
	LastMouseButton uint32
	QuitRequested   bool
}

// ---------------------------------------------------------------------------
// Experiment — facade that ties together the subsystems of a running experiment
// ---------------------------------------------------------------------------

// Experiment manages the global state of a behavioral or psychophysics experiment.
// It owns the SDL window/renderer (`Screen`), input devices (`Keyboard`, `Mouse`),
// audio device/manager, and the `DataFile` used for logging responses.
//
// It acts as a **facade**: most of its methods are thin delegations to the
// focused subsystem packages (io.Screen, io.Keyboard, design.Experiment, etc.).
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
	Screen          *io.Screen
	Keyboard        *io.Keyboard
	Mouse           *io.Mouse
	Data            *io.DataFile
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
	OutputDirectory string
	// Info holds the key→value map returned by GetParticipantInfo, if called.
	Info            map[string]string

	sdlLoader interface{ Unload() }
	imgLoader interface{ Unload() }
	ttfLoader interface{ Unload() }

	event EventState
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
//   - `-d`  developer mode: opens a 1024×768 window instead of fullscreen
//   - `-s N` subject ID (default 0)
//
// Any extra flags the caller registered with the flag package before calling
// this function are parsed at the same time, so register experiment-specific
// flags first, then call NewExperimentFromFlags.
//
// The experiment is fully initialized (SDL, audio, font, data file) before
// being returned. If initialization fails the program exits via log.Fatal.
// The caller should defer exp.End() immediately after this call.
func NewExperimentFromFlags(name string, bg, fg sdl.Color, fontSize float32) *Experiment {
	develop := flag.Bool("d", false, "Developer mode (windowed 1024×768)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 768, false
	}

	exp := NewExperiment(name, width, height, fullscreen, bg, fg, fontSize)
	exp.SubjectID = *subject
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	// _ = exp.ShowSplash(true)
	return exp
}

// Show presents a visual stimulus on the experiment screen, clearing it first
// and flipping the backbuffer afterwards. It is equivalent to:
//
//	stim.Present(exp.Screen, true, true)
//
// This is the standard way to display a stimulus in a trial loop.
// If the user requests to exit during presentation, this method will panic
// with an internal sentinel to abort the experiment loop gracefully.
func (e *Experiment) Show(v stimuli.VisualStimulus) error {
	err := e.Do(func() error {
		return v.Present(e.Screen, true, true)
	})
	if IsEndLoop(err) {
		panic(exitPanic{err: err})
	}
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
	err := e.Do(func() error {
		return e.Screen.ClearAndUpdate()
	})
	if err != nil {
		return err
	}
	return e.Wait(ms)
}

// Wait blocks for the given number of milliseconds while keeping the OS
// responsive by pumping SDL events. If a quit event or ESC key is detected
// during the wait, it panics with an internal sentinel to exit gracefully.
func (e *Experiment) Wait(ms int) error {
	start := getTicks()
	for {
		elapsed := int(getTicks() - start)
		if elapsed >= ms {
			return nil
		}

		// Pump events so the OS stays responsive and ESC is detected promptly.
		state := e.PollEvents(nil)
		if state.QuitRequested {
			panic(exitPanic{err: sdl.EndLoop})
		}

		clock.Wait(1)
	}
}

// SetOutputDirectory overrides the default folder used to store .xpd result
// files. If not called, Initialize will use the user's home directory
// with the folder name defined by io.DataFileDirectory (default "goxpy_data").
func (e *Experiment) SetOutputDirectory(dir string) {
	e.OutputDirectory = dir
}

// Initialize loads the embedded SDL/TTF binaries, initializes SDL (video,
// events and audio), opens the default playback audio device, creates the
// main window/renderer (`io.Screen`), and creates the default `DataFile`.
//
// It must be called exactly once before using the experiment, and `End`
// should be deferred immediately after successful initialization.
func (e *Experiment) Initialize() error {
	e.sdlLoader = binsdl.Load()
	e.imgLoader = binimg.Load()
	e.ttfLoader = binttf.Load()

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS | sdl.INIT_AUDIO); err != nil {
		return err
	}

	if err := ttf.Init(); err != nil {
		return err
	}

	// If no explicit window size was provided, we use the autodetect mode (0,0)
	// which io.NewScreen handles by using native resolution and high pixel density.
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

	screen, err := io.NewScreen(e.Name, e.WindowWidth, e.WindowHeight, e.BackgroundColor, e.Fullscreen)
	if err != nil {
		return err
	}
	e.Screen = screen
	e.Keyboard = &io.Keyboard{
		PollKeys: func() (sdl.Keycode, bool) {
			state := e.PollEvents(nil)
			return state.LastKey, state.QuitRequested
		},
	}
	e.Mouse = &io.Mouse{
		PollButtons: func() (uint32, bool) {
			state := e.PollEvents(nil)
			return state.LastMouseButton, state.QuitRequested
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
	dataFile, err := io.NewDataFile(outDir, e.SubjectID, e.Name)
	if err != nil {
		return err
	}
	e.Data = dataFile
	if len(e.Info) > 0 {
		e.Data.WriteParticipantInfo(e.Info)
	}

	return nil
}

// pollEvent is a hook for sdl.PollEvent to allow unit testing without
// a live SDL context.
var pollEvent = sdl.PollEvent

// getTicks is a hook for sdl.Ticks to allow unit testing.
var getTicks = sdl.Ticks

// sdlDelay is a hook for sdl.Delay to allow unit testing.
var sdlDelay = sdl.Delay

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
	e.event.LastMouseButton = 0

	var ev sdl.Event
	for pollEvent(&ev) {
		switch ev.Type {
		case sdl.EVENT_QUIT:
			e.event.QuitRequested = true
		case sdl.EVENT_KEY_DOWN:
			k := ev.KeyboardEvent().Key
			if k == sdl.K_ESCAPE {
				e.event.QuitRequested = true
			}
			if e.event.LastKey == 0 {
				e.event.LastKey = k
			}
		case sdl.EVENT_MOUSE_BUTTON_DOWN:
			if e.event.LastMouseButton == 0 {
				e.event.LastMouseButton = uint32(ev.MouseButtonEvent().Button)
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
	subtitle := "Goxpyriment " + io.Version
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
		e.Data.Save()
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
		return logic()
	})
}
