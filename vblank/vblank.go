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

// Stats counts how each frame's vblank was resolved.
//
// It exists so the sequence handling can be checked from a run's own data rather
// than trusted. The failure it guards against — accepting the previous frame's
// vblank because the query beat the IRQ — is invisible in the timestamps
// themselves, since a one-frame error on a frame grid still looks regular. What
// is visible is how often the resolution had to wait, and whether the sequence
// ever advanced by something other than one.
type Stats struct {
	// Frames resolved to a measured vblank.
	Frames uint64
	// WaitedForNext counts frames where the query beat the vblank IRQ and an
	// absolute wait was needed. A large fraction is not an error — it means the
	// caller consistently gets there first — but it is where the old code
	// silently returned the previous frame instead.
	WaitedForNext uint64
	// MaxWaitNS is the longest such wait. It should stay well inside one frame;
	// approaching a frame period means the query is landing a whole frame early
	// and the pacing, not this code, is what to look at.
	MaxWaitNS uint64
	// SequenceGaps counts frames where the count jumped by more than one, i.e.
	// the display advanced while the caller did not. These are dropped frames,
	// reported rather than smoothed.
	SequenceGaps uint64
	// Failures counts frames where no measured vblank could be obtained and the
	// caller had to fall back to an estimate.
	Failures uint64
}

// StatsReporter is implemented by backends that can report resolution counts.
// Callers should type-assert; not every backend keeps them.
type StatsReporter interface{ Stats() Stats }

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
// It does NOT consult EnvOptIn — that switch belongs to the caller, which has to
// report the difference between "this machine has no vblank clock" and "you did
// not ask for one" in its own vocabulary. Check Enabled first.
func AutoDetect() Timer { return autoDetect() }

// EnvOptIn names the environment variable that turns the vblank clock on.
//
// It lives here rather than in each caller so the spelling cannot drift: a probe
// that reports hardware while the run beside it has the switch off would
// silently mislabel the arms of an A/B.
const EnvOptIn = "GOXPY_VBLANK"

// Enabled reports whether GOXPY_VBLANK=on asked for the vblank clock.
//
// # Why this is opt-in
//
// Not because it is broken — the defect that made it so is fixed, and measured
// to be fixed — but because it has nothing left to win.
//
// Anchoring flip timestamps on the kernel's vblank stamps ought to beat deriving
// them from the pacing schedule. Measured with a photodiode against a TTL, 1010
// cycles per run, on a Raspberry Pi 4 (V3D/kmsdrm) and a Radeon Pro W5700
// (radeonsi, X11 exclusive fullscreen):
//
//	arm                      flip -> photon slope        one-frame errors
//	schedule (5 runs)        -1.62 .. +0.12 ppm          none in any run
//	vblank, after the fix    +0.48 ppm                   none
//
// The two are indistinguishable, and both are flat to well under a ppm. So the
// default is the one with five runs behind it on two machines rather than one.
//
// The case this exists for is a host whose nominal refresh is badly wrong, where
// a schedule advancing on it would walk away from the panel. Neither machine
// measured here is that host: the W5700's nominal is 5.9 ppm from true, which
// over an eight-minute block is 0.2 ms. Until such a rig turns up, the switch
// also serves its original purpose — comparing the two on ONE machine in ONE
// thermal state, interleaved, because a panel's rate moves tens of ppm over the
// first minutes of a run and an A/B that cannot be interleaved is not an A/B.
//
// # What the defect was, so it is not reintroduced
//
// The backend asked for the MOST RECENT vblank and took whatever came back. The
// caller queries just after holding to the frame boundary, so that read lands
// within microseconds of the vblank IRQ and can fall on either side of it. When
// it fell before, the answer was the PREVIOUS frame's vblank — a whole frame
// out, on a grid where a frame-quantised error still looks perfectly regular.
//
// It was not a rare race. Instrumented on the W5700, the query beat the IRQ on
// 9498 of 30300 frames, 31.3%. Before the fix that produced a 13-second burst of
// +-1 frame errors four minutes into one run and a -48 ppm slope, while the TTL
// from the same loop held at +1.5 ppm; on the Pi it made the first 58-89 cycles
// of a run report onsets AFTER the photons had already been detected.
//
// drm_linux.go now resolves every frame against the vblank COUNT, so the same
// 31.3% resolve to the vblank they belong to (max wait 0.546 ms, no failures).
// Stats reports those counts, and they are worth reading on any run that enables
// this: the failure they guard against cannot be seen in the timestamps.
func Enabled() bool { return os.Getenv(EnvOptIn) == "on" }
