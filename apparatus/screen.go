// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

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
	"sort"
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
	CanvasOffset *sdl.FPoint   // If not nil, use this instead of true center
	LogicalSize  *sdl.FPoint   // If not nil, use this for CenterToSDL
	lastFlipNS   uint64        // SDL-clock (sdl.TicksNS) time of the previous present; 0 = none yet
	frameDur     time.Duration // cached nominal frame duration used by Update's pacing (0 = not yet computed)
	vsyncCached  int           // cached renderer VSync state; pacing is off when 0
	vsyncKnown   bool          // whether vsyncCached has been filled in
}

// CenterToSDL converts center‑based coordinates to SDL top‑left based
// coordinates using either the current logical size, canvas offset, or the
// renderer output size as a fallback.
//
// IMPORTANT — axis convention: the center-based system has its origin (0,0) at
// the screen center and **+Y points UP** (like maths / vision-science plots),
// which is the OPPOSITE of SDL's top-left, Y-down pixel space. Note the "- y"
// below: a stimulus at a larger Y is drawn HIGHER on the screen. So to place a
// header above a box, give it a MORE POSITIVE Y; to place a caption below, a
// more negative Y. Getting this backwards (using negative Y for "up") is a
// recurring bug — everything renders vertically mirrored. All stimulus
// positions, mouse coordinates (Screen.MousePosition), and layout code use this
// convention.
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

// CenteredRect returns the SDL destination rectangle for a w×h texture centered
// at the center-based position pos. It converts pos via CenterToSDL and offsets
// by half the size, factoring out the destX/destY + FRect idiom repeated by
// every texture-backed stimulus's Draw method.
func (s *Screen) CenteredRect(pos sdl.FPoint, w, h float32) *sdl.FRect {
	cx, cy := s.CenterToSDL(pos.X, pos.Y)
	return &sdl.FRect{X: cx - w/2, Y: cy - h/2, W: w, H: h}
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
	SDLVersion     string  // SDL library version, e.g. "3.2.10"
	VideoDriver    string  // SDL video driver, e.g. "wayland", "x11", "windows"
	RendererName   string  // SDL renderer BACKEND, e.g. "opengl", "vulkan", "metal" — not the GPU
	GLRenderer     string  // OpenGL GL_RENDERER: the GPU/driver actually rendering; "" if unavailable
	PhysicalW      int32   // renderer output width in physical pixels (HiDPI-aware)
	PhysicalH      int32   // renderer output height in physical pixels
	LogicalW       int32   // logical window width (experiment coordinate space)
	LogicalH       int32   // logical window height
	Fullscreen     bool    // true when running in fullscreen mode
	FullscreenMode string  // "exclusive" or "desktop" — whether the compositor was bypassed
	VSync          int     // VSync state: 1=on, 0=off, -1=adaptive
	NominalHz      float64 // refresh rate SDL reports for the current display mode
	MeasuredHz     float64 // refresh rate measured at startup (CalibrateRefresh); 0 = not measured
	AudioDriver    string  // SDL audio driver, e.g. "pulseaudio", "alsa", "coreaudio"
	AudioFormat    string  // audio sample format, e.g. "SDL_AUDIO_F32LE"
	AudioFreq      int32   // sample rate in Hz, e.g. 44100 or 48000
	AudioChannels  int32   // number of audio output channels (1=mono, 2=stereo)
	AudioFrames    int32   // hardware buffer size in sample frames
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
		// Which GPU ran it — RendererName only says which SDL backend did.
		info.GLRenderer = glRendererString()
	}
	if d := s.FrameDuration(); d > 0 {
		info.NominalHz = float64(time.Second) / float64(d)
	}
	if s.Window != nil {
		info.Fullscreen = (s.Window.Flags() & sdl.WINDOW_FULLSCREEN) != 0
		if info.Fullscreen {
			// Exclusive vs desktop changes whether the compositor sits in the
			// presentation path, so results from the two are not comparable.
			// Record which one this session actually got.
			info.FullscreenMode = FullscreenPolicyInEffect().String()
		}
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
	if err := s.Renderer.Clear(); err != nil {
		return fmt.Errorf("apparatus.Screen.Clear: %w", err)
	}
	return s.fillWholeTarget()
}

