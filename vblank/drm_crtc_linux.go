// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build linux

// Choosing WHICH CRTC to read vblanks from.
//
// # The bug this exists to prevent
//
// The vblank ioctl names a CRTC by index, and the backend used to take the
// first index that answered. On a machine with one display that is right by
// luck; on a laptop with an external monitor it is right half the time.
//
// Measured on a Dell Precision 5490 (i915, kmsdrm on a bare VT) presenting to a
// U2720Q on DP-1, with the internal 2560x1600 panel also lit on crtc 0:
//
//	crtc 0  eDP-1  2560x1600  268800 kHz / (2720 x 1646)  =  60.03860 Hz
//	crtc 1  DP-1   2560x1440                              =  59.95142 Hz  (photodiode)
//
// The backend read crtc 0. Both pipes answer the ioctl, so nothing complained.
// The flip timestamps then advanced on the internal panel's grid — 1449 ppm
// fast — while the photons came out on the external one, so the recorded onset
// walked 24.2 us per frame until it was a whole frame out and the loop stalled a
// frame to catch up. Over an 8.4 minute capture: 44 stalls, a flip-to-photon lag
// sawtoothing across 16.83 ms (38.33 to 55.16 ms, one full frame peak to peak),
// 44 cycles whose period column read 516.34 ms instead of 499.68, and 16 whose
// bright_duration read 216.60 ms instead of 199.88. The presents themselves were
// perfect — the photodiode saw an unbroken 30-frames-per-cycle lock with 3 us
// stability — so nothing in the run's own numbers said the timestamps were on
// the wrong display. `onset_source` said `hardware-verified` in all 1010 rows,
// which is exactly the wrong reassurance: the hardware was real, it was just the
// wrong hardware.
//
// # How the right one is found
//
// DRM_IOCTL_MODE_GETRESOURCES lists the card's CRTCs, and DRM_IOCTL_MODE_GETCRTC
// gives each one its programmed mode — pixel clock, htotal, vtotal, and the
// visible size. That yields the exact frame period of every lit pipe with no
// waiting and no guessing, and the caller says which display it is presenting to
// (see Target). The pipe whose mode matches is the pipe to read.
//
// Matching on the MODE rather than on the connector is deliberate. The obvious
// route — connector -> encoder -> crtc_id — resolves the chain correctly and
// then leaves the real question open, because SDL does not publish which
// connector its window is on. The mode does not need it: SDL's refresh rate and
// resolution come from this same kernel mode by way of kmsdrm, randr or
// wl_output, so the correct pipe matches to a handful of ppm and a wrong one
// misses by hundreds. The connector chain is still walked, but only to put a
// name — "DP-1", "eDP-1" — in the backend's Description, so that a wrong choice
// is legible in the data file rather than inferred from a drift plot eight
// minutes later.
//
// If no lit CRTC matches, that is reported as an error rather than resolved by
// picking the closest: falling back to present-return anchoring is a known
// quantity, and reading a display the experiment is not showing anything on is
// not.
//
// Reference: include/uapi/drm/drm_mode.h. Every struct size and ioctl number
// below was taken from the kernel headers on the machine, not from memory, and
// is asserted at compile time.

package vblank

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	// DRM_IOCTL_MODE_GETRESOURCES = DRM_IOWR(0xA0, struct drm_mode_card_res)
	// DRM_IOCTL_MODE_GETCRTC      = DRM_IOWR(0xA1, struct drm_mode_crtc)
	// DRM_IOCTL_MODE_GETENCODER   = DRM_IOWR(0xA6, struct drm_mode_get_encoder)
	// DRM_IOCTL_MODE_GETCONNECTOR = DRM_IOWR(0xA7, struct drm_mode_get_connector)
	//
	// Encoding is (3 << 30) | (size << 16) | ('d' << 8) | nr, with the sizes
	// asserted against the Go structs below.
	drmIoctlModeGetResources uintptr = 0xC04064A0
	drmIoctlModeGetCrtc      uintptr = 0xC06864A1
	drmIoctlModeGetEncoder   uintptr = 0xC01464A6
	drmIoctlModeGetConnector uintptr = 0xC05064A7

	// drm_mode_modeinfo flags that change how long one frame lasts.
	drmModeFlagInterlace uint32 = 0x10
	drmModeFlagDblScan   uint32 = 0x20

	// drm_mode_get_connector.connection == 1 means connected.
	drmModeConnected uint32 = 1

	// The tolerance both the mode match and the live check use is
	// crtcMatchPPM, in vblank.go — it is one policy, and Stats.WrongDisplay
	// applies it on machines this file does not build for.
)

