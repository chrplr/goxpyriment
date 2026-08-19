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

	// PresentReturn means the onset is the instant SDL_RenderPresent returned,
	// on a frame where the driver blocked to the retrace and therefore handed
	// back a hardware instant. Not measured by the OS, but not synthesised
	// either: it is set by the display, re-established every frame, and cannot
	// accumulate error.
	//
	// This is the common case on a well-behaved driver, and it used to be
	// reported as Estimated — so a Raspberry Pi 4 capture whose every onset came
	// from the hardware was labelled "vsync-estimated" in all 1000 rows.
	PresentReturn

	// Scheduled means the driver did not block, so Update held the frame and the
	// onset is the boundary it held to. This one IS synthesised: it advances at
	// the nominal frame period and is only as accurate as that period.
	Scheduled
)

// String returns a stable form for logs and data files.
func (s Source) String() string {
	switch s {
	case Estimated:
		return "vsync-estimated"
	case HardwareVerified:
		return "hardware-verified"
	case PresentReturn:
		return "present-return"
	case Scheduled:
		return "pacing-schedule"
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
	// TargetFrameNS is the frame period of the display the caller said it was
	// presenting to (Target.FrameNS), or 0 if it did not say.
	TargetFrameNS uint64
	// MeasuredFrameNS is the mean interval between the vblanks resolved so far,
	// computed from the vblank COUNT rather than the frame count so that
	// dropped frames do not stretch it. It stays 0 until enough frames have
	// been seen for the figure to mean anything.
	MeasuredFrameNS uint64
}

// crtcMatchPPM is how far a display's measured or programmed frame period may
// sit from the one the caller named before the two are taken to be different
// displays.
//
// The budget it has to cover: SDL falls back to a refresh rounded to two
// decimals when a driver gives no exact rational, which is 83 ppm at 60 Hz; and
// the live check measures a cadence rather than reading a mode, which adds tens
// of ppm over its sample. 300 ppm covers both with room and is still five times
// smaller than the 1449 ppm a Precision 5490 was out by when it read the wrong
// CRTC. It is deliberately NOT set to the tens of ppm a panel's own crystal
// wanders by: this asks "is this the same display", not "is this display
// accurate".
const crtcMatchPPM = 300

// MismatchPPM reports how far the vblanks actually being read sit from the frame
// period the caller named, in parts per million, and whether both figures are
// known yet.
//
// This is the check that catches a backend reading the wrong display. A CRTC is
// chosen from its programmed mode, which is exact but is still a claim about
// hardware; this is the cadence measured on the running machine. It also catches
// the one case mode selection cannot: a pipe that is programmed with the right
// mode but is not scanning it out, whose sequence therefore does not advance.
func (s Stats) MismatchPPM() (int64, bool) {
	if s.TargetFrameNS == 0 || s.MeasuredFrameNS == 0 {
		return 0, false
	}
	return (int64(s.MeasuredFrameNS) - int64(s.TargetFrameNS)) * 1_000_000 /
		int64(s.TargetFrameNS), true
}

// WrongDisplay reports whether the measured cadence is too far from the named
// display's for the two to be the same display.
//
// False on a run that has not measured yet, which is the safe direction: it says
// "no evidence of a mismatch", not "verified".
func (s Stats) WrongDisplay() bool {
	ppm, ok := s.MismatchPPM()
	if !ok {
		return false
	}
	return ppm > crtcMatchPPM || ppm < -crtcMatchPPM
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

// Target describes the display the caller is presenting to.
//
// It exists because "the vblank clock" is not one clock. A machine lights one
// CRTC per head, each with its own cadence, and the backend has to be told which
// one the experiment is showing stimuli on — a laptop with an external monitor
// runs two pipes a thousand ppm apart, and reading the wrong one produces
// timestamps that look perfectly regular while walking a frame away from the
// photons. See the header of drm_crtc_linux.go for the capture that established
// this.
//
// FrameNS is the one field that matters; the size only breaks ties between heads
// running the same rate. A zero Target means "I cannot say", and a backend that
// gets one falls back to the older blind probe rather than refusing to start.
type Target struct {
	// FrameNS is the nominal frame period of the display, in nanoseconds —
	// apparatus.Screen.FrameDuration(). 0 means unknown.
	FrameNS uint64
	// Width and Height are the display mode's size in pixels. 0 means unknown.
	Width, Height int
}

// AutoDetect returns the best Timer available on this platform without naming a
// display. Equivalent to AutoDetectFor(Target{}).
//
// Prefer AutoDetectFor wherever the display is known: on a multi-head machine
// this cannot tell which CRTC to read and has to guess.
func AutoDetect() Timer { return AutoDetectFor(Target{}) }

// AutoDetectFor returns the best Timer available on this platform for the given
// display, falling back to NewFallback if no backend initialises. It never
// returns nil; failures are logged and surface through the returned Timer's
// Description.
//
// A backend that cannot find a CRTC matching the target reports that as a
// failure rather than reading whichever one answers. Falling back to
// present-return anchoring is a known quantity — five photodiode runs behind it,
// see Enabled — and timing a display the experiment is not drawing on is not.
//
// It does NOT consult EnvOptIn — that switch belongs to the caller, which has to
// report the difference between "this machine has no vblank clock" and "you did
// not ask for one" in its own vocabulary. Check Enabled first.
func AutoDetectFor(t Target) Timer { return autoDetect(t) }

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
