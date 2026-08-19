// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Package present adapts the vblank clock to MovieManager's presentation model.
//
// The backends used to live here. They now live in the leaf package
// github.com/chrplr/goxpyriment/vblank, because apparatus.Screen needs them and
// this package imports apparatus — the dependency ran the wrong way for the one
// caller that needs it most. Nothing was lost in the move: the backends never
// used the *apparatus.Screen they were handed.
//
// What stays here is what is specific to presenting movies rather than to
// reading a vblank clock: the LookAhead source, which describes a callback that
// fires from the decode loop BEFORE the frame is presented and therefore carries
// no vsync timestamp at all. That distinction is meaningless to a vblank clock
// and would be a semantic leak in the leaf package, so the two vocabularies are
// kept apart and translated at this seam.
//
// The public API is unchanged. Use AutoDetect to pick the best Timer; it never
// returns nil.
package present

import (
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/vblank"
)

// OnsetSource indicates the precision class of an Onset timestamp.
//
// VsyncEstimated: post-Present FlipTS only. Accurate to within roughly
// one display scanout period (~16 ms at 60 Hz). The default when no
// HardwareVerified backend is available.
//
// HardwareVerified: OS-measured first-pixel-visible time. Accurate to
// whatever the display controller / vblank IRQ stamps it with, typically
// well under 1 ms. Subject to compositor latency: see Plan.md §8.
//
// LookAhead: pre-vsync, fired from the per-frame decode loop in
// MovieManager.DrawWithoutFlip. TimestampNS is the SDL ticks at fire
// time (NOT a vsync timestamp).
type OnsetSource int

const (
	VsyncEstimated OnsetSource = iota
	HardwareVerified
	LookAhead
)

// String returns a stable human-readable form for logs.
func (s OnsetSource) String() string {
	switch s {
	case VsyncEstimated:
		return "vsync-estimated"
	case HardwareVerified:
		return "hardware-verified"
	case LookAhead:
		return "look-ahead"
	default:
		return "unknown"
	}
}

// fromVblank maps a vblank.Source onto the richer OnsetSource.
//
// The two enumerations agree numerically today, but the mapping is written out
// rather than cast: LookAhead exists only on this side, so the sets are not the
// same shape and a cast would silently do the wrong thing the moment either one
// gains a member.
func fromVblank(s vblank.Source) OnsetSource {
	switch s {
	case vblank.HardwareVerified:
		return HardwareVerified
	default:
		return VsyncEstimated
	}
}

// Timer reports the OS-measured display-refresh time most likely to
// correspond to a given Present(). Implementations are safe for
// concurrent use; backends that publish vsync timestamps from a
// background OS callback (CVDisplayLink) handle their own locking.
type Timer interface {
	// RecordFlip is called by MovieManager immediately after each
	// FlipTS. Backends that need a synchronous query (Linux DRM) use
	// this hook; backends that publish asynchronously (macOS
	// CVDisplayLink) treat it as a no-op.
	RecordFlip(flipTS uint64)

	// OnsetForFlip returns the OS-measured first-pixel-visible time for
	// a frame whose Present returned at flipTS (in SDL ticks).
	//
	// Returns ok=false if no precise estimate is available yet — the
	// caller should defer the consumer's callback to a future
	// NotifyFlipped (the OS is expected to publish the next vsync
	// within ~1 frame period) and retry.
	OnsetForFlip(flipTS uint64) (timestamp uint64, source OnsetSource, ok bool)

	// Precision returns the OnsetSource that this timer reports for
	// ok=true outcomes. Used by MovieManager to decide whether to emit
	// the one-time precision warning.
	Precision() OnsetSource

	// Close releases any background resources (CVDisplayLink callback
	// registration, DRM file descriptor). Idempotent.
	Close() error

	// Description returns a one-line human-readable identifier for logs.
	Description() string
}

// adapter presents a vblank.Timer through this package's Timer interface.
type adapter struct{ t vblank.Timer }

func (a adapter) RecordFlip(flipTS uint64) { a.t.RecordFlip(flipTS) }

func (a adapter) OnsetForFlip(flipTS uint64) (uint64, OnsetSource, bool) {
	ts, src, ok := a.t.OnsetForFlip(flipTS)
	return ts, fromVblank(src), ok
}

func (a adapter) Precision() OnsetSource { return fromVblank(a.t.Precision()) }
func (a adapter) Close() error           { return a.t.Close() }
func (a adapter) Description() string    { return a.t.Description() }

// NewFallback returns the no-OS-integration Timer. Precision:
// VsyncEstimated (~vsync-period). Always available, never errors.
func NewFallback() Timer { return adapter{vblank.NewFallback()} }

// AutoDetect picks the best available Timer for the current platform.
// Falls back to NewFallback() if no platform-specific backend is
// available or if its initialisation fails.
//
// On macOS: tries CVDisplayLink via CoreVideo (purego).
// On Linux:  tries DRM_IOCTL_WAIT_VBLANK, searching every /dev/dri/cardN and
// selecting the CRTC that drives the screen's display.
// Elsewhere: returns NewFallback().
//
// The screen argument names the display to time. It used to be unused — no
// backend needed it — which was true right up until a laptop with an external
// monitor showed that "which display" is the question a vblank clock has to
// answer first. A nil screen still works and leaves the backend to guess.
//
// The return value is never nil. Failures are logged via the standard
// log package and surfaced through the returned Timer's Description.
func AutoDetect(screen *apparatus.Screen) Timer {
	var t vblank.Target
	if screen != nil {
		t = screen.VblankTarget()
	}
	return adapter{vblank.AutoDetectFor(t)}
}
