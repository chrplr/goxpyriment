// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build darwin

// macOS CVDisplayLink backend.
//
// Uses CoreVideo's CVDisplayLink to receive a callback at every vsync,
// publishing the next-frame display-time (inOutputTime->hostTime) into
// a ring buffer. OnsetForFlip looks up the smallest published vsync
// timestamp greater than or equal to the requested FlipTS.
//
// All cross-language calls go through purego (no cgo). The CVDisplayLink
// callback fires on a background CV thread; the ring buffer uses a
// mutex to coordinate with OnsetForFlip / RecordFlip on the main thread.
//
// host time -> SDL ticks: SDL_GetTicksNS on macOS is built on
// mach_absolute_time, so we capture the offset between mach_absolute_time
// (in nanoseconds, after applying mach_timebase_info) and SDL_GetTicksNS
// at backend construction. Subsequent vsync timestamps are converted by
// applying that offset.
//
// CVDisplayLink reference:
//   https://developer.apple.com/documentation/corevideo/cvdisplaylink-714
//
// inOutputTime semantics: the CVTimeStamp the callback receives via
// inOutputTime is "the time at which the next frame will be displayed",
// i.e., the upcoming vsync's first-pixel time. With a vsync-blocking
// FIFO Present (goxpyriment's default), Present unblocks at the same
// vsync, so the matching inOutputTime is the one for the vsync we
// just exited Present on.

package present

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/ebitengine/purego"

	"github.com/chrplr/goxpyriment/apparatus"
)

// vsyncRingSize is the number of recent vsync timestamps we keep. With
// a 60 Hz display this covers ~270 ms of history, easily enough to
// match any FlipTS to its corresponding vsync even when the manager is
// briefly stalled (e.g., GC pause).
const vsyncRingSize = 16

// cvTimeStamp mirrors the CVTimeStamp struct from <CoreVideo/CVBase.h>.
// Field offsets MUST match exactly — purego does not pad for us.
//
//	struct CVTimeStamp {
//	    uint32_t        version;             // offset  0  (4 bytes)
//	    int32_t         videoTimeScale;      // offset  4  (4 bytes)
//	    int64_t         videoTime;           // offset  8  (8 bytes)
//	    uint64_t        hostTime;            // offset 16  (8 bytes)
//	    double          rateScalar;          // offset 24  (8 bytes)
//	    int64_t         videoRefreshPeriod;  // offset 32  (8 bytes)
//	    CVSMPTETime     smpteTime;           // offset 40  (24 bytes)
//	    uint64_t        flags;               // offset 64  (8 bytes)
//	    uint64_t        reserved;            // offset 72  (8 bytes)
//	};
//
// Total: 80 bytes. We only read HostTime (offset 16); the rest is
// declared for layout fidelity.
type cvTimeStamp struct {
	Version            uint32   // offset 0
	VideoTimeScale     int32    // offset 4
	VideoTime          int64    // offset 8
	HostTime           uint64   // offset 16
	RateScalar         float64  // offset 24
	VideoRefreshPeriod int64    // offset 32
	SmpteTime          [24]byte // offset 40 (CVSMPTETime, 24 bytes)
	Flags              uint64   // offset 64
	Reserved           uint64   // offset 72
}

// machTimebase mirrors mach_timebase_info_data_t.
type machTimebase struct {
	Numer uint32
	Denom uint32
}

// Library handles + bound function pointers (initialised once).
var (
	cvOnce      sync.Once
	cvLoadErr   error
	cvCoreVideo uintptr
	cvLibSystem uintptr
	cvAPI       struct {
		// CoreVideo
		CVDisplayLinkCreateWithActiveCGDisplays func(out *uintptr) int32
		CVDisplayLinkSetOutputCallback          func(link uintptr, callback uintptr, ctx uintptr) int32
		CVDisplayLinkStart                      func(link uintptr) int32
		CVDisplayLinkStop                       func(link uintptr) int32
		CVDisplayLinkRelease                    func(link uintptr)
		// libSystem
		MachTimebaseInfo func(info *machTimebase) int32
		MachAbsoluteTime func() uint64
	}
)

