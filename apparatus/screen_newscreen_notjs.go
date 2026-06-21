// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

//go:build !js

package apparatus

import (
	"fmt"
	"runtime"

	"github.com/Zyko0/go-sdl3/sdl"
)

// displayByIndex resolves a 0-based display index to an SDL DisplayID.
// Index 0 always refers to the primary display.
// Returns an error if index is out of range.
func displayByIndex(index int) (sdl.DisplayID, error) {
	if index == 0 {
		return sdl.GetPrimaryDisplay(), nil
	}
	displays, err := sdl.GetDisplays()
	if err != nil {
		return 0, fmt.Errorf("enumerate displays: %w", err)
	}
	if index < 0 || index >= len(displays) {
		return 0, fmt.Errorf("display index %d out of range [0, %d)", index, len(displays))
	}
	return displays[index], nil
}

// ListDisplays returns metadata for all connected displays, ordered so that
// index 0 is the primary display. Pass an index to NewScreen (or set
// Experiment.ScreenNumber) to open the window on a specific monitor.
func ListDisplays() ([]DisplayInfo, error) {
	displays, err := sdl.GetDisplays()
	if err != nil {
		return nil, fmt.Errorf("enumerate displays: %w", err)
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
		// For secondary displays, embed the target display's position in the
		// creation properties instead of calling SetPosition after the window
		// is already fullscreen. Post-creation SetPosition on a fullscreen
		// window can crash the display manager on some compositors (KDE/GNOME
		// on X11). The windowed path uses the same property-at-creation approach.
		var window *sdl.Window
		var werr error
		if displayIndex != 0 {
			if bounds, berr := target.Bounds(); berr == nil && bounds != nil {
				props, perr := sdl.NewProperties(map[string]any{
					"SDL.window.create.title":              title,
					"SDL.window.create.x":                  bounds.X,
					"SDL.window.create.y":                  bounds.Y,
					"SDL.window.create.width":              bounds.W,
					"SDL.window.create.height":             bounds.H,
					"SDL.window.create.fullscreen":         true,
					"SDL.window.create.high_pixel_density": true,
				})
				if perr == nil {
					window, werr = sdl.CreateWindowWithProperties(props)
					props.Destroy()
				}
			}
		}
		if window == nil {
			window, werr = sdl.CreateWindow(title, 0, 0, sdl.WINDOW_HIGH_PIXEL_DENSITY|sdl.WINDOW_FULLSCREEN)
		}
		if werr != nil {
			return nil, fmt.Errorf("apparatus.NewScreen: creating fullscreen window: %w", werr)
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
		//     real window dimensions; window.Size() before this loop returns 1×1
		//     (e.g. software renderer on Raspberry Pi).
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
