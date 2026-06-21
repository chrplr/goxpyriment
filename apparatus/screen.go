// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

// Package apparatus provides the low-level input/output subsystems for goxpyriment:
//
//   - Screen — SDL window/renderer management, center-based coordinate system,
//     VSync control, and logical resolution mapping.
//   - Keyboard — blocking (Wait/WaitKeys) and non-blocking (Check) key input.
//   - Mouse — cursor visibility, position, and button input.
//   - GamePad — game-controller button input.
//   - GammaCorrector — inverse-gamma look-up table for luminance linearization.
//   - ResponseDevice / Response — unified response abstraction over keyboard, mouse,
//     gamepad, and TTL devices.
//
// Hardware trigger devices (DLP-IO8, parallel port, serial port) are in the
// separate [github.com/chrplr/goxpyriment/triggers] package.
//
// Most types in this package are not used directly; the control.Experiment
// facade creates and wires them together during initialization.
package apparatus

import (
	"fmt"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
)

// Re-export SDL types for convenience and to avoid direct SDL dependencies in user code.
type FRect = sdl.FRect
type FPoint = sdl.FPoint
type Color = sdl.Color

// Rendering type aliases — re-exported so callers need not import go-sdl3 directly.
type Texture = sdl.Texture
type Surface = sdl.Surface
type PixelFormat = sdl.PixelFormat
type TextureAccess = sdl.TextureAccess
type BlendMode = sdl.BlendMode

// Common pixel format / texture access / blend mode constants.
const (
	PIXELFORMAT_RGBA32      PixelFormat   = sdl.PIXELFORMAT_RGBA32
	TEXTUREACCESS_STREAMING TextureAccess = sdl.TEXTUREACCESS_STREAMING
	BLENDMODE_BLEND         BlendMode     = sdl.BLENDMODE_BLEND
)

// CreateSurfaceFrom allocates a Surface backed by existing pixel data.
// This wraps sdl.CreateSurfaceFrom so callers can avoid importing go-sdl3 directly.
func CreateSurfaceFrom(width, height int, format PixelFormat, pixels []byte, pitch int) (*Surface, error) {
	return sdl.CreateSurfaceFrom(width, height, format, pixels, pitch)
}

// Screen wraps the SDL window and hardware‑accelerated renderer.
// It is responsible for:
//   - managing the backbuffer / presenting frames (Clear, Update, Flip),
//   - tracking the logical coordinate system and conversion from centered
//     coordinates to SDL's top‑left space (CenterToSDL),
//   - holding the default font and optional canvas/logical size overrides.
type Screen struct {
	Window       *sdl.Window
	Renderer     *sdl.Renderer
	BgColor      sdl.Color
	Width        int
	Height       int
	DefaultFont  *ttf.Font
	CanvasOffset *sdl.FPoint // If not nil, use this instead of true center
	LogicalSize  *sdl.FPoint // If not nil, use this for CenterToSDL
	lastFlipTime time.Time   // tracks per-frame pacing for PacedFlip
}

// CenterToSDL converts center‑based coordinates to SDL top‑left based
// coordinates using either the current logical size, canvas offset, or the
// renderer output size as a fallback.
func (s *Screen) CenterToSDL(x, y float32) (float32, float32) {
	if s.CanvasOffset != nil {
		return s.CanvasOffset.X + x, s.CanvasOffset.Y - y
	}
	if s.LogicalSize != nil {
		return s.LogicalSize.X/2 + x, s.LogicalSize.Y/2 - y
	}
	w, h, _ := s.Renderer.CurrentOutputSize()
	return float32(w)/2 + x, float32(h)/2 - y
}

// LogicalCenterToSDL converts center-based coordinates to SDL top-left based coordinates using specified dimensions.
func (s *Screen) LogicalCenterToSDL(x, y float32, width, height float32) (float32, float32) {
	return width/2 + x, height/2 - y
}

