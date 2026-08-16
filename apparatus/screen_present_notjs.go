// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build !js

package apparatus

import (
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

// present submits the backbuffer to the display. On desktop this is plain
// SDL_RenderPresent, which blocks on VSYNC when the driver honours it.
//
// Do not add SDL_FlushRenderer here. It was tried, on the theory that the
// clear-only-frame bug (see Screen.fillWholeTarget) came from the driver
// deferring command submission until it had real GPU work — flushing should then
// have forced the frame out regardless of what the caller drew. Measured with a
// photodiode against tests/test_clear_only_frames, it made no difference: a frame
// carrying no draw calls was still not scanned out. Whatever elides the frame
// sits below SDL's command batch, so flushing that batch cannot reach it. The
// only remedy found is to give the frame real draw work, which is what
// fillWholeTarget does.
// It records the SDL-clock time of every present in lastFlipNS, including
// unpaced ones (Flip/FlipTS/Update). paceToFrame needs the time of the
// PREVIOUS present to know where the next frame boundary falls; when only the
// paced calls maintained that field, any unpaced flip in between left the
// baseline stale — pointing at a present two or more frames back — so the
// target was already in the past and the following paced flip did not pace at
// all. A loop mixing paced presentation calls with unpaced raw flips hit
// exactly this.
func (s *Screen) present() error {
	if err := s.Renderer.Present(); err != nil {
		return err
	}
	s.presentNS = sdl.TicksNS()
	return nil
}

// paceToFrame waits until the expected frame boundary after a present, for
// drivers where SDL_RenderPresent returns immediately (triple/mailbox
// buffering: NVIDIA + compositor, Wayland mailbox). On well-behaved
// double-buffered VSYNC there is nothing to wait for and it does nothing.
//
// The spin runs on the SDL clock (sdl.TicksNS) — the same clock FlipTS
// stamps onsets with and that input events carry — so the frame boundary the
// spin waits for and the timestamp the caller records live on one timebase.
//
// Only the last spinTailNS of the wait is spun; the rest is slept. Spinning the
// tail is necessary because sub-millisecond sleep cannot land a frame boundary
// on its own. Spinning the *whole* wait is worse than imprecise, it is a
// hazard: control requests SCHED_FIFO by default, and a real-time thread at
// 100% duty trips the kernel's real-time throttle — sched_rt_runtime_us is
// 950000 of a 1000000 period, so the thread is suspended for 50 ms once a
// second. Measured on a 22-core Linux 7.0 host, a pinned SCHED_FIFO 50 thread
// spinning continuously took 24 stalls in 25 s, 51.0 ms each, at exactly
// 1.000 s intervals. That is three dropped frames a second at 60 Hz. It only
// bites when the machine is otherwise busy — idle, the runqueue borrows unused
// real-time bandwidth from other CPUs and never throttles — so it is invisible
// on a quiet development machine and appears on a loaded one.
//
// The exposure is entirely conditional on the driver. Where SDL_RenderPresent
// blocks correctly there is nothing left to wait for and neither the sleep nor
// the spin runs, exactly as before. It is the triple/mailbox-buffered case,
// where present returns immediately and the wait is most of a frame, that would
// otherwise hold the CPU at ~100% duty.
//
// prevFlipNS is the value of lastFlipNS captured BEFORE the present this call
// follows — the frame boundary is one frame duration after that, not after the
// present that just happened. The caller must sample it before presenting,
// because present() overwrites lastFlipNS with its own timestamp.
//
// # What the schedule is anchored to, and why it matters
//
// Each call decides which of two clocks the NEXT frame's target derives from,
// and the two branches below are that choice:
//
//   - present returned at or after the target — the driver blocked on the
//     retrace itself, so its return carries the display's own cadence and is
//     the better anchor. lastFlipNS is left exactly as present() stamped it and
//     the chain re-anchors to the hardware every frame. On a driver that always
//     blocks this is the only branch ever taken and pacing costs one clock read.
//   - present returned early (triple/mailbox buffering) — there is no hardware
//     instant to anchor to, so the target itself becomes the anchor: the
//     schedule advances by exactly frameDur per frame, independent of when the
//     spin happened to exit.
//
// Anchoring the paced branch on the SCHEDULED boundary rather than on the spin
// exit is what stops the chain from ratcheting. The spin exits at target + ε,
// where ε is one iteration of the clock-read loop; assigning that back to
// lastFlipNS folded ε into the next target, and since ε >= 0 always, the
// schedule slid one-signed and never averaged out.
//
// Measured on a Raspberry Pi 4 (V3D, kmsdrm, 60.0000 Hz nominal) on 2026-08-09
// with a BBTK v3, photodiode against a GPIO TTL fired at the flip, 1010 cycles
// of 30 frames over 505 s: the TTL edge slid monotonically from 20.5 ms to
// 6.9 ms ahead of the photodiode onset. Both channels were individually regular
// (residuals about a straight line < 0.5 ms); their periods simply differed by
// 14.4 us per 30-frame cycle. The loop's own flip timestamps reported
// 500.014 ms against 30 x frameDur = 499.99998 ms — an excess of 14.0 us per
// cycle, 0.467 us per frame, which is the ε above. The framework's idea of when
// the frame appeared was 14 ms adrift from the actual photons by the end of an
// 8-minute run, and growing; that error reaches experiment code through FlipTS,
// so it lands directly in any reaction time measured from an onset.
//
// The same session on an Intel/Mesa laptop at the console showed no drift at
// all (TTL 499.7104 ms vs photodiode 499.7096 ms per cycle) despite an
// identical 0.438 us/frame ε, because there present blocks: its nominal
// 60.04 Hz frame is SHORTER than the panel's real one, the target was always
// already in the past, and every frame re-anchored. That contrast is what
// identified the mechanism.
//
// What remains after this is a residual proportional to how wrong the nominal
// refresh rate is: the paced branch now runs at exactly frameDur, so any gap
// between that and the panel's true frame still shows up as a one-signed slide,
// just without ε on top.
//
// Seeding frameDur from CalibrateRefresh instead would make that WORSE, not
// better. Measured 2026-08-14 against the panel's true frame period, recovered
// from the photodiode trains in the runs above by regression over 1000 cycles
// (cmd/timing-drift):
//
//	                     Pi 4 (V3D)        Precision 5490 (Intel/Mesa)
//	true panel rate      60.0000 Hz        60.0385 Hz
//	nominal display mode 60.0000 Hz        60.0400 Hz   ->  -0.1 /  -25 ppm
//	CalibrateRefresh(60) 60.0043 Hz        60.0228 Hz   -> -72 / +261 ppm
//
// The display mode is the better estimate on both machines, by a factor of
// ten. CalibrateRefresh takes the median of 59 intervals from a loop that is
// deliberately unpaced, so on a driver that does not block it measures the
// loop rather than the panel, and its median is quantised besides. It stays
// the right tool for the job it documents — telling a non-blocking driver
// apart from a frame-dropping one — but it is not a rate reference.
//
// The rate reference that IS accurate is the kernel's own vblank timestamp:
// consecutive DRM_IOCTL_WAIT_VBLANK stamps on the 5490 give 60.0384 Hz, which
// is 1.3 ppm from the photodiode-derived truth. media/present/drm_linux.go
// already reads them, for the movie player only.
func (s *Screen) paceToFrame(prevFlipNS uint64) {
	// Cache the nominal frame duration on first use: it is fixed for the
	// session, and re-querying the SDL display mode every frame is avoidable
	// work on this timing-critical path.
	if s.frameDur == 0 {
		s.frameDur = s.FrameDuration()
	}
	if prevFlipNS == 0 {
		s.heldToTarget = false
		return // first flip of the session: nothing to pace against
	}
	frame := uint64(s.frameDur.Nanoseconds())
	now := sdl.TicksNS()

	// A hardware anchor is a MEASURED vblank, and the most recent vblank is up
	// to a full frame old by the time present returns — so anchor+frameDur is
	// routinely already past, and treating that as "the driver blocked" stopped
	// pacing altogether (measured: 1799 blocked / 0 paced, where the same
	// machine had been pacing 92% of frames). Step forward in whole frames from
	// the anchor instead, to the first boundary after now. The anchor is re-read
	// from hardware every frame, so nothing accumulates however wrong frameDur
	// is; it only has to be good enough to count frames.
	if s.anchorIsHW && frame > 0 {
		n := (now-prevFlipNS)/frame + 1
		target := prevFlipNS + n*frame
		s.holdUntil(target, target-now, true)
		return
	}

	target := prevFlipNS + frame
	if now >= target {
		// The driver blocked past the boundary on its own. Leave present()'s
		// stamp in place — it is the moment SDL_RenderPresent returned, which
		// is both the better anchor for the next frame and what FlipTS
		// documents itself as returning. This branch also absorbs a stall: a
		// target left far in the past is simply abandoned rather than chased.
		s.blockedFrames++
		s.heldToTarget = false
		return
	}
	if short := target - now; short <= frame/hwAnchorSlackDiv {
		// Early, but only just: present covered all but a fraction of the
		// frame, so it did block on the retrace and merely came back a little
		// inside the NOMINAL boundary. Treat it as the branch above — keep
		// present's stamp and do not hold. See hwAnchorSlackDiv for why the
		// shortfall is a phase offset rather than buffering, and why holding
		// here was actively harmful.
		s.blockedFrames++
		s.earlyFrames++
		s.earlyNS += short
		if short > s.earlyMaxNS {
			s.earlyMaxNS = short
		}
		s.heldToTarget = false
		return
	}
	s.holdUntil(target, target-now, false)
}

// hwAnchorSlackDiv sets how far inside the nominal boundary SDL_RenderPresent
// may return and still count as having blocked on the retrace: frameDur divided
// by this, so 2.08 ms of a 16.67 ms frame.
//
// The two cases are far apart, which is what makes a threshold workable at all.
// A present that blocks returns one panel period after the last one, so the only
// thing separating it from the nominal boundary is the phase between the two
// grids plus jitter — measured on a Precision 5490 (Intel/Mesa, Wayland,
// 60.04 Hz nominal) on 2026-08-16: mean 0.676 ms, max 1.14 ms of a 16.661 ms
// frame, i.e. present had covered 15.99 ms of every frame. A present that does
// NOT block returns as soon as the command batch is queued, leaving most of the
// frame: 6.5 ms mean early on the same machine windowed, 15+ ms on a Radeon Pro
// W5700. Nothing observed sits between.
//
// Before this branch existed those 0.676 ms went to holdUntil, and the cost was
// not the wait but the anchor: holding replaced present's hardware stamp with
// the schedule, so 98.8 % of frames on that machine were timestamped by a clock
// free-running at the nominal rate — the same construction that put a Pi 4
// 14 ms adrift over eight minutes (see above). Anchoring on present instead
// re-locks every frame to the panel, and costs the caller nothing: the loop
// returns up to one slack early, draws, and the next present blocks to the same
// vblank it would have waited for.
//
// The known false positive is a loop on a NON-blocking driver whose drawing
// alone fills all but a slack of the frame; its presents then look near-boundary
// while carrying no hardware instant. It is visible in the stats — Early with a
// mean shortfall pressed up against the ceiling, rather than the sub-millisecond
// figure a blocking driver gives — and pacing was already doing almost nothing
// for such a loop.
const hwAnchorSlackDiv = 8

// holdUntil sleeps and then spins to target, and records the outcome.
//
// hwAnchored says whether the onset for this frame is a measured vblank (the
// GOXPY_VBLANK path, where the hold advances to the next boundary after a
// kernel timestamp) rather than the synthesised boundary this hold waited for.
// It only selects which tally the frame lands in: the hold is identical, but a
// vblank-anchored frame is NOT schedule-timestamped and must not be counted as
// though it were.
//
// Only the last spinTailNS is spun; see the note above on the real-time
// throttle for why the rest is slept.
func (s *Screen) holdUntil(target, wait uint64, hwAnchored bool) {
	now := sdl.TicksNS()
	if target-now > spinTailNS+minSleepNS {
		time.Sleep(time.Duration(target - now - spinTailNS))
		now = sdl.TicksNS()
	}
	for now < target {
		now = sdl.TicksNS()
	}
	// The scheduled boundary, NOT the spin exit (now): see the ratcheting note
	// above. The residual truncation in frameDur — FrameDuration converts a
	// float64 to a Duration, losing under a nanosecond per frame — does still
	// accumulate here, at 0.04 ppm. That is 20 us over the 8-minute run that
	// exposed the 14.5 ms drift.
	s.pacedTargetNS, s.heldToTarget = target, true
	if hwAnchored {
		s.vblankHeldFrames++
		return
	}
	// Tally how early the present came back, not how long the wait took to
	// serve: wait was measured before sleeping, so it is a property of the
	// driver rather than of this function's own overshoot.
	s.pacedFrames++
	s.pacedWaitNS += wait
	if wait > s.pacedWaitMaxNS {
		s.pacedWaitMaxNS = wait
	}
}

// spinTailNS is how much of the pacing wait is spun rather than slept.
//
// It has to cover how late time.Sleep can return, or the sleep overshoots the
// frame boundary and the frame is missed — which pure spinning never does, and
// is the cost being traded against the throttle above. Measured on this host,
// worst overshoot over 400 sleeps at each of 1, 5, 10 and 14 ms: 0.826 ms at
// normal priority on an idle machine, 0.734 ms at SCHED_FIFO 50 idle, and
// 0.255 ms at SCHED_FIFO 50 under full CPU load (real-time wakeups are prompt
// precisely when the machine is busy). 2 ms is about 2.4x the worst of those.
//
// The cost is bounded duty rather than bounded time: 2 ms of spin is 12% of a
// 60 Hz frame, 24% of a 120 Hz one and 48% at 240 Hz, all under the 95% the
// throttle allows.
const spinTailNS uint64 = uint64(2 * time.Millisecond)

// minSleepNS is the shortest sleep worth taking, and with spinTailNS it sets
// the frame period below which this degrades to a pure spin.
//
// The two constants sum to 3 ms, so a frame shorter than that is never slept
// through. That is deliberate: at 480 Hz the whole frame is 2.083 ms, less than
// the tail alone, so the sleep would be a fraction of a millisecond and its
// overshoot — up to 0.734 ms at SCHED_FIFO 50 — would sail past the boundary
// and miss the frame. A missed frame is a worse failure than a busy CPU.
//
// So above roughly 330 Hz this reverts to the old behaviour and the throttle
// hazard comes back with it, in the one case that triggers it: a driver whose
// present does not block. If that combination is ever real — a 480 Hz panel
// presenting through a mailbox-buffered compositor — the remedy is not in this
// function. Either give the render loop a driver that blocks (the spin then
// runs zero iterations and nothing is at stake), or lift the kernel limit with
// sysctl kernel.sched_rt_runtime_us=-1, or run without -realtime-priority.
const minSleepNS uint64 = uint64(1 * time.Millisecond)
