// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package control

import (
	"log"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/io"
	"github.com/Zyko0/go-sdl3/bin/binimg"
	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/bin/binttf"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
)

// EventState provides a convenient summary of the last processed input events.
// It is updated by Experiment.PollEvents.
type EventState struct {
	LastKey         sdl.Keycode
	LastMouseButton uint32
	QuitRequested   bool
}

// Experiment manages the global state of a behavioral or psychophysics experiment.
// It owns the SDL window/renderer (`Screen`), input devices (`Keyboard`, `Mouse`),
// audio device/manager, and the `DataFile` used for logging responses.
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

	sdlLoader interface{ Unload() }
	imgLoader interface{ Unload() }
	ttfLoader interface{ Unload() }

	event EventState
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

// SetOutputDirectory overrides the default folder used to store .xpd result
// files. If not called, Initialize will use io.DataFileDirectory, which
// defaults to "xpd_results".
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
	e.Keyboard = &io.Keyboard{}
	e.Mouse = &io.Mouse{Screen: screen}

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

	return nil
}

// PollEvents processes all pending SDL events, updates the experiment's
// aggregate `EventState`, and optionally forwards each SDL event to the
// provided handler callback.
//
// The handler can return true to stop processing further events for this
// polling cycle. The returned `EventState` summarizes the last keyboard and
// mouse button pressed and whether a quit/escape was requested.
func (e *Experiment) PollEvents(handle func(ev sdl.Event) bool) EventState {
	// Reset summary for this polling cycle.
	e.event.LastKey = 0
	e.event.LastMouseButton = 0
	e.event.QuitRequested = false

	var ev sdl.Event
	for sdl.PollEvent(&ev) {
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
	return state.LastKey, state.LastMouseButton, nil
}

func (e *Experiment) AddDataVariableNames(names []string) {
	e.Design.AddDataVariableNames(names)
	if e.Data != nil {
		e.Data.AddVariableNames(names)
	}
}

func (e *Experiment) AddBlock(b *design.Block, copies int) {
	e.Design.AddBlock(b, copies)
}

func (e *Experiment) AddExperimentInfo(text string) {
	e.Design.AddExperimentInfo(text)
}

func (e *Experiment) ShuffleBlocks() {
	e.Design.ShuffleBlocks()
}

func (e *Experiment) AddBWSFactor(name string, conditions []interface{}) {
	e.Design.AddBWSFactor(name, conditions)
}

func (e *Experiment) GetPermutedBWSFactorCondition(name string) interface{} {
	return e.Design.GetPermutedBWSFactorCondition(name, e.SubjectID)
}

func (e *Experiment) Summary() string {
	return e.Design.Summary()
}

// SetVSync toggles vertical synchronization on the screen.
// 1 to enable, 0 to disable.
func (e *Experiment) SetVSync(vsync int) error {
	if e.Screen == nil {
		return nil
	}
	return e.Screen.SetVSync(vsync)
}

// SetLogicalSize sets a device-independent resolution for the experiment.
func (e *Experiment) SetLogicalSize(width, height int32) error {
	if e.Screen == nil {
		return nil
	}
	return e.Screen.SetLogicalSize(width, height)
}

// Flip presents the backbuffer to the display using the experiment's screen.
// When VSync is enabled, this will typically block until the next vertical retrace.
func (e *Experiment) Flip() error {
	if e.Screen == nil {
		return nil
	}
	return e.Screen.Flip()
}

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

// End cleans up resources.
func (e *Experiment) End() {
	if e.Data != nil {
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
// The provided callback is called once per frame until it returns either:
//   - nil          to continue the loop,
//   - sdl.EndLoop  to terminate cleanly,
//   - any other error, which is returned to the caller.
func (e *Experiment) Run(logic func() error) error {
	// For simplicity in this prototype, we'll run the logic directly.
	// In a real implementation, we'd handle the RunLoop properly.
	return sdl.RunLoop(func() error {
		return logic()
	})
}