// MousePosition returns the current mouse cursor position in the center-based
// coordinate system used by visual stimuli (0,0 = screen center).
//
// On HiDPI screens and whenever a logical presentation size has been set via
// SetLogicalSize, SDL's GetMouseState returns raw window-pixel coordinates
// that differ from the renderer's logical coordinate space. This method
// converts correctly using SDL_RenderCoordinatesFromWindow before applying
// the center-offset transform, so the returned (x, y) can be compared
// directly with stimulus positions.
func (s *Screen) MousePosition() (float32, float32) {
	_, wx, wy := sdl.GetMouseState()
	// Convert from window-pixel space to logical renderer space.
	lx, ly, err := s.Renderer.RenderCoordinatesFromWindow(wx, wy)
	if err != nil {
		lx, ly = wx, wy
	}
	// Convert from SDL top-left logical coords to center-based coords,
	// mirroring the inverse of CenterToSDL.
	if s.CanvasOffset != nil {
		return lx - s.CanvasOffset.X, s.CanvasOffset.Y - ly
	}
	if s.LogicalSize != nil {
		return lx - s.LogicalSize.X/2, s.LogicalSize.Y/2 - ly
	}
	w, h, _ := s.Renderer.CurrentOutputSize()
	return lx - float32(w)/2, float32(h)/2 - ly
}

// SetLogicalSize sets a device‑independent logical resolution for the
// renderer. All subsequent drawing operations are scaled to this size using
// SDL's logical presentation (letterboxed by default).
func (s *Screen) SetLogicalSize(width, height int32) error {
	s.LogicalSize = &sdl.FPoint{X: float32(width), Y: float32(height)}
	return s.Renderer.SetLogicalPresentation(width, height, sdl.LOGICAL_PRESENTATION_LETTERBOX)
}

// SystemInfo holds SDL, renderer, and audio runtime properties captured once
// at experiment startup for inclusion in the data file metadata header.
// Together with DisplayInfo it provides a complete postmortem record of the
// software and hardware configuration used during a session.
type SystemInfo struct {
	SDLVersion    string // SDL library version, e.g. "3.2.10"
	VideoDriver   string // SDL video driver, e.g. "wayland", "x11", "windows"
	RendererName  string // GPU renderer backend, e.g. "opengl", "vulkan", "metal"
	PhysicalW     int32  // renderer output width in physical pixels (HiDPI-aware)
	PhysicalH     int32  // renderer output height in physical pixels
	LogicalW      int32  // logical window width (experiment coordinate space)
	LogicalH      int32  // logical window height
	Fullscreen    bool   // true when running in fullscreen mode
	VSync         int    // VSync state: 1=on, 0=off, -1=adaptive
	AudioDriver   string // SDL audio driver, e.g. "pulseaudio", "alsa", "coreaudio"
	AudioFormat   string // audio sample format, e.g. "SDL_AUDIO_F32LE"
	AudioFreq     int32  // sample rate in Hz, e.g. 44100 or 48000
	AudioChannels int32  // number of audio output channels (1=mono, 2=stereo)
	AudioFrames   int32  // hardware buffer size in sample frames
}

// GatherSystemInfo collects SDL and renderer properties from this Screen.
// Audio fields (AudioDriver, AudioFormat, AudioFreq, AudioChannels, AudioFrames)
// are left at their zero values; the caller fills them in after opening the
// audio device.
func (s *Screen) GatherSystemInfo() SystemInfo {
	info := SystemInfo{
		SDLVersion:  sdl.GetVersion().String(),
		VideoDriver: sdl.GetCurrentVideoDriver(),
	}
	if s.Renderer != nil {
		if name, err := s.Renderer.Name(); err == nil {
			info.RendererName = name
		}
		if w, h, err := s.Renderer.RenderOutputSize(); err == nil {
			info.PhysicalW = w
			info.PhysicalH = h
		}
		if v, err := s.VSync(); err == nil {
			info.VSync = v
		}
	}
	if s.Window != nil {
		info.Fullscreen = (s.Window.Flags() & sdl.WINDOW_FULLSCREEN) != 0
	}
	if s.LogicalSize != nil {
		info.LogicalW = int32(s.LogicalSize.X)
		info.LogicalH = int32(s.LogicalSize.Y)
	} else {
		info.LogicalW = int32(s.Width)
		info.LogicalH = int32(s.Height)
	}
	return info
}

// DisplayInfo holds display properties queried once at experiment startup.
// It is intended to be logged into the .csv metadata header so that stimulus
// timing can be interpreted correctly during analysis.
type DisplayInfo struct {
	ID             uint32  // SDL display ID
	Name           string  // monitor name reported by the OS
	NativeW        int32   // native display width in pixels
	NativeH        int32   // native display height in pixels
	PixelDensity   float32 // HiDPI scale from display mode (1.0 = standard, 2.0 = Retina-style)
	ContentScale   float32 // OS content-scale factor (logical→physical; may differ from PixelDensity)
	RefreshRate    float32 // nominal refresh rate in Hz
	BitsPerPixel   uint8   // total bits per pixel (e.g. 32)
	BitsPerChannel uint8   // bits per colour channel (e.g. 8 for sRGB, 10 for HDR)
	PixelFormat    string  // human-readable pixel format name (e.g. "SDL_PIXELFORMAT_XRGB8888")
	BoundsX        int32   // display desktop origin X in screen coordinates
	BoundsY        int32   // display desktop origin Y in screen coordinates
	BoundsW        int32   // display desktop width in screen coordinates
	BoundsH        int32   // display desktop height in screen coordinates
}

