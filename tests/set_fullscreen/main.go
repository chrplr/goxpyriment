package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	// 0. Load SDL3 binary
	defer binsdl.Load().Unload()

	// 1. Initialize SDL Video
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		fmt.Printf("sdl.Init failed: %v\n", err)
		os.Exit(-1)
	}
	defer sdl.Quit()

	// 2. Determine native resolution before creating window
	display := sdl.GetPrimaryDisplay()
	if display == 0 {
		fmt.Println("Could not get primary displayID, trying GetDisplays")
		displays, err := sdl.GetDisplays()
		if err != nil || len(displays) == 0 {
			fmt.Printf("Failed to get displays: %v\n", err)
			return
		}
		display = displays[0]
	}

	desktopMode, err := display.DesktopDisplayMode()
	if err != nil {
		fmt.Printf("Could not get desktop display mode: %v\n", err)
		return
	}

	fmt.Printf("Native resolution: %dx%d @ %.2fHz\n", desktopMode.W, desktopMode.H, desktopMode.RefreshRate)

	// 3. Create window directly in fullscreen at native resolution
	// This avoids the flicker of a windowed mode window with a title bar.
	window, err := sdl.CreateWindow("SDL3 Exclusive Fullscreen (Go)", int(desktopMode.W), int(desktopMode.H), sdl.WINDOW_FULLSCREEN)
	if err != nil {
		fmt.Printf("Window creation failed: %v\n", err)
		return
	}
	defer window.Destroy()

	// 4. Set specific exclusive mode behavior
	if err := window.SetFullscreenMode(desktopMode); err != nil {
		fmt.Printf("Could not set window fullscreen mode: %v\n", err)
	}

	// 5. Ensure fullscreen (optional here since it was in flags, but good for completeness)
	if err := window.SetFullscreen(true); err != nil {
		fmt.Printf("Could not set fullscreen: %v\n", err)
	}

	// Main Loop
	fmt.Println("Running... Press ESC to quit.")
	running := true
	for running {
		var ev sdl.Event
		for sdl.PollEvent(&ev) {
			switch ev.Type {
			case sdl.EVENT_QUIT:
				running = false
			case sdl.EVENT_KEY_DOWN:
				if ev.KeyboardEvent().Key == sdl.K_ESCAPE {
					running = false
				}
			}
		}
		sdl.Delay(10)
	}
}
