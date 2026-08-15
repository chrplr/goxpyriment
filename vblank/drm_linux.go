// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build linux

// Linux DRM (Direct Rendering Manager) backend for hardware-verified
// vsync timestamps.
//
// After each Present, the manager calls RecordFlip(flipTS); we issue
// DRM_IOCTL_WAIT_VBLANK with type=DRM_VBLANK_RELATIVE | sequence=0 to
// query the most recent vblank's count and timestamp. The kernel
// stamps the vblank timestamp in CLOCK_MONOTONIC (default for DRM
// drivers); we convert it to SDL ticks by applying a fixed epoch
// offset captured at backend construction.
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

	// drm_vblank_seq_type bits.
	drmVblankRelative uint32 = 0x1

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
	var tried []string
	for card := 0; card < maxCards; card++ {
		path := fmt.Sprintf("/dev/dri/card%d", card)
		fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_CLOEXEC, 0)
		if err != nil {
			tried = append(tried, fmt.Sprintf("%s: open: %v", path, err))
			continue
		}
		crtc, err := probeCRTCs(fd)
		if err != nil {
			tried = append(tried, fmt.Sprintf("%s: %v", path, err))
			_ = syscall.Close(fd)
			continue
		}
		return newBackendOn(fd, path, crtc)
	}
	return nil, fmt.Errorf("no DRM node answered DRM_IOCTL_WAIT_VBLANK (%s)", strings.Join(tried, "; "))
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

func (b *drmBackend) RecordFlip(flipTS uint64) {
	if b.closed.Load() {
		return
	}
	v := drmWaitVblank{Type: requestType(b.crtc), Sequence: 0}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(b.fd), drmIoctlWaitVblank,
		uintptr(unsafe.Pointer(&v))); errno != 0 {
		// Silent: this fires every frame; persistent failure would spam
		// the log. Closing-and-falling-back can be added if needed.
		return
	}
	tvalSec := *(*int64)(unsafe.Pointer(&v.Data[0]))
	tvalUsec := *(*int64)(unsafe.Pointer(&v.Data[8]))
	clockNS := uint64(tvalSec)*1e9 + uint64(tvalUsec)*1000
	sdlTicks := uint64(int64(clockNS) - b.epochOffsetNS)
	if sdlTicks == 0 {
		sdlTicks = 1 // preserve "0 = unset" sentinel
	}
	b.mu.Lock()
	b.pairs[b.pairsIdx] = drmFlipVsync{flipTS: flipTS, vsyncTS: sdlTicks}
	b.pairsIdx = (b.pairsIdx + 1) % drmRingSize
	b.mu.Unlock()
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