// fillWholeTarget paints the entire render target with the renderer's current
// draw color, on top of a clear that has just happened.
//
// This exists to defeat a presentation bug seen on Linux under a compositor
// (GNOME/Mutter, both native Wayland and Xwayland; reproduced on Intel Meteor
// Lake / i915 with the opengl, vulkan and software SDL renderers). A frame
// whose only content is SDL_RenderClear — no draw calls at all — is not
// reliably scanned out: the panel can hold a stale frame for seconds while the
// client and the compositor both report every frame presented on time. Adding
// one real draw call per frame makes the problem disappear. The same loop is
// perfect on the kmsdrm backend, where no compositor is involved.
//
// The rendered image is unchanged — the fill uses the colour Clear just used.
// Blend mode is forced to NONE for the fill and restored afterwards, because
// SDL_RenderClear ignores the blend mode while SDL_RenderFillRect honours it,
// and a translucent background colour would otherwise blend instead of replace.
func (s *Screen) fillWholeTarget() error {
	prev, err := s.Renderer.DrawBlendMode()
	if err != nil {
		return fmt.Errorf("apparatus.Screen.Clear: reading blend mode: %w", err)
	}
	if prev != sdl.BLENDMODE_NONE {
		if err := s.Renderer.SetDrawBlendMode(sdl.BLENDMODE_NONE); err != nil {
			return fmt.Errorf("apparatus.Screen.Clear: setting blend mode: %w", err)
		}
		defer func() { _ = s.Renderer.SetDrawBlendMode(prev) }()
	}
	if err := s.Renderer.RenderFillRect(nil); err != nil {
		return fmt.Errorf("apparatus.Screen.Clear: filling target: %w", err)
	}
	return nil
}

// ClearAndUpdate clears the screen with the background color and presents the buffer.
// It is a convenience for the common pattern Clear() then Update().
func (s *Screen) ClearAndUpdate() error {
	if err := s.Clear(); err != nil {
		return fmt.Errorf("apparatus.Screen.ClearAndUpdate: %w", err)
	}
	return s.Update()
}

// Update presents the rendered buffer and does not return until the frame
// boundary, so every Update occupies exactly one display frame.
//
// On desktop this maps to SDL_RenderPresent followed by a busy-wait to the
// expected frame boundary (see paceToFrame). The wait exists because
// SDL_RenderPresent cannot be trusted to block until the retrace: under
// triple/mailbox buffering it queues the frame and returns immediately.
// That is not an exotic configuration — measured on Intel i915 + Wayland with
// a well-behaved 120 Hz panel, unaided presents still came back as little as
// 6.95 ms apart against an 8.33 ms frame. Where the driver does block
// correctly, the wait runs zero iterations and costs nothing, which is why
// this is unconditional rather than selected per platform: the correct
// behaviour is not predictable from GOOS, video driver, or window mode.
//
// Pacing is skipped when VSync is disabled, since a caller who turned VSync
// off wants frames as fast as the GPU produces them.
//
// In the browser (GOOS=js) present parks until the next requestAnimationFrame
// — the browser's VSYNC equivalent — because canvas updates are only
// composited when the page yields, and paceToFrame is a no-op there
// (see screen_present_js.go).
func (s *Screen) Update() error {
	// Ensure we are presenting the window, not a texture
	if s.Renderer.RenderTarget() != nil {
		if err := s.Renderer.SetRenderTarget(nil); err != nil {
			return fmt.Errorf("apparatus.Screen.Update: resetting render target: %w", err)
		}
	}
	// Sample the previous flip's time before presenting: present() records its
	// own timestamp into lastFlipNS, and the boundary to wait for is one frame
	// after the PREVIOUS flip.
	prevFlipNS := s.lastFlipNS
	if err := s.present(); err != nil {
		return err
	}
	if s.pacingEnabled() {
		s.paceToFrame(prevFlipNS)
	}
	return nil
}

// Flip is an alias for Update and presents the backbuffer to the display,
// returning at the frame boundary.
func (s *Screen) Flip() error {
	return s.Update()
}