// Compile-time assertions that each struct matches the kernel's. A mismatch
// makes the index negative and the build fails here rather than at run time with
// an EINVAL that would be read as "this machine has no vblank clock".
var (
	_ [1]struct{} = [1]struct{}{}
	_             = [1]struct{}{}[unsafe.Sizeof(drmModeCardRes{})-64]
	_             = [1]struct{}{}[unsafe.Sizeof(drmModeInfo{})-68]
	_             = [1]struct{}{}[unsafe.Sizeof(drmModeCrtc{})-104]
	_             = [1]struct{}{}[unsafe.Sizeof(drmModeGetEncoder{})-20]
	_             = [1]struct{}{}[unsafe.Sizeof(drmModeGetConnector{})-80]
)

// drmModeCardRes mirrors struct drm_mode_card_res (64 bytes).
//
// Called twice: once with the pointers zero to learn the counts, once with them
// pointing at slices big enough to receive the ids.
type drmModeCardRes struct {
	FbIDPtr         uint64
	CrtcIDPtr       uint64
	ConnectorIDPtr  uint64
	EncoderIDPtr    uint64
	CountFbs        uint32
	CountCrtcs      uint32
	CountConnectors uint32
	CountEncoders   uint32
	MinWidth        uint32
	MaxWidth        uint32
	MinHeight       uint32
	MaxHeight       uint32
}

// drmModeInfo mirrors struct drm_mode_modeinfo (68 bytes).
type drmModeInfo struct {
	Clock      uint32 // pixel clock in kHz
	HDisplay   uint16
	HSyncStart uint16
	HSyncEnd   uint16
	HTotal     uint16
	HSkew      uint16
	VDisplay   uint16
	VSyncStart uint16
	VSyncEnd   uint16
	VTotal     uint16
	VScan      uint16
	VRefresh   uint32 // the kernel's own rounded Hz; not used, see modeFrameNS
	Flags      uint32
	Type       uint32
	Name       [32]byte
}

// drmModeCrtc mirrors struct drm_mode_crtc (104 bytes).
type drmModeCrtc struct {
	SetConnectorsPtr uint64
	CountConnectors  uint32
	CrtcID           uint32
	FbID             uint32
	X                uint32
	Y                uint32
	GammaSize        uint32
	ModeValid        uint32
	Mode             drmModeInfo
}

// drmModeGetEncoder mirrors struct drm_mode_get_encoder (20 bytes).
type drmModeGetEncoder struct {
	EncoderID      uint32
	EncoderType    uint32
	CrtcID         uint32
	PossibleCrtcs  uint32
	PossibleClones uint32
}

// drmModeGetConnector mirrors struct drm_mode_get_connector (80 bytes).
type drmModeGetConnector struct {
	EncodersPtr     uint64
	ModesPtr        uint64
	PropsPtr        uint64
	PropValuesPtr   uint64
	CountModes      uint32
	CountProps      uint32
	CountEncoders   uint32
	EncoderID       uint32
	ConnectorID     uint32
	ConnectorType   uint32
	ConnectorTypeID uint32
	Connection      uint32
	MmWidth         uint32
	MmHeight        uint32
	Subpixel        uint32
	Pad             uint32
}

// connectorTypeNames indexes DRM_MODE_CONNECTOR_* and spells each one the way
// the kernel does in /sys/class/drm, so a name printed here can be pasted
// straight into a path.
var connectorTypeNames = []string{
	"Unknown", "VGA", "DVI-I", "DVI-D", "DVI-A", "Composite", "SVIDEO",
	"LVDS", "Component", "DIN", "DP", "HDMI-A", "HDMI-B", "TV", "eDP",
	"Virtual", "DSI", "DPI", "Writeback", "SPI", "USB",
}

func connectorName(typ, typeID uint32) string {
	name := "Unknown"
	if int(typ) < len(connectorTypeNames) {
		name = connectorTypeNames[typ]
	}
	return fmt.Sprintf("%s-%d", name, typeID)
}

