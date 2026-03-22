// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"fmt"
	"log"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
)

func main() {
	// Initialize the SDL3 library (extracts and loads the embedded binary)
	sdlLoader := binsdl.Load()
	defer sdlLoader.Unload()

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		log.Fatalf("SDL_Init Error: %v", err)
	}
	defer sdl.Quit()

	// 1. Create window with High-DPI and Fullscreen flags
	// Using 0,0 for width/height with FULLSCREEN often defaults to native desktop res
	window, err := sdl.CreateWindow("Physical Res Demo", 0, 0,
		sdl.WINDOW_HIGH_PIXEL_DENSITY|sdl.WINDOW_FULLSCREEN)
	if err != nil {
		log.Fatalf("CreateWindow Error: %v", err)
	}
	defer window.Destroy()

	renderer, err := window.CreateRenderer("")
	if err != nil {
		log.Fatalf("CreateRenderer Error: %v", err)
	}
	defer renderer.Destroy()

	// 2. Query the actual physical pixel dimensions
	physW, physH, err := window.SizeInPixels()
	if err != nil {
		fmt.Printf("SizeInPixels Error: %v\n", err)
	}

	// Get refresh rate
	refreshRate := float32(0)
	displayID := sdl.GetDisplayForWindow(window)
	if displayID != 0 {
		mode, err := displayID.CurrentDisplayMode()
		if err == nil {
			refreshRate = mode.RefreshRate
		}
	}

	// 3. Get the scale factor (Density) to fix input coordinates
	pixelDensity, err := window.PixelDensity()
	if err != nil {
		fmt.Printf("PixelDensity Error: %v\n", err)
	}

	running := true
	for running {
		var event sdl.Event
		for sdl.PollEvent(&event) {
			switch event.Type {
			case sdl.EVENT_QUIT:
				running = false
			case sdl.EVENT_KEY_DOWN:
				if event.KeyboardEvent().Key == control.K_ESCAPE {
					running = false
				}
			}
		}

		// --- INPUT MAPPING ---
		_, logicalMouseX, logicalMouseY := sdl.GetMouseState()

		// Map logical mouse coordinates to physical pixels
		physMouseX := logicalMouseX * pixelDensity
		physMouseY := logicalMouseY * pixelDensity

		// --- RENDERING ---
		renderer.SetDrawColor(20, 20, 20, 255) // Dark Gray
		renderer.Clear()

		// Draw a 1-pixel thick border around the physical edge
		renderer.SetDrawColor(255, 255, 255, 255)

		// Display info
		infoStr := fmt.Sprintf("Bypassing Scaling | Res: %dx%d | Refresh: %.2fHz", physW, physH, refreshRate)
		renderer.DebugText(20, 20, infoStr)

		// Draw center cross
		centerX, centerY := float32(physW)/2, float32(physH)/2
		crossSize := float32(20)
		renderer.RenderLine(centerX-crossSize, centerY, centerX+crossSize, centerY) // Horizontal
		renderer.RenderLine(centerX, centerY-crossSize, centerX, centerY+crossSize) // Vertical

		// Draw a small square at the physical mouse position
		mouseRect := control.FRect{
			X: physMouseX - 5,
			Y: physMouseY - 5,
			W: 10,
			H: 10,
		}
		renderer.RenderFillRect(&mouseRect)

		// Also draw a border around the physical edge
		screenRect := control.FRect{
			X: 0,
			Y: 0,
			W: float32(physW),
			H: float32(physH),
		}
		renderer.RenderRect(&screenRect)

		renderer.Present()
	}
}