// pacingEnabled reports whether Update should hold to the frame boundary. It
// caches the renderer's VSync state — querying SDL every frame is avoidable
// work on this path — and SetVSync refreshes the cache.
func (s *Screen) pacingEnabled() bool {
	if !s.vsyncKnown {
		v, err := s.Renderer.VSync()
		if err != nil {
			v = 1 // assume VSync is on; pacing is the safe default
		}
		s.vsyncCached, s.vsyncKnown = int(v), true
	}
	return s.vsyncCached != 0
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
	// Update already stamped this flip — present() records the time it
	// returned, and paceToFrame refreshes it to the frame boundary it waited
	// for. Reading the clock again here would be a third sdl.TicksNS() call
	// per flip for a value already in hand, a few nanoseconds later.
	return s.lastFlipNS, nil
}

// SetVSync toggles vertical synchronization.
// vsync: 1 to enable, 0 to disable, -1 for adaptive vsync.
//
// Turning VSync off also turns off Update's frame pacing — see pacingEnabled.
func (s *Screen) SetVSync(vsync int) error {
	if err := s.Renderer.SetVSync(int32(vsync)); err != nil {
		return err
	}
	s.vsyncCached, s.vsyncKnown = vsync, true
	return nil
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

// CalibrateRefresh measures the display's actual frame period by presenting n
// frames and returning the median interval between them.
//
// It deliberately bypasses Update's pacing and presents directly, so what it
// reports is the unaided driver behaviour — otherwise it would simply measure
// the pacing spin and always agree with the nominal rate.
//
// Compare the result against FrameDuration to catch the three configurations
// that silently corrupt stimulus timing:
//
//   - measured ≈ nominal — the normal case.
//   - measured well BELOW nominal — SDL_RenderPresent is not blocking to the
//     retrace (triple/mailbox buffering). Update's pacing covers this.
//   - measured well ABOVE nominal — frames are being dropped before they reach
//     the panel (a compositor throttling an unfocused or occluded window will
//     do this, as will a GPU that cannot keep up). Pacing CANNOT fix this; it
//     enforces a minimum frame time, not a maximum.
//
// Each frame is a real Clear (which fills the target), never a bare present:
// a frame carrying no draw calls is not reliably scanned out (see
// fillWholeTarget).
func (s *Screen) CalibrateRefresh(n int) (time.Duration, error) {
	if n < 2 {
		return 0, fmt.Errorf("apparatus.Screen.CalibrateRefresh: need at least 2 frames, got %d", n)
	}
	intervals := make([]time.Duration, 0, n-1)
	var prev uint64
	for i := 0; i < n; i++ {
		if err := s.Clear(); err != nil {
			return 0, fmt.Errorf("apparatus.Screen.CalibrateRefresh: %w", err)
		}
		if s.Renderer.RenderTarget() != nil {
			if err := s.Renderer.SetRenderTarget(nil); err != nil {
				return 0, fmt.Errorf("apparatus.Screen.CalibrateRefresh: resetting render target: %w", err)
			}
		}
		if err := s.present(); err != nil {
			return 0, fmt.Errorf("apparatus.Screen.CalibrateRefresh: %w", err)
		}
		now := sdl.TicksNS()
		if prev != 0 {
			intervals = append(intervals, time.Duration(now-prev))
		}
		prev = now
	}
	// The pacer's baseline is now a bare present rather than a paced flip;
	// leaving it would make the first Update after calibration pace against it.
	// That is harmless (it is at most one frame old) and self-correcting.
	sort.Slice(intervals, func(i, j int) bool { return intervals[i] < intervals[j] })
	return intervals[len(intervals)/2], nil
}

// There is deliberately no "wait for n VSYNC edges" method.
//
// Nothing in SDL3 exposes the retrace on its own — there is no SDL_WaitVBlank —
// so the only way to be locked to the display is to present a frame, and a
// frame that carries no draw calls is not reliably scanned out under a
// compositor (see fillWholeTarget). A hold must therefore redraw its content
// once per frame:
//
//	for i := 0; i < n; i++ {
//	    screen.Clear()
//	    stim.Draw(screen)
//	    screen.Flip()
//	}
//
// control.Experiment.ShowFrames and control.Experiment.BlankFrames wrap that
// loop. An earlier WaitFrames(n) re-cleared with the renderer's *current* draw
// color instead of redrawing, which silently painted the whole screen with
// whichever color the last stimulus happened to leave set.

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