// crtcInfo describes one lit CRTC on a card.
type crtcInfo struct {
	// index is the position in the resources' CRTC id array, which is what the
	// vblank ioctl's high-crtc bits name. It is NOT the crtc id.
	index     uint32
	id        uint32
	width     int
	height    int
	frameNS   uint64 // from the programmed mode, exact
	connector string // "DP-1"; empty when the chain could not be walked
}

// String names the head this CRTC is driving. It degrades rather than lying:
// the blind probe fills in nothing but the index, and saying so is better than
// printing a plausible 0x0@0.0000 Hz.
func (c crtcInfo) String() string {
	if c.frameNS == 0 {
		if c.connector != "" {
			return c.connector
		}
		return "an unidentified head"
	}
	who := c.connector
	if who == "" {
		who = fmt.Sprintf("crtc id %d", c.id)
	}
	return fmt.Sprintf("%s %dx%d@%.4f Hz", who, c.width, c.height, hzOf(c.frameNS))
}

// hzOf converts a frame period to Hz, returning 0 for an unset period.
func hzOf(frameNS uint64) float64 {
	if frameNS == 0 {
		return 0
	}
	return 1e9 / float64(frameNS)
}

// modeFrameNS returns the exact duration of one frame of mode m, in
// nanoseconds, or 0 if the mode carries no usable timing.
//
// The arithmetic is drm_mode_vrefresh's, inverted and kept in integers: period =
// htotal * vtotal / (clock kHz * 1000). The three adjustments are the same ones
// the kernel applies, and they are here rather than assumed away because a
// caller that hit an interlaced mode would otherwise be handed a period exactly
// twice too long with nothing to say so.
func modeFrameNS(m *drmModeInfo) uint64 {
	if m.Clock == 0 || m.HTotal == 0 || m.VTotal == 0 {
		return 0
	}
	// htotal*vtotal is under 1e8 for any real mode, so the 1e6 scaling stays
	// far inside int64.
	num := int64(m.HTotal) * int64(m.VTotal) * 1_000_000
	if m.Flags&drmModeFlagInterlace != 0 {
		num /= 2 // two fields make one frame, so frames come twice as often
	}
	if m.Flags&drmModeFlagDblScan != 0 {
		num *= 2
	}
	if m.VScan > 1 {
		num *= int64(m.VScan)
	}
	den := int64(m.Clock)
	return uint64((num + den/2) / den)
}

// ioctlPtr issues one ioctl on fd, retrying EINTR because the Go runtime's
// preemption signals land here and every request below is a pure read.
func ioctlPtr(fd int, req uintptr, arg unsafe.Pointer) error {
	for try := 0; try < drmWaitRetries; try++ {
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
		if errno == syscall.EINTR || errno == syscall.EAGAIN {
			continue
		}
		if errno != 0 {
			return errno
		}
		return nil
	}
	return syscall.EINTR
}

