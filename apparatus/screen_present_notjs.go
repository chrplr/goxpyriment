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
	s.lastFlipNS = sdl.TicksNS()
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
func (s *Screen) paceToFrame(prevFlipNS uint64) {
	// Cache the nominal frame duration on first use: it is fixed for the
	// session, and re-querying the SDL display mode every frame is avoidable
	// work on this timing-critical path.
	if s.frameDur == 0 {
		s.frameDur = s.FrameDuration()
	}
	if prevFlipNS == 0 {
		return // first flip of the session: nothing to pace against
	}
	target := prevFlipNS + uint64(s.frameDur.Nanoseconds())
	now := sdl.TicksNS()
	// Sleep off the bulk only when there is enough of the wait left for that to
	// be safe: the tail still has to cover the sleep's overshoot, and a sleep
	// shorter than minSleepNS is not worth the risk of taking one. Below that
	// threshold — which is every frame on a panel faster than about 330 Hz —
	// this is a pure spin, exactly as it was before.
	if now < target && target-now > spinTailNS+minSleepNS {
		time.Sleep(time.Duration(target - now - spinTailNS))
		now = sdl.TicksNS()
	}
	for now < target {
		now = sdl.TicksNS()
	}
	// The flip is deemed to land at the end of the spin, which is what the
	// next frame paces against.
	s.lastFlipNS = now
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