// DisplayInfo queries the display properties for the screen's current window.
// Fields that cannot be determined are left at their zero values.
func (s *Screen) DisplayInfo() DisplayInfo {
	id := sdl.GetDisplayForWindow(s.Window)
	info := DisplayInfo{ID: uint32(id)}

	if name, err := id.Name(); err == nil {
		info.Name = name
	}
	if mode, err := id.CurrentDisplayMode(); err == nil && mode != nil {
		info.NativeW = mode.W
		info.NativeH = mode.H
		info.PixelDensity = mode.PixelDensity
		info.RefreshRate = mode.RefreshRate
		info.PixelFormat = mode.Format.Name()
		if details, err := mode.Format.Details(); err == nil && details != nil {
			info.BitsPerPixel = details.BitsPerPixel
			info.BitsPerChannel = details.Rbits
		}
	}
	if scale, err := id.ContentScale(); err == nil {
		info.ContentScale = scale
	}
	if bounds, err := id.Bounds(); err == nil && bounds != nil {
		info.BoundsX = bounds.X
		info.BoundsY = bounds.Y
		info.BoundsW = bounds.W
		info.BoundsH = bounds.H
	}
	return info
}

// Size returns the current renderer output size.
func (s *Screen) Size() (int32, int32, error) {
	w, h, err := s.Renderer.RenderOutputSize()
	return w, h, err
}

// Clear clears the screen with the background color.
func (s *Screen) Clear() error {
	if err := s.Renderer.SetDrawColor(s.BgColor.R, s.BgColor.G, s.BgColor.B, s.BgColor.A); err != nil {
		return fmt.Errorf("apparatus.Screen.Clear: setting draw color: %w", err)
	}
	return s.Renderer.Clear()
}

// ClearAndUpdate clears the screen with the background color and presents the buffer.
// It is a convenience for the common pattern Clear() then Update().
func (s *Screen) ClearAndUpdate() error {
	if err := s.Clear(); err != nil {
		return fmt.Errorf("apparatus.Screen.ClearAndUpdate: %w", err)
	}
	return s.Update()
}

// Update presents the rendered buffer.
func (s *Screen) Update() error {
	// Ensure we are presenting the window, not a texture
	if s.Renderer.RenderTarget() != nil {
		if err := s.Renderer.SetRenderTarget(nil); err != nil {
			return fmt.Errorf("apparatus.Screen.Update: resetting render target: %w", err)
		}
	}
	return s.Renderer.Present()
}

// Flip is an alias for Update and presents the backbuffer to the display.
// When VSync is enabled on the renderer, this call will typically block
// until the next vertical retrace, providing a well-defined stimulus onset.
func (s *Screen) Flip() error {
	return s.Update()
}

// FlipTS presents the backbuffer (like Flip) and immediately captures the
// SDL nanosecond timestamp after the flip completes.
//
// The returned timestamp is in the same nanosecond reference frame as SDL3
// event timestamps (sdl.TicksNS()), so it can be directly subtracted from
// the Timestamp field of a KeyboardEvent or MouseButtonEvent to compute
// a hardware-precision reaction time:
//
//	onset, _ := screen.FlipTS()
//	key, eventTS, _ := kb.GetKeyEventTS(keys, -1)
//	rtNS := int64(eventTS - onset)
func (s *Screen) FlipTS() (uint64, error) {
	if err := s.Update(); err != nil {
		return 0, fmt.Errorf("apparatus.Screen.FlipTS: %w", err)
	}
	return sdl.TicksNS(), nil
}

