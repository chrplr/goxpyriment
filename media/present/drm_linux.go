// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

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

package present

import (
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/chrplr/goxpyriment/apparatus"
)

const (
	// DRM_IOCTL_WAIT_VBLANK = _IOWR('d', 0x3a, union drm_wait_vblank)
	// where the union is 24 bytes (size of the larger reply branch).
	// Encoding: (3 << 30) | (24 << 16) | ('d' << 8) | 0x3a = 0xC018643A.
	drmIoctlWaitVblank uintptr = 0xC018643A

	// drm_vblank_seq_type bits.
	drmVblankRelative uint32 = 0x1

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
	fd int

	mu       sync.Mutex
	pairs    [drmRingSize]drmFlipVsync
	pairsIdx int

	// CLOCK_MONOTONIC ns -> SDL ticks: sdl_ticks = clock_ns - epochOffsetNS.
	epochOffsetNS int64

	closed atomic.Bool
}

func newDRMBackend(_ *apparatus.Screen) (Timer, error) {
	// Try /dev/dri/card0..3. Most desktop systems have card0; some
	// embedded / multi-GPU boxes have card1+.
	var (
		fd  int
		err error
	)
	for i := 0; i < 4; i++ {
		path := fmt.Sprintf("/dev/dri/card%d", i)
		fd, err = syscall.Open(path, syscall.O_RDWR|syscall.O_CLOEXEC, 0)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("open /dev/dri/cardN: %w", err)
	}

	// Probe the ioctl to confirm we have permission and that the
	// driver supports vblank queries.
	probe := drmWaitVblank{Type: drmVblankRelative, Sequence: 0}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(fd), drmIoctlWaitVblank,
		uintptr(unsafe.Pointer(&probe))); errno != 0 {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("DRM_IOCTL_WAIT_VBLANK probe: %w", errno)
	}

	// Capture epoch offset between CLOCK_MONOTONIC and sdl.TicksNS.
	clockNow, err := monotonicNS()
	if err != nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("clock_gettime(CLOCK_MONOTONIC): %w", err)
	}
	sdlNow := sdl.TicksNS()

	return &drmBackend{
		fd:            fd,
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
	v := drmWaitVblank{Type: drmVblankRelative, Sequence: 0}
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

func (b *drmBackend) OnsetForFlip(flipTS uint64) (uint64, OnsetSource, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, p := range b.pairs {
		if p.flipTS == flipTS && p.vsyncTS != 0 {
			return p.vsyncTS, HardwareVerified, true
		}
	}
	return 0, VsyncEstimated, false
}

func (b *drmBackend) Precision() OnsetSource { return HardwareVerified }

func (b *drmBackend) Description() string {
	return "Linux DRM vblank (DRM_IOCTL_WAIT_VBLANK, hardware-verified)"
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
