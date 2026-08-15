// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build linux

// Linux DRM (Direct Rendering Manager) backend for hardware-verified
// vsync timestamps.
//
// After each Present, the manager calls RecordFlip(flipTS). The kernel stamps
// each vblank in CLOCK_MONOTONIC (the default for DRM drivers); we convert to
// SDL ticks with a fixed epoch offset captured at backend construction.
//
// # Which vblank, and why the sequence number decides
//
// Asking only for the MOST RECENT vblank (DRM_VBLANK_RELATIVE, sequence 0) is
// not enough, and the difference is a whole frame. The caller queries just after
// holding to the frame boundary, so the query lands within microseconds of the
// vblank IRQ — and whether it lands before or after is a coin flip. Before, and
// the answer is the PREVIOUS frame's vblank.
//
// Measured with a photodiode, that produced onsets a frame late in bursts
// lasting tens of seconds: on a Raspberry Pi 4 the first 58-89 cycles of a run
// reported the flip AFTER the photons had already been detected, and on a Radeon
// Pro W5700 a 13-second window four minutes into a run oscillated between +1 and
// -1 frame. Three runs in four were affected. A caller cannot detect this: the
// timestamps look perfectly regular, because a frame-quantised error on a
// frame-quantised grid is invisible from the host side.
//
// So this tracks the vblank COUNT and resolves each frame against it:
//
//   - sequence advanced by one since the last frame — the expected case; that
//     vblank is this frame's, and its timestamp is used as measured.
//   - sequence has not advanced — the query beat the IRQ. Poll until the count
//     reaches lastSeq+1 and take THAT vblank's timestamp, giving up after a
//     short budget rather than accepting the stale one.
//   - sequence jumped by more than one — frames were missed. The timestamp is
//     still a real measurement of the current vblank, so it is used, and the gap
//     is counted rather than smoothed over.
//
// Every timestamp this returns is therefore a kernel measurement of a vblank
// whose sequence number is known. Nothing is extrapolated.
//
// The race is not rare, and the counts are the reason to keep this rather than
// simplify it back. Instrumented on the W5700 over a 1010-cycle run: the query
// beat the IRQ on 9498 of 30300 frames, 31.3%, every one of which would
// previously have been stamped with the wrong vblank. Resolving them by count
// cost a maximum wait of 0.546 ms and failed on none, and the run's onsets went
// from a -48 ppm slope with seven one-frame jumps to +0.48 ppm with none —
// matching the photodiode to the second decimal.
//
// # Why polling, and not a blocking wait
//
// Asking the kernel to wait for the vblank we want would be the obvious way to
// do it, and neither form of that request is usable here. Measured on Linux 7.0
// with i915, on /dev/dri/card1:
//
//	relative seq=1               ok, sequence advanced by exactly 1, waited 14.577 ms
//	absolute seq=current+1       EBUSY
//	absolute | NEXTONMISS        EINVAL
//	relative | NEXTONMISS        EINVAL
//
// So only the RELATIVE form blocks, and relative means "one more than the count
// when the ioctl runs" — not "the vblank I named". If the IRQ fires between the
// query above and that ioctl, it waits for the vblank AFTER the one wanted and
// returns a full frame later, which stalls the render loop for 16 ms and drops
// the frame it was trying to time. Trading a one-frame timestamp error for a
// one-frame stall is not a fix.
//
// Polling relative-0 has neither problem: each poll is a cheap ioctl that
// returns the current count immediately, the loop stops the moment the wanted
// sequence appears, and the budget is ours rather than the display's. If the
// vblank does not arrive within it, the frame is reported as an estimate and
// counted — a known-unknown instead of a wrong number.
//
// All cross-language calls use the standard syscall package (no cgo,
// no purego). DRM is opened on /dev/dri/cardN (first that succeeds);
// the user must be a member of the video group for read access on
// most distributions.
//
// Reference:
//   https://www.kernel.org/doc/html/latest/gpu/drm-uapi.html
//   include/uapi/drm/drm.h (struct drm_wait_vblank)

package vblank

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"
)

