// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

// SDL-level device inventory: the displays and audio outputs as SDL sees them.
//
// This deliberately lives here rather than in sysinfo/. That package reads the
// OS (/proc, lspci, WMIC) and stays free of SDL, so it can be collected without
// initialising anything. What an experiment actually needs to know — which
// monitor index -d selects, which audio device will be opened — is only
// answerable through SDL, and requires SDL to be initialised first.

import (
	"fmt"
	"strings"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/vblank"
)

// DevicesString returns a printable inventory of the displays and audio
// playback devices SDL can see, formatted to sit alongside sysinfo's output.
//
// It initialises and tears down SDL itself, so it must NOT be called on a live
// experiment — use it before any Experiment is created (as -sysinfo does), or
// the second sdl.Init will fight the first over device ownership.
//
// Errors are reported inline rather than returned: a machine with no sound card
// should still print its displays, and vice versa.
func DevicesString() string {
	sdlLib := loadSDL()
	defer sdlLib.Unload()

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_AUDIO); err != nil {
		return fmt.Sprintf("Displays:   (unavailable: sdl.Init: %v)\n", err)
	}
	defer sdl.Quit()

	var b strings.Builder
	writeDisplays(&b)
	writeAudioOutputs(&b)
	writeVblank(&b)
	return b.String()
}

// PrintDevices writes DevicesString() to stdout.
func PrintDevices() { fmt.Print(DevicesString()) }

// FrameDurationOfDisplay reports the frame period of display `index` (0 = the
// primary, the same ordering ListDisplays uses), initialising and shutting down
// SDL video around the query.
//
// It exists for one-shot use before an Experiment exists — a harness sizing a
// capture window, or deciding a tone length, without being told the refresh rate
// by a human. That used to be a REFRESH_HZ variable in run-timing-tests.sh and a
// -hz flag here, and between them they produced two typos in two days: 60.197,
// which is a valid float and silently shortened every tone, and 60.0.197, which
// is not and cost a nine-minute session.
//
// Do NOT call it while an Experiment is running: it calls sdl.Init and sdl.Quit
// itself, and the second Init fights the first over device ownership.
func FrameDurationOfDisplay(index int) (time.Duration, error) {
	sdlLib := loadSDL()
	defer sdlLib.Unload()
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		return 0, fmt.Errorf("control: sdl.Init: %w", err)
	}
	defer sdl.Quit()
	displays, err := sdl.GetDisplays()
	if err != nil {
		return 0, fmt.Errorf("control: enumerating displays: %w", err)
	}
	if index < 0 || index >= len(displays) {
		return 0, fmt.Errorf("control: display %d out of range (%d connected)", index, len(displays))
	}
	return apparatus.FrameDurationForDisplay(displays[index]), nil
}

// section renders label + rows in the two-column layout sysinfo uses, so the
// combined output reads as one report rather than two stapled together.
func section(b *strings.Builder, label string, rows []string) {
	const width = 11
	indent := strings.Repeat(" ", width+1)
	for i, row := range rows {
		if i == 0 {
			fmt.Fprintf(b, "%-*s %s\n", width, label+":", row)
		} else {
			fmt.Fprintf(b, "%s%s\n", indent, row)
		}
	}
}

func writeDisplays(b *strings.Builder) {
	displays, err := ListDisplays()
	if err != nil {
		section(b, "Displays", []string{fmt.Sprintf("(unavailable: %v)", err)})
		return
	}
	if len(displays) == 0 {
		section(b, "Displays", []string{"(none detected)"})
		return
	}

	rows := make([]string, 0, len(displays)+1)
	for i, d := range displays {
		name := d.Name
		if name == "" {
			name = "(unnamed)"
		}
		// The leading index is the whole point of this section: it is the value
		// to pass to -d. ListDisplays orders the primary first, so index 0 is
		// always the primary — which is NOT necessarily the monitor an
		// experiment should run on.
		row := fmt.Sprintf("[%d] %-22s %dx%d  %.3f Hz  bounds %d,%d %dx%d",
			i, name, d.NativeW, d.NativeH, d.RefreshRate,
			d.BoundsX, d.BoundsY, d.BoundsW, d.BoundsH)
		if d.ContentScale > 0 && d.ContentScale != 1 {
			row += fmt.Sprintf("  scale %.2f", d.ContentScale)
		}
		if i == 0 {
			row += "  [primary]"
		}
		rows = append(rows, row)
	}
	rows = append(rows, fmt.Sprintf("video driver: %s   (the [N] above is the -d N value)",
		sdl.GetCurrentVideoDriver()))
	section(b, "Displays", rows)
}

