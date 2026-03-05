package control

import (
	"goxpyriment/design"
	"goxpyriment/io"
	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/bin/binimg"
	"github.com/Zyko0/go-sdl3/bin/binttf"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
)

// Experiment manages the overall state and initialization.
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
	DefaultFont     *ttf.Font
	AudioDevice     sdl.AudioDeviceID
	WindowWidth     int
	WindowHeight    int
	Fullscreen      bool
	
	sdlLoader interface { Unload() }
	imgLoader interface { Unload() }
	ttfLoader interface { Unload() }
}

// NewExperiment creates a new Experiment instance.
func NewExperiment(name string, width, height int, fullscreen bool) *Experiment {
	return &Experiment{
		Name:            name,
		Design:          design.NewExperiment(name),
		BackgroundColor: DefaultBackgroundColor,
		ForegroundColor: DefaultTextColor,
		SubjectID:       0, // Default subject ID
		WindowWidth:     width,
		WindowHeight:    height,
		Fullscreen:      fullscreen,
	}
}

// Initialize sets up SDL and the screen.
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
	
	// Initialize Audio
	dev, err := sdl.AUDIO_DEVICE_DEFAULT_PLAYBACK.OpenAudioDevice(nil)
	if err != nil {
		return err
	}
	e.AudioDevice = dev
	
	screen, err := io.NewScreen(e.Name, e.WindowWidth, e.WindowHeight, e.BackgroundColor, e.Fullscreen)
	if err != nil {
		return err
	}
	e.Screen = screen
	e.Keyboard = &io.Keyboard{}
	e.Mouse = &io.Mouse{}
	
	// Initialize DataFile
	dataFile, err := io.NewDataFile("", e.SubjectID, e.Name)
	if err != nil {
		return err
	}
	e.Data = dataFile
	
	return nil
}

// HandleEvents processes pending SDL events and returns the first keyboard and mouse button pressed, and any termination signal.
func (e *Experiment) HandleEvents() (sdl.Keycode, uint32, error) {
	var key sdl.Keycode
	var btn uint32

	var event sdl.Event
	for sdl.PollEvent(&event) {
		switch event.Type {
		case sdl.EVENT_QUIT:
			return 0, 0, sdl.EndLoop
		case sdl.EVENT_KEY_DOWN:
			k := event.KeyboardEvent().Key
			if k == sdl.K_ESCAPE {
				return 0, 0, sdl.EndLoop
			}
			if key == 0 {
				key = k
			}
		case sdl.EVENT_MOUSE_BUTTON_DOWN:
			if btn == 0 {
				btn = uint32(event.MouseButtonEvent().Button)
			}
		}
	}
	return key, btn, nil
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

// Run executes the main experiment logic.
func (e *Experiment) Run(logic func() error) error {
	// For simplicity in this prototype, we'll run the logic directly.
	// In a real implementation, we'd handle the RunLoop properly.
	return sdl.RunLoop(func() error {
		return logic()
	})
}