const (
	// DRM_IOCTL_WAIT_VBLANK = _IOWR('d', 0x3a, union drm_wait_vblank)
	// where the union is 24 bytes (size of the larger reply branch).
	// Encoding: (3 << 30) | (24 << 16) | ('d' << 8) | 0x3a = 0xC018643A.
	drmIoctlWaitVblank uintptr = 0xC018643A

	// drm_vblank_seq_type bits. Only the relative form is used, and only with
	// sequence 0 — a pure, immediate read of the current count and its
	// timestamp. The header note above records what the other forms did when
	// tried, so they are not tried again.
	drmVblankRelative uint32 = 0x1

	// How many times to retry an interrupted query.
	//
	// The Go runtime preempts goroutines with signals, so an ioctl here can
	// return EINTR through no fault of the display. A relative-0 query is a
	// pure read, so retrying it is free of side effects.
	drmWaitRetries = 8

	// How long to poll for the vblank a frame is waiting on before giving up
	// and reporting an estimate.
	//
	// It only has to cover the gap between the caller's query and the IRQ, which
	// is microseconds when the caller has just paced to the frame boundary. 3 ms
	// is generous for that while staying well inside a frame at any refresh rate
	// this runs at, so a budget exhaustion means something is actually wrong —
	// the caller is a whole frame early, or the CRTC has stopped — rather than
	// the timing being tight.
	drmSeqPollBudgetNS uint64 = 3_000_000

	// The CRTC index goes in bits 1-5 of the type field:
	//   #define _DRM_VBLANK_HIGH_CRTC_SHIFT 1
	//   #define _DRM_VBLANK_HIGH_CRTC_MASK  0x0000003e
	// Leaving them clear asks for CRTC 0, which is not always the one driving
	// the display.
	drmVblankHighCrtcShift uint32 = 1
	drmVblankHighCrtcMask  uint32 = 0x3e

	// How many card nodes and CRTCs to search. Both are small on purpose: the
	// search runs once at construction and every miss costs one failed ioctl.
	maxCards = 4
	maxCrtcs = 4

	// CLOCK_MONOTONIC is the same on every Linux arch; SDL3 also uses
	// CLOCK_MONOTONIC for SDL_GetTicksNS.
	clockMonotonic uintptr = 1

	drmRingSize = 8
)

// drmWaitVblank mirrors `union drm_wait_vblank`:
//
//	struct drm_wait_vblank_request {
//	    enum drm_vblank_seq_type type;  // u32
//	    unsigned int sequence;          // u32
//	    unsigned long signal;           // u64 (LP64)
//	};
//	struct drm_wait_vblank_reply {
//	    enum drm_vblank_seq_type type;  // u32
//	    unsigned int sequence;          // u32
//	    long tval_sec;                  // i64 (LP64)
//	    long tval_usec;                 // i64 (LP64)
//	};
//
// Total size: 24 bytes (the larger reply branch). We layer the request
// and reply branches over a fixed 16-byte data area: as request, only
// the first 8 bytes (signal) are meaningful; as reply, the first 8 are
// tval_sec and the next 8 are tval_usec.
type drmWaitVblank struct {
	Type     uint32
	Sequence uint32
	Data     [16]byte
}

type drmFlipVsync struct {
	flipTS  uint64
	vsyncTS uint64
}

type drmBackend struct {
	fd   int
	path string // which /dev/dri node answered, for Description
	crtc uint32 // which CRTC answered; folded into every request

	mu       sync.Mutex
	pairs    [drmRingSize]drmFlipVsync
	pairsIdx int

	// Vblank count of the frame resolved last, and whether one has been.
	// Guarded by mu.
	lastSeq uint32
	haveSeq bool

	stats Stats // guarded by mu

	// CLOCK_MONOTONIC ns -> SDL ticks: sdl_ticks = clock_ns - epochOffsetNS.
	epochOffsetNS int64

	closed atomic.Bool
}

// requestType builds the drm_wait_vblank type field for a CRTC index.
func requestType(crtc uint32) uint32 {
	return drmVblankRelative | ((crtc << drmVblankHighCrtcShift) & drmVblankHighCrtcMask)
}

func newDRMBackend() (Timer, error) {
	// Search every card node AND every CRTC, rather than assuming the first
	// node that opens is the one with a display on CRTC 0.
	//
	// Neither assumption holds. A machine can expose a render-only node ahead of
	// the one driving the panel — asking it for a vblank returns EINVAL, because
	// the pipe index exceeds its (zero) CRTC count — and the display need not be
	// on CRTC 0 even on the right node. Both were found the hard way: an
	// Intel/Mesa laptop where card1 answers and card2 returns ENOTSUP on every
	// CRTC, and a Radeon Pro W5700 workstation where the first node that opened
	// returned EINVAL and the backend gave up without trying the next.
	//
	// The old code broke out of its loop on the first successful OPEN and then
	// returned the probe's error, so a single unlucky enumeration order was
	// enough to lose the vblank clock on a machine that had one.
	fd, path, crtc, err := findDRMNode()
	if err != nil {
		return nil, err
	}
	return newBackendOn(fd, path, crtc)
}

