// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build !js

package apparatus

import (
	"fmt"
	"runtime"

	"github.com/Zyko0/go-sdl3/sdl"
)

// orderedDisplays returns the connected displays with the primary first,
// followed by the rest in SDL's enumeration order.
//
// SDL_GetDisplays does NOT promise the primary comes first. Resolving index 0
// via GetPrimaryDisplay while indexing the raw list for every other value made
// the two disagree: whenever the primary sat at position n > 0 it answered to
// both 0 and n, and whatever sat at position 0 could not be selected at all.
// Deriving both displayByIndex and ListDisplays from this one ordering keeps
// the index accepted by -d identical to the position ListDisplays reports.
func orderedDisplays() ([]sdl.DisplayID, error) {
	displays, err := sdl.GetDisplays()
	if err != nil {
		return nil, fmt.Errorf("enumerate displays: %w", err)
	}
	primary := sdl.GetPrimaryDisplay()
	ordered := make([]sdl.DisplayID, 0, len(displays))
	for _, id := range displays {
		if id == primary {
			ordered = append(ordered, id)
			break
		}
	}
	for _, id := range displays {
		if id != primary {
			ordered = append(ordered, id)
		}
	}
	return ordered, nil
}

// FullscreenPolicy selects how a fullscreen window is presented.
type FullscreenPolicy int

const (
	// FullscreenAuto picks per platform — see exclusiveFullscreenWanted.
	FullscreenAuto FullscreenPolicy = iota
	// FullscreenExclusive forces exclusive fullscreen (a concrete display mode).
	FullscreenExclusive
	// FullscreenDesktop forces borderless fullscreen-desktop (no mode set).
	FullscreenDesktop
)

func (p FullscreenPolicy) String() string {
	switch p {
	case FullscreenExclusive:
		return "exclusive"
	case FullscreenDesktop:
		return "desktop"
	default:
		return "auto"
	}
}

// fullscreenPolicy is process-wide state applied when the next Screen is
// created, mirroring how control.SetAudioSampleFrames stages the audio buffer
// hint before the device opens. One Screen per experiment makes that safe.
var fullscreenPolicy = FullscreenAuto

// SetFullscreenPolicy overrides the automatic per-platform choice. Call it
// BEFORE NewScreen (i.e. before Experiment.Initialize); afterwards it has no
// effect on an already-created window.
//
// It exists so the choice can be MEASURED rather than assumed. The automatic
// policy is a judgement about which backends benefit from bypassing the
// compositor, and that judgement should be checkable on any machine — including
// ones where the automatic answer is "no", such as Wayland, where forcing
// exclusive is known to mis-handle cross-display requests under Mutter.
func SetFullscreenPolicy(p FullscreenPolicy) { fullscreenPolicy = p }

// FullscreenPolicyInEffect reports the policy that will be (or was) applied,
// resolving FullscreenAuto to what it actually chose on this platform. Record it
// alongside timing results: "exclusive" and "desktop" are not comparable.
func FullscreenPolicyInEffect() FullscreenPolicy {
	if exclusiveFullscreenWanted() {
		return FullscreenExclusive
	}
	return FullscreenDesktop
}

