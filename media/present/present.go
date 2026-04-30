// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Package present provides PresentTimer backends that report
// hardware-verified vsync timestamps for an apparatus.Screen.
//
// A Timer answers, for a given Present-return time (FlipTS), what the
// OS-measured first-pixel-visible time was for the frame whose pixels
// were just queued. Three implementations ship today:
//
//   - vsync-estimated fallback: returns FlipTS itself, no OS integration
//     (precision: ~one display scanout period, ~16 ms at 60 Hz).
//   - macOS CVDisplayLink (cvdisplaylink_darwin.go): publishes per-vsync
//     timestamps from the CoreVideo display link, accurate to whatever
//     macOS measures from the display controller.
//   - Linux DRM (drm_linux.go): queries DRM_IOCTL_WAIT_VBLANK after each
//     Present, accurate to whatever the kernel stamps the vblank IRQ
//     with.
//
// All three implementations are pure Go (no cgo). The macOS backend
// uses purego to load CoreVideo and libSystem; the Linux backend uses
// the syscall package for the DRM ioctl. Stage 5 of media/Plan.md.
//
// Use AutoDetect to pick the best Timer for the current platform; it
// falls back to vsync-estimated if the platform-specific backend can't
// initialise (missing display, permission denied, etc.).
package present

import (
	"github.com/chrplr/goxpyriment/apparatus"
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

// fallback is the always-available Timer that simply returns FlipTS
// itself with VsyncEstimated source.
type fallback struct{}

// NewFallback returns the no-OS-integration Timer. Precision:
// VsyncEstimated (~vsync-period). Always available, never errors.
func NewFallback() Timer { return fallback{} }

func (fallback) RecordFlip(uint64) {}

func (fallback) OnsetForFlip(flipTS uint64) (uint64, OnsetSource, bool) {
	return flipTS, VsyncEstimated, true
}

func (fallback) Precision() OnsetSource { return VsyncEstimated }
func (fallback) Close() error           { return nil }
func (fallback) Description() string {
	return "vsync-estimated (post-Present FlipTS, no OS integration)"
}

// AutoDetect picks the best available Timer for the current platform
// and screen. Falls back to NewFallback() if no platform-specific
// backend is available or if its initialisation fails.
//
// On macOS: tries CVDisplayLink via CoreVideo (purego).
// On Linux:  tries DRM_IOCTL_WAIT_VBLANK on /dev/dri/cardX.
// Elsewhere: returns NewFallback().
//
// The return value is never nil. Failures are logged via the standard
// log package and surfaced through the returned Timer's Description.
func AutoDetect(screen *apparatus.Screen) Timer {
	return autoDetect(screen)
}