// findDRMNode returns an open fd on the first card/CRTC pair that answers a
// vblank query, along with which one it was. The caller owns the fd.
//
// Separate from newDRMBackend so it can be used without newBackendOn, which
// needs SDL loaded to capture the clock epoch — a unit test exercising the ioctl
// has no SDL and does not need one.
func findDRMNode() (fd int, path string, crtc uint32, err error) {
	var tried []string
	for card := 0; card < maxCards; card++ {
		p := fmt.Sprintf("/dev/dri/card%d", card)
		f, oerr := syscall.Open(p, syscall.O_RDWR|syscall.O_CLOEXEC, 0)
		if oerr != nil {
			tried = append(tried, fmt.Sprintf("%s: open: %v", p, oerr))
			continue
		}
		c, perr := probeCRTCs(f)
		if perr != nil {
			tried = append(tried, fmt.Sprintf("%s: %v", p, perr))
			_ = syscall.Close(f)
			continue
		}
		return f, p, c, nil
	}
	return -1, "", 0, fmt.Errorf("no DRM node answered DRM_IOCTL_WAIT_VBLANK (%s)",
		strings.Join(tried, "; "))
}

// probeCRTCs returns the first CRTC on fd that answers a vblank query.
//
// It cannot tell a live CRTC from an idle one — a blanked pipe answers without
// its sequence advancing — because distinguishing them means waiting, and this
// runs in a constructor. Callers that care check the sequence themselves;
// tests/test_vblank_drift reports the grid residual for exactly this reason.
func probeCRTCs(fd int) (uint32, error) {
	var errs []string
	for crtc := uint32(0); crtc < maxCrtcs; crtc++ {
		probe := drmWaitVblank{Type: requestType(crtc), Sequence: 0}
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
			uintptr(fd), drmIoctlWaitVblank,
			uintptr(unsafe.Pointer(&probe))); errno != 0 {
			errs = append(errs, fmt.Sprintf("crtc %d: %v", crtc, errno))
			continue
		}
		return crtc, nil
	}
	return 0, fmt.Errorf("no CRTC answered (%s)", strings.Join(errs, ", "))
}

func newBackendOn(fd int, path string, crtc uint32) (Timer, error) {

	// Capture epoch offset between CLOCK_MONOTONIC and sdl.TicksNS.
	clockNow, err := monotonicNS()
	if err != nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("clock_gettime(CLOCK_MONOTONIC): %w", err)
	}
	sdlNow := sdl.TicksNS()

	return &drmBackend{
		fd:            fd,
		path:          path,
		crtc:          crtc,
		epochOffsetNS: int64(clockNow) - int64(sdlNow),
	}, nil
}

// monotonicNS returns CLOCK_MONOTONIC in nanoseconds via the syscall
// package (no libc / no purego).
func monotonicNS() (uint64, error) {
	var ts [2]int64 // {sec, nsec}; layout matches kernel timespec on LP64
	_, _, errno := syscall.Syscall(syscall.SYS_CLOCK_GETTIME,
		clockMonotonic, uintptr(unsafe.Pointer(&ts)), 0)
	if errno != 0 {
		return 0, errno
	}
	return uint64(ts[0])*1e9 + uint64(ts[1]), nil
}

// query issues one DRM_IOCTL_WAIT_VBLANK and returns the sequence and the
// timestamp converted to SDL ticks.
//
// An absolute request blocks until the named vblank; a relative request with
// sequence 0 returns the most recent one immediately. EINTR is retried because
// the Go runtime's preemption signals land here, and an absolute request is
// idempotent under retry.
func (b *drmBackend) query(typ, seq uint32) (uint32, uint64, error) {
	for try := 0; try < drmWaitRetries; try++ {
		v := drmWaitVblank{Type: typ, Sequence: seq}
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
			uintptr(b.fd), drmIoctlWaitVblank, uintptr(unsafe.Pointer(&v)))
		if errno == syscall.EINTR || errno == syscall.EAGAIN {
			continue
		}
		if errno != 0 {
			return 0, 0, errno
		}
		tvalSec := *(*int64)(unsafe.Pointer(&v.Data[0]))
		tvalUsec := *(*int64)(unsafe.Pointer(&v.Data[8]))
		clockNS := uint64(tvalSec)*1e9 + uint64(tvalUsec)*1000
		ticks := uint64(int64(clockNS) - b.epochOffsetNS)
		if ticks == 0 {
			ticks = 1 // preserve the "0 = unset" sentinel
		}
		return v.Sequence, ticks, nil
	}
	return 0, 0, syscall.EINTR
}