// exclusiveFullscreenWanted reports whether the fullscreen window should be
// pinned to a concrete display mode. In SDL3 a non-nil fullscreen mode means
// EXCLUSIVE fullscreen; nil means fullscreen-desktop (borderless).
//
// SetFullscreenPolicy overrides everything below. The rest is the FullscreenAuto
// default: two independent reasons to want exclusive, one per platform:
//
//   - Windows: exclusive fullscreen is what takes DWM out of the presentation
//     chain. Fullscreen-desktop leaves every frame going through the
//     compositor, which adds presentation latency and can drop or duplicate
//     frames — exactly what these experiments set out to avoid.
//   - X11: same argument. Compositing window managers often unredirect
//     fullscreen windows on their own, but "often" is not a guarantee worth
//     resting a timing measurement on, so ask for it explicitly.
//
// Deliberately NOT enabled elsewhere:
//
//   - Wayland: measured on GNOME/Mutter 2026-07-29 — SDL sends the mode's
//     geometry but the compositor keeps the surface on its current output, so a
//     cross-display request mode-sets the WRONG monitor and still presents
//     there. Wayland gives clients no way to demand an output; use a single
//     active output instead. -exclusive-fullscreen=on forces it anyway, for
//     re-testing on other compositors.
//
//   - macOS: WindowServer cannot be bypassed, so exclusive fullscreen buys no
//     timing benefit and only adds a mode-set and Spaces churn.
//
//   - KMS/DRM: there is no compositor to escape.
//
//   - dummy/offscreen and any future backend: an allowlist rather than an
//     exclusion list, so an unfamiliar driver gets the conservative path.
//
// The mode passed is always the display's CURRENT mode, so no resolution or
// refresh-rate change is ever requested — only the fullscreen style changes.
func exclusiveFullscreenWanted() bool {
	switch fullscreenPolicy {
	case FullscreenExclusive:
		return true
	case FullscreenDesktop:
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	switch sdl.GetCurrentVideoDriver() {
	case "windows", "x11":
		return true
	}
	return false
}

// displayByIndex resolves a 0-based display index to an SDL DisplayID.
// Index 0 always refers to the primary display.
// Returns an error if index is out of range.
func displayByIndex(index int) (sdl.DisplayID, error) {
	ordered, err := orderedDisplays()
	if err != nil {
		return 0, err
	}
	if len(ordered) == 0 {
		// Enumeration succeeded but reported nothing: fall back to the primary
		// rather than refusing to open, which is what index 0 did before.
		if index == 0 {
			return sdl.GetPrimaryDisplay(), nil
		}
		return 0, fmt.Errorf("display index %d requested but no displays were enumerated", index)
	}
	if index < 0 || index >= len(ordered) {
		return 0, fmt.Errorf("display index %d out of range [0, %d)", index, len(ordered))
	}
	return ordered[index], nil
}

// ListDisplays returns metadata for all connected displays, ordered so that
// index 0 is the primary display. Pass an index to NewScreen (or set
// Experiment.ScreenNumber) to open the window on a specific monitor.
func ListDisplays() ([]DisplayInfo, error) {
	displays, err := orderedDisplays()
	if err != nil {
		return nil, err
	}
	infos := make([]DisplayInfo, len(displays))
	for i, id := range displays {
		infos[i] = DisplayInfo{ID: uint32(id)}
		if name, err := id.Name(); err == nil {
			infos[i].Name = name
		}
		if mode, err := id.CurrentDisplayMode(); err == nil && mode != nil {
			infos[i].NativeW = mode.W
			infos[i].NativeH = mode.H
			infos[i].RefreshRate = mode.RefreshRate
		}
		if scale, err := id.ContentScale(); err == nil {
			infos[i].ContentScale = scale
		}
		if bounds, err := id.Bounds(); err == nil && bounds != nil {
			infos[i].BoundsX = bounds.X
			infos[i].BoundsY = bounds.Y
			infos[i].BoundsW = bounds.W
			infos[i].BoundsH = bounds.H
		}
	}
	return infos, nil
}

// NewScreen initializes a new SDL window and renderer.
//
// width and height specify the logical experiment resolution. When fullscreen
// is true, or when width/height are 0, the physical window is created at the
// desktop's native resolution in exclusive fullscreen and the renderer is
// configured with a logical size matching the requested resolution (if > 0).
//
// displayIndex selects which monitor to use (0 = primary display). Pass a
// value ≥ 1 to target a secondary monitor — useful in lab settings where the
// experimenter and participant use different screens. Use ListDisplays() to
// enumerate available displays and their properties.
func NewScreen(title string, width, height int, bgColor sdl.Color, fullscreen bool, displayIndex int) (*Screen, error) {
	target, err := displayByIndex(displayIndex)
	if err != nil {
		return nil, fmt.Errorf("apparatus.NewScreen: selecting display %d: %w", displayIndex, err)
	}

	if fullscreen || (width == 0 && height == 0) {
		// Create the window directly with the FULLSCREEN flag.
		// Two-step approach (create then SetFullscreen) is unreliable on
		// KMS/DRM: SetFullscreen is asynchronous there, so window.Size()
		// immediately after still returns the pre-fullscreen dimensions.
		// Including WINDOW_FULLSCREEN at creation time avoids this race.
		//
		// Embed the target display's position in the creation properties instead
		// of calling SetPosition after the window is already fullscreen.
		// Post-creation SetPosition on a fullscreen window can crash the display
		// manager on some compositors (KDE/GNOME on X11). The windowed path uses
		// the same property-at-creation approach.
		//
		// This runs for EVERY index, including 0. Leaving index 0 to a bare
		// CreateWindow meant the default (-d absent, or -d 0) never named a
		// display at all, so SDL opened wherever it liked — in practice on
		// whichever output had focus. Two runs of the same command could land on
		// different monitors, which is invisible in the software timing numbers
		// and ruins a photodiode recording.
		// Where a fullscreen MODE is wanted (see exclusiveFullscreenWanted), the
		// mode has to be in place BEFORE the window first enters fullscreen.
		// Wayland is the reason: the compositor binds the surface to an output
		// when it goes fullscreen, at map time. A window born fullscreen has
		// already been assigned by then, and setting the mode afterwards does
		// not move it — which is exactly why -d 0 and -d 1 both opened on the
		// built-in panel. So for those backends: create HIDDEN and windowed,
		// set the mode, enter fullscreen, then show.
		//
		// Everything else keeps the original one-step creation. That matters
		// most on KMS/DRM, where SetFullscreen is asynchronous and window.Size()
		// straight afterwards still reports pre-fullscreen dimensions; including
		// WINDOW_FULLSCREEN at creation time avoids that race.
		deferFullscreen := exclusiveFullscreenWanted()

		// Embed the target display's position in the creation properties rather
		// than calling SetPosition later. Post-creation SetPosition on a
		// fullscreen window can crash the display manager on some compositors
		// (KDE/GNOME on X11). The windowed path below does the same.
		//
		// This runs for EVERY index, including 0. Leaving index 0 to a bare
		// CreateWindow meant the default (-d absent, or -d 0) never named a
		// display at all, so SDL opened wherever it liked — in practice on
		// whichever output had focus. Two runs of the same command could land on
		// different monitors, which is invisible in the software timing numbers
		// and ruins a photodiode recording.
		var window *sdl.Window
		var werr error
		if bounds, berr := target.Bounds(); berr == nil && bounds != nil {
			create := map[string]any{
				"SDL.window.create.title":              title,
				"SDL.window.create.x":                  bounds.X,
				"SDL.window.create.y":                  bounds.Y,
				"SDL.window.create.width":              bounds.W,
				"SDL.window.create.height":             bounds.H,
				"SDL.window.create.high_pixel_density": true,
			}
			if deferFullscreen {
				create["SDL.window.create.hidden"] = true
			} else {
				create["SDL.window.create.fullscreen"] = true
			}
			props, perr := sdl.NewProperties(create)
			if perr == nil {
				window, werr = sdl.CreateWindowWithProperties(props)
				props.Destroy()
			}
		}
		if window == nil {
			// Degraded path: the target display could not be resolved or the
			// properties could not be built. Open fullscreen wherever SDL picks
			// — losing display targeting, but still opening.
			deferFullscreen = false
			window, werr = sdl.CreateWindow(title, 0, 0, sdl.WINDOW_HIGH_PIXEL_DENSITY|sdl.WINDOW_FULLSCREEN)
		}
		if werr != nil {
			return nil, fmt.Errorf("apparatus.NewScreen: creating fullscreen window: %w", werr)
		}

		if deferFullscreen {
			// A display that reports no current mode is not fatal: fall through
			// to fullscreen-desktop on whatever output the creation properties
			// managed to reach.
			if mode, merr := target.CurrentDisplayMode(); merr == nil && mode != nil {
				_ = window.SetFullscreenMode(mode)
			}
			if err := window.SetFullscreen(true); err != nil {
				window.Destroy()
				return nil, fmt.Errorf("apparatus.NewScreen: entering fullscreen on display %d: %w", displayIndex, err)
			}
			if err := window.Show(); err != nil {
				window.Destroy()
				return nil, fmt.Errorf("apparatus.NewScreen: showing fullscreen window: %w", err)
			}
		}

		// Block until the window reaches its final state (fullscreen mode
		// applied, resize event processed). Required on KMS/DRM and Wayland
		// where state changes are deferred.
		if err := window.Sync(); err != nil {
			window.Destroy()
			return nil, fmt.Errorf("SyncWindow: %w", err)
		}

		// On Linux, prefer Vulkan over OpenGL. With NVIDIA proprietary drivers
		// on X11, the OpenGL renderer can silently render to a non-visible
		// framebuffer in fullscreen mode (blank screen or SIGSEGV in Present).
		// Vulkan + WSI handles fullscreen presentation correctly on all Linux
		// GPU vendors. SetHint has normal priority so SDL_RENDER_DRIVER in the
		// environment still overrides this (e.g. SDL_RENDER_DRIVER=software).
		if runtime.GOOS == "linux" {
			_ = sdl.SetHint(sdl.HINT_RENDER_DRIVER, "vulkan")
		}

		renderer, err := window.CreateRenderer("")
		if err != nil {
			window.Destroy()
			return nil, fmt.Errorf("apparatus.NewScreen: creating renderer: %w", err)
		}

		if err := renderer.SetVSync(1); err != nil {
			renderer.Destroy()
			window.Destroy()
			return nil, fmt.Errorf("apparatus.NewScreen: enabling vsync: %w", err)
		}

		// Warm-up: present several blank frames to flush driver state before
		// any experiment content is drawn.
		//   - KMS/DRM: SDL_RenderPresent submits an async drmModePageFlip;
		//     PumpEvents is required to process the completion event and switch
		//     the display from the text console to graphics mode.
		//   - Vulkan/OpenGL on X11: the first few presents may be discarded
		//     while the compositor or driver initialises the swap chain.
		//   - Wayland: PumpEvents is required to let the compositor assign the
		//     real window dimensions; window.Size() before this loop can return
		//     1×1.
		// 10 frames is conservative but still imperceptible (<200 ms at 60 Hz).
		for i := 0; i < 10; i++ {
			_ = renderer.SetDrawColor(bgColor.R, bgColor.G, bgColor.B, bgColor.A)
			_ = renderer.Clear()
			_ = renderer.Present()
			sdl.PumpEvents()
		}

		// Query the logical (OS) pixel dimensions after the warmup loop so that
		// the Wayland compositor has had a chance to assign the real window size.
		// On standard displays these equal the physical pixel dimensions; on HiDPI
		// displays (macOS Retina, some Linux setups) they are smaller by the
		// content-scale factor. We set a logical presentation matching this size
		// so all drawing commands operate in the logical coordinate space and SDL3
		// handles physical upscaling transparently.
		//
		// Fallback chain: window.Size() → display bounds → (0,0 — no presentation)
		logW, logH, err := window.Size()
		if err != nil || logW <= 1 || logH <= 1 {
			// Wayland with software renderer may still return 1×1 — fall back to
			// the display's reported bounds.
			if bounds, berr := target.Bounds(); berr == nil && bounds != nil && bounds.W > 1 {
				logW, logH = bounds.W, bounds.H
			} else {
				logW, logH = 0, 0
			}
		}

		if logW > 0 && logH > 0 {
			// STRETCH: no letterboxing — fullscreen always matches the display
			// aspect ratio, so stretch == letterbox in practice.
			if err := renderer.SetLogicalPresentation(logW, logH, sdl.LOGICAL_PRESENTATION_STRETCH); err != nil {
				renderer.Destroy()
				window.Destroy()
				return nil, fmt.Errorf("SetLogicalPresentation: %w", err)
			}
		}

		logicalSize := &sdl.FPoint{X: float32(logW), Y: float32(logH)}

		// In KMS/DRM mode (bare TTY), SDL3 has no desktop compositor to supply a
		// cursor shape — it must manage the cursor itself via a DRM cursor plane
		// or software compositing. Without an explicit cursor surface, ShowCursor()
		// makes "nothing" visible. Explicitly creating a system cursor gives SDL3
		// a shape to render in both hardware and software fallback paths.
		if cursor, err := sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_DEFAULT); err == nil {
			_ = sdl.SetCursor(cursor)
		}
		_ = sdl.ShowCursor()

		return &Screen{
			Window:      window,
			Renderer:    renderer,
			BgColor:     bgColor,
			Width:       int(logW),
			Height:      int(logH),
			LogicalSize: logicalSize,
		}, nil
	}

	// Windowed path: build window properties with x/y position set at creation
	// time so that the window manager sees the target-display hint before the
	// window is mapped. This is more reliable than a post-creation SetPosition
	// call, particularly on X11. On Wayland, compositors ignore app-supplied
	// positions for toplevel windows regardless of approach; fullscreen mode
	// (-d X without -w) is the reliable path there.
	props, err := sdl.NewProperties(map[string]any{
		"SDL.window.create.title":  title,
		"SDL.window.create.width":  int32(width),
		"SDL.window.create.height": int32(height),
		"SDL.window.create.hidden": true,
	})
	if err != nil {
		return nil, fmt.Errorf("create window properties: %w", err)
	}
	defer props.Destroy()

	if displayIndex != 0 {
		if bounds, err := target.Bounds(); err == nil && bounds != nil {
			x := bounds.X + (bounds.W-int32(width))/2
			y := bounds.Y + (bounds.H-int32(height))/2
			_ = props.SetNumberProperty("SDL.window.create.x", int64(x))
			_ = props.SetNumberProperty("SDL.window.create.y", int64(y))
		}
	}

	window, err := sdl.CreateWindowWithProperties(props)
	if err != nil {
		return nil, fmt.Errorf("apparatus.NewScreen: creating window: %w", err)
	}

	renderer, err := window.CreateRenderer("")
	if err != nil {
		window.Destroy()
		return nil, fmt.Errorf("apparatus.NewScreen: creating renderer: %w", err)
	}

	if err := renderer.SetVSync(1); err != nil {
		renderer.Destroy()
		window.Destroy()
		return nil, fmt.Errorf("apparatus.NewScreen: enabling vsync: %w", err)
	}

	s := &Screen{
		Window:   window,
		Renderer: renderer,
		BgColor:  bgColor,
		Width:    width,
		Height:   height,
	}

	if err := window.Show(); err != nil {
		renderer.Destroy()
		window.Destroy()
		return nil, fmt.Errorf("apparatus.NewScreen: showing window: %w", err)
	}

	// Ensure a cursor shape is loaded and visible (mirrors fullscreen path).
	if cursor, err := sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_DEFAULT); err == nil {
		_ = sdl.SetCursor(cursor)
	}
	_ = sdl.ShowCursor()

	return s, nil
}