func loadCVAPI() error {
	cvOnce.Do(func() {
		const cvPath = "/System/Library/Frameworks/CoreVideo.framework/Versions/A/CoreVideo"
		cv, err := purego.Dlopen(cvPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			cvLoadErr = fmt.Errorf("dlopen CoreVideo: %w", err)
			return
		}
		cvCoreVideo = cv

		// libSystem.B.dylib is the unified macOS C library; mach_*
		// symbols live there.
		ls, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			cvLoadErr = fmt.Errorf("dlopen libSystem: %w", err)
			return
		}
		cvLibSystem = ls

		purego.RegisterLibFunc(&cvAPI.CVDisplayLinkCreateWithActiveCGDisplays, cv,
			"CVDisplayLinkCreateWithActiveCGDisplays")
		purego.RegisterLibFunc(&cvAPI.CVDisplayLinkSetOutputCallback, cv,
			"CVDisplayLinkSetOutputCallback")
		purego.RegisterLibFunc(&cvAPI.CVDisplayLinkStart, cv, "CVDisplayLinkStart")
		purego.RegisterLibFunc(&cvAPI.CVDisplayLinkStop, cv, "CVDisplayLinkStop")
		purego.RegisterLibFunc(&cvAPI.CVDisplayLinkRelease, cv, "CVDisplayLinkRelease")
		purego.RegisterLibFunc(&cvAPI.MachTimebaseInfo, ls, "mach_timebase_info")
		purego.RegisterLibFunc(&cvAPI.MachAbsoluteTime, ls, "mach_absolute_time")
	})
	return cvLoadErr
}

// cvBackend is the CVDisplayLink Timer implementation.
//
// One backend instance owns one CVDisplayLinkRef and one callback slot.
// The callback writes vsync timestamps into a fixed-size ring buffer;
// OnsetForFlip scans the buffer for the smallest entry >= flipTS.
type cvBackend struct {
	link uintptr // CVDisplayLinkRef

	// Ring buffer, written by the CVDisplayLink callback (background
	// thread), read by OnsetForFlip (main thread). All values are SDL
	// ticks, never zero (we coerce a 0 reading to 1 to keep "0 = unset"
	// sentinel semantics).
	mu      sync.Mutex
	ring    [vsyncRingSize]uint64
	ringIdx int
	closed  atomic.Bool

	// mach_absolute_time → SDL ticks conversion: sdl_ns = mach_ns - epochOffsetNS.
	// machRatioNumer/Denom are mach_timebase_info; the conversion is
	// nanoseconds = mach_time * Numer / Denom, performed in int64.
	machRatioNumer uint64
	machRatioDenom uint64
	epochOffsetNS  int64 // sdl_ticks - mach_ns at construction

	// callbackKey holds the C function pointer purego allocated for our
	// Go callback; we keep it pinned for the backend's lifetime so the
	// trampoline stays valid.
	callbackPtr uintptr
}

// activeBackend is the single live backend; the C callback can only
// pass a void* context (uintptr) which we pass through, but we also
// guard against torn dlclose ordering by using a package-level pointer.
// Since exactly one MovieManager normally exists, allowing only one
// active CVDisplayLink at a time is a non-restriction; the package
// returns an error if a second backend is constructed before the first
// is closed.
var (
	activeBackend atomic.Pointer[cvBackend]
)

func newCVDisplayLinkBackend(_ *apparatus.Screen) (Timer, error) {
	if err := loadCVAPI(); err != nil {
		return nil, err
	}

	// mach_timebase_info: nanoseconds = mach_time * numer / denom.
	var tb machTimebase
	if rc := cvAPI.MachTimebaseInfo(&tb); rc != 0 {
		return nil, fmt.Errorf("mach_timebase_info failed (kern_return=%d)", rc)
	}
	if tb.Numer == 0 || tb.Denom == 0 {
		return nil, fmt.Errorf("mach_timebase_info returned zeros (numer=%d denom=%d)", tb.Numer, tb.Denom)
	}

	// Compute the epoch offset between mach_absolute_time (ns) and
	// sdl.TicksNS. Both are monotonic, both are in nanoseconds; the
	// difference is constant (within the few-ns scheduling jitter
	// between the two calls below). We capture it once.
	machBefore := cvAPI.MachAbsoluteTime()
	sdlNow := sdl.TicksNS()
	machAfter := cvAPI.MachAbsoluteTime()
	machNow := (machBefore + machAfter) / 2 // average to halve sample-time error
	machNS := machToNS(machNow, uint64(tb.Numer), uint64(tb.Denom))
	epochOffset := int64(sdlNow) - int64(machNS)

	b := &cvBackend{
		machRatioNumer: uint64(tb.Numer),
		machRatioDenom: uint64(tb.Denom),
		epochOffsetNS:  epochOffset,
	}

	// Refuse to construct if another backend is alive — only one
	// CVDisplayLink callback context can be installed safely at a time
	// without a more elaborate dispatch table.
	if !activeBackend.CompareAndSwap(nil, b) {
		return nil, fmt.Errorf("another CVDisplayLink backend is already active; close it before creating a new one")
	}

	// Create the link.
	var link uintptr
	if rc := cvAPI.CVDisplayLinkCreateWithActiveCGDisplays(&link); rc != 0 || link == 0 {
		activeBackend.Store(nil)
		return nil, fmt.Errorf("CVDisplayLinkCreateWithActiveCGDisplays failed (rc=%d)", rc)
	}
	b.link = link

	// Allocate a C callback trampoline pointing at our Go function.
	// purego.NewCallback creates a stable function pointer; keep it
	// pinned for the backend's lifetime.
	b.callbackPtr = purego.NewCallback(cvDisplayLinkCallback)
	if rc := cvAPI.CVDisplayLinkSetOutputCallback(link, b.callbackPtr, 0); rc != 0 {
		cvAPI.CVDisplayLinkRelease(link)
		activeBackend.Store(nil)
		return nil, fmt.Errorf("CVDisplayLinkSetOutputCallback failed (rc=%d)", rc)
	}

	if rc := cvAPI.CVDisplayLinkStart(link); rc != 0 {
		cvAPI.CVDisplayLinkRelease(link)
		activeBackend.Store(nil)
		return nil, fmt.Errorf("CVDisplayLinkStart failed (rc=%d)", rc)
	}

	return b, nil
}

