// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

// get-display-info queries every connected display via SDL3 and prints its
// properties: name, bounds, current mode, desktop mode, content scale,
// orientation, and the full list of available fullscreen resolutions.
//
// Usage:
//
//	go run ./cmd/get-display-info
package main

import (
	"fmt"
	"log"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"
)

func orientationName(o sdl.DisplayOrientation) string {
	switch o {
	case sdl.ORIENTATION_LANDSCAPE:
		return "landscape"
	case sdl.ORIENTATION_LANDSCAPE_FLIPPED:
		return "landscape (flipped)"
	case sdl.ORIENTATION_PORTRAIT:
		return "portrait"
	case sdl.ORIENTATION_PORTRAIT_FLIPPED:
		return "portrait (flipped)"
	default:
		return "unknown"
	}
}

func printMode(label string, m *sdl.DisplayMode) {
	if m == nil {
		fmt.Printf("    %-16s  (unavailable)\n", label)
		return
	}
	details := ""
	if d, err := m.Format.Details(); err == nil && d != nil {
		details = fmt.Sprintf("  %d bpp  (%db/ch)", d.BitsPerPixel, d.Rbits)
	}
	fmt.Printf("    %-16s  %4d × %-4d  %7.3f Hz  density=%.2f  fmt=%s%s\n",
		label,
		m.W, m.H,
		m.RefreshRate,
		m.PixelDensity,
		m.Format.Name(),
		details,
	)
}

func main() {
	lib := binsdl.Load()
	defer lib.Unload()

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		log.Fatalf("SDL init: %v", err)
	}
	defer sdl.Quit()

	displays, err := sdl.GetDisplays()
	if err != nil {
		log.Fatalf("GetDisplays: %v", err)
	}

	primary := sdl.GetPrimaryDisplay()
	fmt.Printf("Connected displays: %d\n\n", len(displays))

	for _, id := range displays {
		marker := ""
		if id == primary {
			marker = "  [primary]"
		}

		name, _ := id.Name()
		fmt.Printf("Display %d — %q%s\n", uint32(id), name, marker)
		fmt.Printf("  %s\n", "─────────────────────────────────────────────────────────────────────")

		// Desktop area and usable area.
		if b, err := id.Bounds(); err == nil {
			fmt.Printf("  Desktop bounds:   %d,%d  %d×%d\n", b.X, b.Y, b.W, b.H)
		}
		if u, err := id.UsableBounds(); err == nil {
			fmt.Printf("  Usable bounds:    %d,%d  %d×%d\n", u.X, u.Y, u.W, u.H)
		}

		// Content / DPI scale.
		if scale, err := id.ContentScale(); err == nil {
			fmt.Printf("  Content scale:    %.2f\n", scale)
		}

		// Orientation.
		fmt.Printf("  Orientation:      %s  (natural: %s)\n",
			orientationName(id.CurrentDisplayOrientation()),
			orientationName(id.NaturalDisplayOrientation()),
		)

		// Current and desktop modes.
		fmt.Println("  Modes:")
		current, _ := id.CurrentDisplayMode()
		desktop, _ := id.DesktopDisplayMode()
		printMode("current", current)
		printMode("desktop", desktop)

		// All available fullscreen resolutions.
		modes, err := id.FullscreenDisplayModes()
		if err != nil {
			fmt.Printf("  (could not enumerate fullscreen modes: %v)\n", err)
		} else {
			fmt.Printf("  Fullscreen modes: %d available\n", len(modes))
			for _, m := range modes {
				printMode("", m)
			}
		}
		fmt.Println()
	}
}