// PacedFlip presents the backbuffer (like Flip) and, when the driver does not
// block for VSYNC (triple/mailbox buffering), busy-waits until the expected
// frame boundary. It is a drop-in for Flip in any per-frame stimulus loop.
//
// On systems with well-behaved double-buffered VSYNC the busy-wait runs zero
// iterations and overhead is negligible. On systems where SDL_RenderPresent
// returns immediately (NVIDIA + compositor, Wayland mailbox, triple-buffer
// drivers), it enforces the correct frame period so that each call occupies
// exactly one display frame — preventing bright frames from being swallowed
// before the monitor scans them out.
func (s *Screen) PacedFlip() error {
	if err := s.Update(); err != nil {
		return fmt.Errorf("apparatus.Screen.PacedFlip: %w", err)
	}
	now := time.Now()
	if !s.lastFlipTime.IsZero() {
		target := s.lastFlipTime.Add(s.FrameDuration())
		for now.Before(target) {
			now = time.Now()
		}
	}
	s.lastFlipTime = now
	return nil
}

// PacedFlipTS is PacedFlip plus an SDL nanosecond timestamp captured after the
// flip completes. Use this as a drop-in for FlipTS in timing-critical loops
// where triple/mailbox buffering must be guarded against.
func (s *Screen) PacedFlipTS() (uint64, error) {
	if err := s.PacedFlip(); err != nil {
		return 0, fmt.Errorf("apparatus.Screen.PacedFlipTS: %w", err)
	}
	return sdl.TicksNS(), nil
}

// SetVSync toggles vertical synchronization.
// vsync: 1 to enable, 0 to disable, -1 for adaptive vsync.
func (s *Screen) SetVSync(vsync int) error {
	return s.Renderer.SetVSync(int32(vsync))
}

// FrameDuration returns the nominal duration of one display frame based on
// the refresh rate of the screen's current display mode.
// Falls back to 60 Hz if the refresh rate cannot be queried.
func (s *Screen) FrameDuration() time.Duration {
	var hz float32 = 60.0
	id := sdl.GetDisplayForWindow(s.Window)
	if mode, err := id.CurrentDisplayMode(); err == nil && mode != nil && mode.RefreshRate > 0 {
		hz = mode.RefreshRate
	}
	return time.Duration(float64(time.Second) / float64(hz))
}

// VSync returns the current VSync state.
func (s *Screen) VSync() (int, error) {
	v, err := s.Renderer.VSync()
	return int(v), err
}

// WaitFrames holds the current solid-color display state for exactly n
// additional VSYNC edges. Before each Present it re-clears the backbuffer
// with the renderer's current draw color so that the double-buffer swap never
// reveals a stale backbuffer. Returns the SDL nanosecond timestamp after the
// final flip.
//
// Intended use: after a solid-color Clear+Flip, hold that color for n more frames.
//
//	r.SetDrawColor(255, 255, 255, 255)
//	r.Clear()
//	screen.Flip()           // frame 0: white appears
//	screen.WaitFrames(5)    // frames 1–5: white stays on screen
//
// For stimuli composed of textures or multiple draw calls, use an explicit
// per-frame redraw loop instead:
//
//	for i := 0; i < n; i++ {
//	    drawMyStimulus(screen)
//	    screen.Flip()
//	}
func (s *Screen) WaitFrames(n int) (uint64, error) {
	for i := 0; i < n; i++ {
		// Re-clear backbuffer with the current draw color so both buffers stay
		// identical and the swap is visually a no-op.
		if err := s.Renderer.Clear(); err != nil {
			return 0, fmt.Errorf("apparatus.Screen.WaitFrames: clearing backbuffer: %w", err)
		}
		if err := s.PacedFlip(); err != nil {
			return 0, fmt.Errorf("apparatus.Screen.WaitFrames: %w", err)
		}
	}
	return sdl.TicksNS(), nil
}

// RefreshRate returns the display's nominal refresh rate in Hz when VSync is
// enabled, or 0 if VSync is disabled or the rate cannot be queried.
//
// The value comes from the OS display mode (same source as FrameDuration).
// Use the frames sub-test in tests/Timing-Tests for a hardware-verified
// measurement.
func (s *Screen) RefreshRate() float32 {
	v, err := s.VSync()
	if err != nil || v == 0 {
		return 0
	}
	id := sdl.GetDisplayForWindow(s.Window)
	if mode, err := id.CurrentDisplayMode(); err == nil && mode != nil && mode.RefreshRate > 0 {
		return mode.RefreshRate
	}
	return 0
}

// Destroy cleans up the window and renderer.
func (s *Screen) Destroy() {
	if s.Renderer != nil {
		s.Renderer.Destroy()
	}
	if s.Window != nil {
		s.Window.Destroy()
	}
}
