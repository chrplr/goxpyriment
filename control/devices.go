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

	"github.com/Zyko0/go-sdl3/sdl"

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

// writeVblank reports whether this machine can measure a frame's onset, or only
// estimate it.
//
// It is here so the question can be answered BEFORE committing to a capture. The
// backend otherwise appears only in a run's -info.txt, which means a photodiode
// session discovers it has no vblank clock after spending the eight minutes —
// and an A/B whose two arms both ran estimated looks like a null result rather
// than a machine that never had the hardware.
//
// Unlike the rest of this file it needs no SDL: the DRM ioctl and CVDisplayLink
// go straight to the OS, and no window has to exist. It probes and closes
// immediately, so nothing is held when the experiment later opens its own.
func writeVblank(b *strings.Builder) {
	if vblank.Disabled() {
		section(b, "Vblank", []string{
			"disabled by " + vblank.EnvDisable + "=off — onsets will be ESTIMATED",
		})
		return
	}

	t := vblank.AutoDetect()
	defer t.Close() //nolint:errcheck // a probe's close has nothing to report

	rows := []string{t.Description()}
	if t.Precision() != vblank.HardwareVerified {
		// Say what it costs, not just that it happened: an estimated onset can
		// be wrong by an amount only a photodiode can establish, and it drifts
		// rather than scattering, so no host-side statistic will show it.
		rows = append(rows,
			"NO hardware vblank clock: FlipTS onsets are estimated, not measured")
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