// pollForSequence polls the vblank count until it reaches want, and returns
// that vblank's timestamp along with how long the wait took.
//
// It returns ok=false when the budget runs out, which the caller must report as
// an estimate rather than substituting a nearby stamp — the entire point of the
// sequence handling is that a vblank one frame away is a wrong answer, not an
// approximate one.
//
// The poll runs on CLOCK_MONOTONIC rather than SDL ticks so it stays usable
// without SDL loaded, which is what lets the test exercise it.
func (b *drmBackend) pollForSequence(want uint32, budgetNS uint64) (seq uint32, ts uint64, waited uint64, ok bool) {
	start, err := monotonicNS()
	if err != nil {
		return 0, 0, 0, false
	}
	for {
		s, t, qerr := b.query(requestType(b.crtc)|drmVblankRelative, 0)
		now, cerr := monotonicNS()
		if cerr != nil {
			return 0, 0, 0, false
		}
		if qerr == nil && int32(s-want) >= 0 {
			return s, t, now - start, true
		}
		if now-start >= budgetNS {
			return 0, 0, now - start, false
		}
	}
}

func (b *drmBackend) RecordFlip(flipTS uint64) {
	if b.closed.Load() {
		return
	}

	seq, ts, err := b.query(requestType(b.crtc)|drmVblankRelative, 0)
	if err != nil {
		// Silent: this fires every frame, so a persistent failure would spam
		// the log. It surfaces as Stats.Failures and as an Estimated onset.
		b.mu.Lock()
		b.stats.Failures++
		b.mu.Unlock()
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	switch {
	case !b.haveSeq:
		// First frame: nothing to compare against, so take what the display
		// reports and start counting from there.
	default:
		// Signed difference so the uint32 counter's wrap (about two years at
		// 60 Hz, but free to handle) does not read as an enormous jump.
		switch delta := int32(seq - b.lastSeq); {
		case delta <= 0:
			// The query beat the vblank IRQ. Poll until the count reaches the
			// vblank this frame is on, instead of accepting the previous one.
			//
			// Unlocked across the poll so a concurrent OnsetForFlip is not held
			// off for milliseconds. Nothing else writes lastSeq — RecordFlip is
			// called from the render thread only — so it cannot move meanwhile.
			want := b.lastSeq + 1
			b.mu.Unlock()
			gotSeq, gotTS, waited, ok := b.pollForSequence(want, drmSeqPollBudgetNS)
			b.mu.Lock()

			if !ok {
				b.stats.Failures++
				return
			}
			seq, ts = gotSeq, gotTS
			b.stats.WaitedForNext++
			if waited > b.stats.MaxWaitNS {
				b.stats.MaxWaitNS = waited
			}
			// The poll can overshoot if the display advanced more than one
			// vblank while we were getting there; that is a dropped frame and is
			// counted as one rather than hidden by the poll having "succeeded".
			if extra := int32(gotSeq - want); extra > 0 {
				b.stats.SequenceGaps += uint64(extra)
			}
		case delta > 1:
			// The display advanced while the caller did not: dropped frames.
			// The timestamp is still a real measurement of the vblank the frame
			// went out on, so it is kept; the gap is counted so a run can say
			// how many there were rather than absorbing them into the numbers.
			b.stats.SequenceGaps += uint64(delta - 1)
		}
	}

	b.lastSeq, b.haveSeq = seq, true
	b.stats.Frames++
	b.pairs[b.pairsIdx] = drmFlipVsync{flipTS: flipTS, vsyncTS: ts}
	b.pairsIdx = (b.pairsIdx + 1) % drmRingSize
}

// Stats implements StatsReporter.
func (b *drmBackend) Stats() Stats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stats
}

func (b *drmBackend) OnsetForFlip(flipTS uint64) (uint64, Source, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, p := range b.pairs {
		if p.flipTS == flipTS && p.vsyncTS != 0 {
			return p.vsyncTS, HardwareVerified, true
		}
	}
	return 0, Estimated, false
}

func (b *drmBackend) Precision() Source { return HardwareVerified }

// Description names the node and CRTC that answered, so a wrong choice on a
// multi-GPU or multi-head machine is visible rather than inferred.
//
// The word before the path is load-bearing, however odd that looks. This string
// is reproduced verbatim in the documentation, and TeX announces every file it
// opens as "(" immediately followed by a path — so latexmk, which scans the log
// for that pattern to build its dependency list, took "(/dev/dri/card1" for a
// file it should checksum. Reading a DRM character device does not fail fast, it
// blocks, so the PDF build hung indefinitely on the one page quoting this line.
// Anything non-path-shaped after the parenthesis prevents that.
func (b *drmBackend) Description() string {
	return fmt.Sprintf("Linux DRM vblank (card %s, crtc %d, DRM_IOCTL_WAIT_VBLANK, hardware-verified)", b.path, b.crtc)
}

func (b *drmBackend) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}
	if b.fd >= 0 {
		_ = syscall.Close(b.fd)
		b.fd = -1
	}
	return nil
}
