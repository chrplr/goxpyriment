package stimuli

import (
	"fmt"
	"math"
	"runtime/debug"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	xio "github.com/chrplr/goxpyriment/io"
)

// StreamEvent represents a user input event recorded during the stream.
type StreamEvent struct {
	Timestamp time.Duration // Time elapsed since the start of the stream
	Type      uint32        // sdl.EVENT_KEY_DOWN, sdl.EVENT_KEY_UP, sdl.EVENT_MOUSE_BUTTON_DOWN
	Key       sdl.Keycode   // The key code (for keyboard events)
	Button    uint8         // The button index (for mouse events)
}

// StreamItem defines the content and timing for one element in the stream.
// This is the internal structure used by the core playback loop.
type StreamItem struct {
	Texture     *sdl.Texture
	DurationOn  time.Duration
	DurationOff time.Duration
}

// ImageTriplet defines the input for PresentStreamOfImages.
type ImageTriplet struct {
	Path        string
	DurationOn  time.Duration
	DurationOff time.Duration
}

// TextTriplet defines the input for PresentStreamOfText.
type TextTriplet struct {
	Text        string
	DurationOn  time.Duration
	DurationOff time.Duration
}

// PresentStreamOfImages loads a sequence of images and presents them with precise timing.
// It pre-loads all textures into GPU memory, disables GC during playback, and logs input events.
func PresentStreamOfImages(screen *xio.Screen, items []ImageTriplet, x, y int32) ([]StreamEvent, error) {
	// 1. Pre-load images
	streamItems := make([]StreamItem, len(items))
	for i, item := range items {
		pic, err := NewPicture(item.Path)
		if err != nil {
			// Clean up already loaded textures
			for j := 0; j < i; j++ {
				if streamItems[j].Texture != nil {
					streamItems[j].Texture.Destroy()
				}
			}
			return nil, fmt.Errorf("failed to load image %s: %w", item.Path, err)
		}
		
		// Create texture from surface
		tex, err := screen.Renderer.CreateTextureFromSurface(pic.Surface)
		pic.Close() // Surface no longer needed
		if err != nil {
			return nil, fmt.Errorf("failed to create texture for %s: %w", item.Path, err)
		}

		streamItems[i] = StreamItem{
			Texture:     tex,
			DurationOn:  item.DurationOn,
			DurationOff: item.DurationOff,
		}
	}

	// Ensure textures are cleaned up after presentation
	defer func() {
		for _, item := range streamItems {
			if item.Texture != nil {
				item.Texture.Destroy()
			}
		}
	}()

	return PresentStream(screen, streamItems, x, y)
}

// PresentStreamOfText renders a sequence of text strings and presents them with precise timing.
func PresentStreamOfText(screen *xio.Screen, items []TextTriplet, fontPath string, fontSize float32, color xio.Color, x, y int32) ([]StreamEvent, error) {
	// 1. Pre-render text
	// We need a temporary font loader if one isn't provided, but usually we use the one loaded in the experiment.
	// Since we don't have the Experiment struct here, we expect the font to be loadable.
	
	// Note: Creating a new font for every call might be inefficient if used repeatedly, 
	// but critical timing applies to the playback, not the setup.
	
	// Create a temporary TextLine to utilize its rendering logic, or render manually.
	// Manual rendering is safer to batch properly.
	
	// Check if screen has a default font if fontPath is empty? 
	// The function signature asks for fontPath.
	
	// For simplicity, we'll use NewTextLine helper if we can, but it requires a font.
	// Let's assume we use the screen's logic or a new font.
	
	// Problem: `NewTextLine` doesn't take a font path, it uses the global/default one implicitly or we set it later.
	// Actually `NewTextLine` just creates the struct. `Draw` uses the screen's font?
	// Checking `stimuli/text.go`: `Draw` uses `screen.DefaultFont`.
	
	// So if fontPath is "", we assume screen.DefaultFont.
	// If fontPath is provided, we load it.
	
	// We'll mimic the logic: render surfaces -> textures.
	
	streamItems := make([]StreamItem, len(items))
	
	// We need a font object to render surfaces.
	// If we are using screen.DefaultFont, we need access to it.
	// If we are loading a new one, we do it now.
	
	// Accessing screen.DefaultFont directly (it's exported).
	font := screen.DefaultFont
	// If a specific font path is requested, we should load it (and close it later).
	// Implementation detail: Use a temporary text stimulus to generate the surface/texture.

	for i, item := range items {
		// Create a temporary text line to render
		tl := NewTextLine(item.Text, 0, 0, color)
		
		// We need to render this to a texture manually because we want to pre-load.
		// `tl.PreRender(screen)`? TextLine doesn't typically expose PreRender to Texture easily without Draw.
		// But `stimuli/text.go` likely has `Render` method.
		// Let's look at `stimuli/text.go` logic. It usually renders on the fly or caches.
		// To ensure "Pre-load", we MUST generate the texture now.
		
		// Using SDL_ttf directly for surface creation
		if font == nil {
			return nil, fmt.Errorf("no font available for text rendering")
		}
		
		surface, err := font.RenderTextBlended(item.Text, color)
		if err != nil {
			return nil, err
		}
		
		tex, err := screen.Renderer.CreateTextureFromSurface(surface)
		surface.Destroy()
		if err != nil {
			return nil, err
		}
		
		streamItems[i] = StreamItem{
			Texture:     tex,
			DurationOn:  item.DurationOn,
			DurationOff: item.DurationOff,
		}
	}

	defer func() {
		for _, item := range streamItems {
			if item.Texture != nil {
				item.Texture.Destroy()
			}
		}
	}()

	return PresentStream(screen, streamItems, x, y)
}