// listActiveCRTCs returns every CRTC on fd that is currently driving a mode,
// with the connector name attached where the encoder chain resolves.
//
// A card that answers no mode ioctls at all — a render node, or a kernel that
// refuses a non-master fd — returns an error, which the caller treats as "this
// node cannot be interrogated" rather than "this node has no displays".
func listActiveCRTCs(fd int) ([]crtcInfo, error) {
	var res drmModeCardRes
	if err := ioctlPtr(fd, drmIoctlModeGetResources, unsafe.Pointer(&res)); err != nil {
		return nil, fmt.Errorf("GETRESOURCES: %w", err)
	}
	if res.CountCrtcs == 0 {
		return nil, fmt.Errorf("GETRESOURCES: no CRTCs")
	}

	crtcIDs := make([]uint32, res.CountCrtcs)
	connIDs := make([]uint32, res.CountConnectors)
	req := drmModeCardRes{
		CrtcIDPtr:       uint64(uintptr(unsafe.Pointer(&crtcIDs[0]))),
		CountCrtcs:      res.CountCrtcs,
		CountConnectors: res.CountConnectors,
	}
	if res.CountConnectors > 0 {
		req.ConnectorIDPtr = uint64(uintptr(unsafe.Pointer(&connIDs[0])))
	}
	err := ioctlPtr(fd, drmIoctlModeGetResources, unsafe.Pointer(&req))
	// The id arrays are reachable only through integer fields, so the runtime
	// cannot see that the kernel is writing into them.
	runtime.KeepAlive(crtcIDs)
	runtime.KeepAlive(connIDs)
	if err != nil {
		return nil, fmt.Errorf("GETRESOURCES (ids): %w", err)
	}

	names := connectorNamesByCRTC(fd, connIDs)

	var out []crtcInfo
	for i, id := range crtcIDs {
		c := drmModeCrtc{CrtcID: id}
		if err := ioctlPtr(fd, drmIoctlModeGetCrtc, unsafe.Pointer(&c)); err != nil {
			continue
		}
		if c.ModeValid == 0 {
			continue // pipe is off
		}
		frame := modeFrameNS(&c.Mode)
		if frame == 0 {
			continue
		}
		out = append(out, crtcInfo{
			index:     uint32(i),
			id:        id,
			width:     int(c.Mode.HDisplay),
			height:    int(c.Mode.VDisplay),
			frameNS:   frame,
			connector: names[id],
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no CRTC is driving a mode")
	}
	return out, nil
}

// connectorNamesByCRTC maps crtc id -> connector name by walking each connected
// connector's current encoder. Best effort: an unresolved chain costs a name in
// a log line and nothing else, so every error here is a skip.
func connectorNamesByCRTC(fd int, connIDs []uint32) map[uint32]string {
	names := make(map[uint32]string, len(connIDs))
	for _, id := range connIDs {
		// CountModes non-zero asks for the cached state instead of a fresh
		// probe. A probe re-reads the EDID, which takes milliseconds per
		// connector and can wake a sleeping sink — an unreasonable side effect
		// for a call that only wants a name. libdrm's
		// drmModeGetConnectorCurrent does the same thing for the same reason.
		conn := drmModeGetConnector{ConnectorID: id, CountModes: 1}
		if err := ioctlPtr(fd, drmIoctlModeGetConnector, unsafe.Pointer(&conn)); err != nil {
			continue
		}
		if conn.Connection != drmModeConnected || conn.EncoderID == 0 {
			continue
		}
		enc := drmModeGetEncoder{EncoderID: conn.EncoderID}
		if err := ioctlPtr(fd, drmIoctlModeGetEncoder, unsafe.Pointer(&enc)); err != nil {
			continue
		}
		if enc.CrtcID == 0 {
			continue
		}
		names[enc.CrtcID] = connectorName(conn.ConnectorType, conn.ConnectorTypeID)
	}
	return names
}

// ppmApart returns how far got sits from want, in parts per million.
func ppmApart(got, want uint64) int64 {
	if want == 0 {
		return 0
	}
	return (int64(got) - int64(want)) * 1_000_000 / int64(want)
}

// chooseCRTC picks the CRTC driving the display the caller named.
//
// The size is used only to break ties: two heads can run the same mode, in which
// case either answers correctly for timing purposes, and a machine that reports
// a scaled logical size would otherwise disqualify the right pipe over a
// cosmetic mismatch.
func chooseCRTC(cands []crtcInfo, t Target) (crtcInfo, error) {
	if t.FrameNS == 0 {
		// Nothing to match against. Saying so is the honest outcome; the caller
		// decides whether to fall back to a blind probe.
		return crtcInfo{}, fmt.Errorf("caller named no display to match")
	}

	best := -1
	bestScore := int64(-1)
	for i, c := range cands {
		off := ppmApart(c.frameNS, t.FrameNS)
		if off < 0 {
			off = -off
		}
		if off > crtcMatchPPM {
			continue
		}
		// Lower is better; an exact size match wins any rate tie by more than
		// the tolerance can span.
		score := off
		if t.Width > 0 && (c.width != t.Width || c.height != t.Height) {
			score += 2 * crtcMatchPPM
		}
		if best < 0 || score < bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 {
		var seen []string
		for _, c := range cands {
			seen = append(seen, fmt.Sprintf("%s (%+d ppm)", c, ppmApart(c.frameNS, t.FrameNS)))
		}
		return crtcInfo{}, fmt.Errorf(
			"no lit CRTC runs at %.4f Hz within %d ppm; this card drives %s",
			hzOf(t.FrameNS), crtcMatchPPM, strings.Join(seen, ", "))
	}
	return cands[best], nil
}
