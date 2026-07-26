// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Package media provides multi-movie playback with shared master-clock
// synchronisation, look-ahead frame conditions, and post-vsync display
// events suitable for wiring to hardware triggers.
//
// See media/Plan.md for the full design and the PsyScope command
// equivalence table. The package is pure Go (no cgo) and supports only
// .gv movies, decoded via funatsufumiya/go-gv-video. Audio playback is
// out of scope.
package media

import (
	"github.com/chrplr/goxpyriment/media/present"
)

// OnsetSource indicates how an Onset's TimestampNS was obtained.
//
// The canonical type lives in media/present so that package can define
// the Timer interface without importing media (cycle-free). The alias
// here lets users keep writing media.VsyncEstimated etc.
type OnsetSource = present.OnsetSource

// VsyncEstimated marks an Onset whose TimestampNS is the post-Present
// SDL ticks value (apparatus.Screen.FlipTS). Accurate to within roughly
// one display scanout period (~16 ms at 60 Hz). Default when no
// HardwareVerified backend is attached to the MovieManager.
const VsyncEstimated = present.VsyncEstimated

// HardwareVerified marks an Onset whose TimestampNS is the OS-measured
// first-pixel-visible time, reported by macOS CVDisplayLink or Linux
// DRM_IOCTL_WAIT_VBLANK. Sub-millisecond precision in the common case;
// see media/present.Timer documentation for the caveats.
const HardwareVerified = present.HardwareVerified

// LookAhead marks an Onset fired from inside the per-frame decode loop,
// BEFORE the corresponding frame is presented. TimestampNS is the SDL
// ticks value at the moment the callback fired (NOT a vsync timestamp).
// Use look-ahead callbacks to set state that the same frame's
// compositor will read.
const LookAhead = present.LookAhead

// Onset describes a moment when a stimulus appeared, disappeared, or a
// movie reached a target frame, observed from the renderer's perspective.
//
// TimestampNS is in the same nanosecond reference frame as SDL3 event
// timestamps (sdl.TicksNS) and apparatus.Screen.FlipTS, so it can be
// subtracted from a KeyboardEvent.Timestamp to compute reaction times.
type Onset struct {
	// TimestampNS is the SDL ticks value at the relevant moment.
	TimestampNS uint64
	// Frame is the cumulative 1-based movie frame index involved
	// (0 if not applicable, e.g. a generic Display event for a
	// non-movie stimulus).
	Frame int
	// Source documents the precision of TimestampNS.
	Source OnsetSource
	// Movie is the originating movie, or nil for generic Display events
	// of a non-movie stimulus.
	Movie *Movie
	// Name is the stimulus name (movie tag for movies; user-supplied for
	// other stimuli registered via MovieManager.RegisterStimulusName).
	Name string
}