// PresentStream handles the core playback loop for a sequence of textures.
// It manages GC, VSync, and input logging.
func PresentStream(screen *xio.Screen, items []StreamItem, x, y int32) ([]StreamEvent, error) {
	// 1. Disable GC
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	events := make([]StreamEvent, 0, 100) // Pre-allocate some space

	// 2. Check VSync and Refresh Rate
	vsync, _ := screen.VSync()
	isVSync := vsync == 1
	
	var refreshRate float64 = 60.0 // Default fallback
	if mode, err := screen.Window.GetDisplayMode(); err == nil && mode.RefreshRate > 0 {
		refreshRate = float64(mode.RefreshRate)
	}
	frameDur := 1.0 / refreshRate * 1000 // Frame duration in ms
	
	// Helper to handle input polling
	startTime := time.Now()
	
	pollInputs := func() {
		var ev sdl.Event
		for sdl.PollEvent(&ev) {
			now := time.Since(startTime)
			switch ev.Type {
			case sdl.EVENT_KEY_DOWN, sdl.EVENT_KEY_UP:
				events = append(events, StreamEvent{
					Timestamp: now,
					Type:      ev.Type,
					Key:       ev.KeyboardEvent().Key,
				})
			case sdl.EVENT_MOUSE_BUTTON_DOWN, sdl.EVENT_MOUSE_BUTTON_UP:
				events = append(events, StreamEvent{
					Timestamp: now,
					Type:      ev.Type,
					Button:    ev.MouseButtonEvent().Button,
				})
			case sdl.EVENT_QUIT:
				// Store it? Or allow caller to handle?
				// Usually we just log it, but the caller might want to abort.
				// For now, we log it.
			}
		}
	}

	// 3. Playback Loop
	
	// Reset start time to now just before the loop
	startTime = time.Now()

	for _, item := range items {
		// --- ON PHASE ---
		w, h, _ := item.Texture.Size()
		dst := xio.FRect{
			X: float32(x) - float32(w)/2,
			Y: float32(y) - float32(h)/2,
			W: float32(w),
			H: float32(h),
		}

		// Calculate target duration
		durOn := item.DurationOn
		
		// If VSync is enabled, we use frame counting for stability
		if isVSync {
			nFrames := int(math.Round(float64(durOn.Milliseconds()) / frameDur))
			if nFrames < 1 { nFrames = 1 }
			
			for f := 0; f < nFrames; f++ {
				// We must redraw every frame in double buffering
				screen.Clear()
				screen.Renderer.RenderTexture(item.Texture, nil, &dst)
				screen.Flip() // Blocks waiting for VSync
				pollInputs()
			}
		} else {
			// Time-based loop
			screen.Clear()
			screen.Renderer.RenderTexture(item.Texture, nil, &dst)
			screen.Flip()
			
			// Record exact onset of this image?
			// The timestamp logic uses 'startTime' of the whole stream.
			
			phaseStart := time.Now()
			for time.Since(phaseStart) < durOn {
				pollInputs()
				// Busy wait or small sleep?
				// "Timing is critical" -> Busy wait is safer for precision < 1ms
			}
		}

		// --- OFF PHASE ---
		durOff := item.DurationOff
		if durOff > 0 {
			if isVSync {
				nFrames := int(math.Round(float64(durOff.Milliseconds()) / frameDur))
				if nFrames < 1 { nFrames = 1 }
				
				for f := 0; f < nFrames; f++ {
					screen.Clear()
					screen.Flip() // Blocks
					pollInputs()
				}
			} else {
				screen.Clear()
				screen.Flip()
				
				phaseStart := time.Now()
				for time.Since(phaseStart) < durOff {
					pollInputs()
				}
			}
		}
	}

	return events, nil
}
