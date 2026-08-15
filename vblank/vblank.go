// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Package vblank reports the OS-measured time of a display's vertical blank.
//
// # What it is for
//
// A flip timestamp taken in user space says when the program handed a frame
// over, not when the display accepted it. Where SDL_RenderPresent does not block
// to the retrace — which is most configurations measured here — the two are
// different, and the gap is not constant: it walks at whatever rate the
// framework's idea of the refresh differs from the panel's real one. Measured on
// a Raspberry Pi 4, that put the flip timestamps 14 ms away from the photons
// over an eight-minute run.
//
// The kernel stamps every vertical blank, and that stamp is an independent clock
// on the display itself — the role a photodiode plays, minus the photons.
// Against a BBTK photodiode on a Precision 5490 the two agreed to 1.3 ppm.
//
// # Why it is a leaf package
//
// This lives below apparatus rather than inside media so that apparatus.Screen
// can use it. It was previously part of media/present, which imports apparatus,
// so the dependency ran the wrong way for the one caller that needs it most.
// Nothing here imports apparatus; the backends never used the Screen they were
// once handed.
//
// # Backends
//
// All are pure Go, no cgo.
//
//   - Linux: DRM_IOCTL_WAIT_VBLANK via syscall on /dev/dri/cardN (drm_linux.go).
//   - macOS: CVDisplayLink via purego (cvdisplaylink_darwin.go).
//   - Elsewhere: the fallback, which echoes the flip timestamp and reports
//     Estimated so a caller can tell it apart from a real measurement.
//
// Use AutoDetect; it never returns nil.
//
// # What it does not tell you
//
// A vblank is when scanout begins, not when a photon leaves the pixel the
// participant is looking at. Scanout position, panel rise time and the display's
// internal pipeline are all outside this package — they are constant offsets a
// photodiode measures and this cannot. What this establishes is whether that
// offset is CONSTANT, which is the part no host-side statistic can reach.
package vblank

import "os"

// Source describes how a timestamp was obtained.
//
// The distinction is the point of the package: a caller that cannot tell a
// measured vblank from an echoed flip timestamp will report a drift of exactly
// zero on a machine where nothing was measured at all.
type Source int

const (
	// Estimated means no OS measurement was available and the flip timestamp
	// was echoed back. Accurate to about one scanout period — 16 ms at 60 Hz.
	Estimated Source = iota

	// HardwareVerified means the OS measured it, to whatever precision the
	// display controller or vblank IRQ is stamped with — typically well under
	// a millisecond.
	HardwareVerified
)

// String returns a stable form for logs and data files.
func (s Source) String() string {
	switch s {
	case Estimated:
		return "vsync-estimated"
	case HardwareVerified:
		return "hardware-verified"
	default:
		return "unknown"
	}
}

// Timer reports the vblank most likely to correspond to a given flip.
//
// Implementations are safe for concurrent use; backends that publish from a
// background OS callback (CVDisplayLink) handle their own locking.
type Timer interface {
	// RecordFlip is called immediately after each flip. Backends needing a
	// synchronous query (Linux DRM) do their work here; those that publish
	// asynchronously (CVDisplayLink) treat it as a no-op.
	RecordFlip(flipTS uint64)

	// OnsetForFlip returns the OS-measured vblank time for the frame whose
	// flip returned at flipTS, in SDL ticks.
	//
	// ok=false means no estimate is available yet — the OS is expected to
	// publish within about one frame period, so retry on the next flip rather
	// than treating it as a failure.
	OnsetForFlip(flipTS uint64) (timestamp uint64, source Source, ok bool)

	// Precision reports what this timer returns for ok=true outcomes.
	Precision() Source

	// Close releases background resources. Idempotent.
	Close() error

	// Description is a one-line identifier for logs. Backends that search for
	// a usable device name the one they found, so a wrong choice is visible.
	Description() string
}

// fallback echoes the flip timestamp. It is always available and never errors.
type fallback struct{}

// NewFallback returns the no-OS-integration Timer.
func NewFallback() Timer { return fallback{} }

func (fallback) RecordFlip(uint64) {}

func (fallback) OnsetForFlip(flipTS uint64) (uint64, Source, bool) {
	return flipTS, Estimated, true
}

func (fallback) Precision() Source { return Estimated }
func (fallback) Close() error      { return nil }
func (fallback) Description() string {
	return "vsync-estimated (post-present flip timestamp, no OS integration)"
}

// AutoDetect returns the best Timer available on this platform, falling back to
// NewFallback if no backend initialises. It never returns nil; failures are
// logged and surface through the returned Timer's Description.
//
// It does NOT consult EnvDisable — that switch belongs to the caller, which has
// to report the difference between "this machine has no vblank clock" and "you
// switched it off" in its own vocabulary. Check Disabled first.
func AutoDetect() Timer { return autoDetect() }

// EnvDisable names the environment variable that forces the estimated path.
//
// It lives here rather than in each caller so the spelling cannot drift: a probe
// that reports hardware while the run beside it has the switch on would silently
// mislabel the arms of an A/B.
const EnvDisable = "GOXPY_VBLANK"

// Disabled reports whether the vblank clock has been switched off with
// GOXPY_VBLANK=off.
//
// This exists so the two sides of the comparison can be measured on ONE machine
// in ONE thermal state, interleaved. Without it the only available baseline is a
// capture from a different panel and a different code state, and a panel's rate
// moves tens of ppm over the first minutes of a run — larger than the effect
// being measured. An A/B that cannot be interleaved is not an A/B.
func Disabled() bool { return os.Getenv(EnvDisable) == "off" }