// cvDisplayLinkCallback is the C-callable trampoline. It runs on a
// background CV thread for every display refresh. inOutputTime points
// at a CVTimeStamp whose HostTime is the mach time of the upcoming
// frame display.
//
// Signature: CVReturn (*)(CVDisplayLinkRef, const CVTimeStamp*, const CVTimeStamp*, CVOptionFlags, CVOptionFlags*, void*)
func cvDisplayLinkCallback(displayLink uintptr, inNow uintptr, inOutputTime uintptr, flagsIn uint64, flagsOut uintptr, ctx uintptr) int32 {
	b := activeBackend.Load()
	if b == nil || b.closed.Load() || inOutputTime == 0 {
		return 0 // kCVReturnSuccess
	}
	out := (*cvTimeStamp)(unsafe.Pointer(inOutputTime))
	hostTime := out.HostTime
	if hostTime == 0 {
		return 0
	}
	machNS := machToNS(hostTime, b.machRatioNumer, b.machRatioDenom)
	sdlTicks := uint64(int64(machNS) + b.epochOffsetNS)
	if sdlTicks == 0 {
		sdlTicks = 1 // preserve "0 = unset" sentinel
	}
	b.mu.Lock()
	b.ring[b.ringIdx] = sdlTicks
	b.ringIdx = (b.ringIdx + 1) % vsyncRingSize
	b.mu.Unlock()
	return 0
}

// machToNS converts a mach_absolute_time value to nanoseconds via the
// mach_timebase_info (numer, denom) ratio. Uses int64 for safety against
// overflow at the edge of typical mach_time ranges.
func machToNS(mach, numer, denom uint64) uint64 {
	if denom == 0 {
		return 0
	}
	// (a * b) may overflow uint64 if mach is near 2^63 and numer is
	// large. Split mach into hi/lo against denom to keep the multiply in
	// range while preserving exact integer arithmetic.
	hi := mach / denom
	lo := mach % denom
	return hi*numer + (lo*numer)/denom
}

func (b *cvBackend) RecordFlip(uint64) {
	// The CVDisplayLink callback publishes vsync times asynchronously;
	// no synchronous query is needed here.
}

func (b *cvBackend) OnsetForFlip(flipTS uint64) (uint64, OnsetSource, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var best uint64
	for _, v := range b.ring {
		if v == 0 {
			continue
		}
		if v >= flipTS && (best == 0 || v < best) {
			best = v
		}
	}
	if best == 0 {
		return 0, VsyncEstimated, false
	}
	return best, HardwareVerified, true
}

func (b *cvBackend) Precision() OnsetSource { return HardwareVerified }

func (b *cvBackend) Description() string {
	return "macOS CVDisplayLink (CoreVideo, hardware-verified)"
}

func (b *cvBackend) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}
	defer activeBackend.Store(nil)
	if b.link != 0 {
		_ = cvAPI.CVDisplayLinkStop(b.link)
		cvAPI.CVDisplayLinkRelease(b.link)
		b.link = 0
	}
	// Wait briefly for any in-flight callback to observe closed=true and
	// return. The callback is cheap (mutex + array write) and CV's
	// thread will exit Stop before we Release, so a short pause is
	// belt-and-braces.
	time.Sleep(2 * time.Millisecond)
	return nil
}