// writeVblank reports where a frame's onset timestamp will come from, and
// whether a kernel vblank clock is available should you opt into one.
//
// It reports both, always, because they are independent: the opt-in can be set
// on a machine with no backend, and a machine can have a perfectly good backend
// that no run is using. Answering only one of the two leaves a capture unable to
// say afterwards which arm it was.
//
// It is here so the question can be settled BEFORE committing to a capture —
// otherwise a photodiode session discovers it has no vblank clock after spending
// the eight minutes, and an A/B whose two arms ran identically looks like a null
// result rather than a switch that never took.
//
// Unlike the rest of this file it needs no SDL: the DRM ioctl and CVDisplayLink
// go straight to the OS, and no window has to exist. It probes and closes
// immediately, so nothing is held when the experiment later opens its own.
func writeVblank(b *strings.Builder) {
	t := vblank.AutoDetect()
	defer t.Close() //nolint:errcheck // a probe's close has nothing to report

	available := t.Precision() == vblank.HardwareVerified
	var rows []string
	// The default is not one source but whichever the driver leaves available
	// each frame, so it is described that way: saying "the pacing schedule"
	// misreports every machine where SDL_RenderPresent blocks, which is most of
	// them. The run's own Frame pacing block says which it turned out to be.
	//
	// Spelled out rather than summarised. This line is read by someone deciding
	// whether to trust a latency, and the short version -- "onsets come from the
	// present's return" -- assumes the reader already knows that a blocking
	// present returns AT the retrace, which is the entire point being made.
	dflt := []string{
		"onsets are timestamped the moment SDL_RenderPresent() returns.",
		"Where the driver blocks on the retrace, that moment IS the retrace,",
		"so the timestamp is a hardware instant. On a frame where the present",
		"returned early instead, the timestamp comes from the pacing schedule.",
	}
	lead := func(prefix string) []string {
		out := append([]string(nil), dflt...)
		out[0] = prefix + out[0]
		return out
	}
	switch {
	case vblank.Enabled() && available:
		rows = append(rows, "IN USE (opt-in): "+t.Description())
	case vblank.Enabled():
		rows = append(rows, vblank.EnvOptIn+"=on, but no hardware vblank clock on this machine")
		rows = append(rows, lead("falling back: ")...)
	case available:
		rows = append(rows, lead("default: ")...)
		rows = append(rows, "available if asked for with "+vblank.EnvOptIn+"=on: "+t.Description())
	default:
		rows = append(rows, lead("default: ")...)
		rows = append(rows, "no hardware vblank clock on this machine")
	}
	section(b, "Vblank", rows)
}

func writeAudioOutputs(b *strings.Builder) {
	devices, err := sdl.GetAudioPlaybackDevices()
	if err != nil {
		section(b, "Audio out", []string{fmt.Sprintf("(unavailable: %v)", err)})
		return
	}

	rows := make([]string, 0, len(devices)+1)
	for i, id := range devices {
		name, nerr := id.Name()
		if nerr != nil || name == "" {
			name = "(unnamed)"
		}
		rows = append(rows, fmt.Sprintf("[%d] %s", i, name))
	}
	if len(rows) == 0 {
		rows = append(rows, "(no playback devices)")
	}
	// The driver matters more than the device list for timing: it is what
	// SDL_AUDIODRIVER selects, and the tests pin it deliberately.
	rows = append(rows, fmt.Sprintf("driver: %s", sdl.GetCurrentAudioDriver()))
	section(b, "Audio out", rows)
}
