// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build linux

package vblank

import (
	"fmt"
	"syscall"
	"testing"
)

// The two heads of the Precision 5490 capture that motivated CRTC selection.
// Real numbers rather than round ones: the point of the test is that 1449 ppm
// apart is far enough to tell apart, and inventing a cleaner pair would test a
// gap that never occurred.
var (
	edp5490 = crtcInfo{ // 268800 kHz / (2720 x 1646)
		index: 0, id: 150, width: 2560, height: 1600,
		frameNS: 16_655_952, connector: "eDP-1",
	}
	dp5490 = crtcInfo{ // what the photodiode measured on the U2720Q
		index: 1, id: 268, width: 2560, height: 1440,
		frameNS: 16_680_170, connector: "DP-1",
	}
)

func TestModeFrameNSMatchesDRMArithmetic(t *testing.T) {
	// The internal panel's mode, read from the kernel with drm_info.
	m := drmModeInfo{Clock: 268800, HDisplay: 2560, HTotal: 2720, VDisplay: 1600, VTotal: 1646}
	got := modeFrameNS(&m)
	if got != 16_655_952 {
		t.Errorf("modeFrameNS = %d ns (%.5f Hz), want 16655952 ns (60.03860 Hz)", got, hzOf(got))
	}

	// An interlaced mode delivers frames twice as often as the vtotal suggests.
	m.Flags = drmModeFlagInterlace
	if il := modeFrameNS(&m); il != got/2 {
		t.Errorf("interlaced: %d ns, want half of %d", il, got)
	}

	// A mode with no timing must say so rather than dividing by zero.
	if z := modeFrameNS(&drmModeInfo{}); z != 0 {
		t.Errorf("empty mode: %d ns, want 0", z)
	}
}

func TestChooseCRTCPicksTheDisplayBeingPresentedTo(t *testing.T) {
	cands := []crtcInfo{edp5490, dp5490}

	// The failing case: the experiment is on the external monitor and crtc 0,
	// which answers just as readily, is 1449 ppm away.
	got, err := chooseCRTC(cands, Target{FrameNS: dp5490.frameNS, Width: 2560, Height: 1440})
	if err != nil {
		t.Fatalf("choosing the external monitor: %v", err)
	}
	if got.index != dp5490.index {
		t.Errorf("chose crtc %d (%s), want crtc %d (%s)", got.index, got, dp5490.index, dp5490)
	}

	// And the other way round, so the test cannot pass by always answering 1.
	got, err = chooseCRTC(cands, Target{FrameNS: edp5490.frameNS, Width: 2560, Height: 1600})
	if err != nil {
		t.Fatalf("choosing the internal panel: %v", err)
	}
	if got.index != edp5490.index {
		t.Errorf("chose crtc %d (%s), want crtc %d (%s)", got.index, got, edp5490.index, edp5490)
	}
}

func TestChooseCRTCRefusesRatherThanGuessing(t *testing.T) {
	// A display neither pipe is driving: 75 Hz against two 60 Hz heads.
	_, err := chooseCRTC([]crtcInfo{edp5490, dp5490}, Target{FrameNS: 13_333_333})
	if err == nil {
		t.Fatal("a 75 Hz target matched a 60 Hz card; it must refuse instead")
	}
	// The error has to name what the card does drive, or the person reading it
	// cannot tell a wrong -d from an unplugged monitor.
	if want := "eDP-1"; !contains(err.Error(), want) {
		t.Errorf("error %q does not name the heads it saw (%q)", err, want)
	}

	// No target named: also a refusal, so the caller decides whether to fall
	// back to the blind probe rather than this silently doing it.
	if _, err := chooseCRTC([]crtcInfo{dp5490}, Target{}); err == nil {
		t.Fatal("an empty Target must not resolve to a CRTC")
	}
}

func TestChooseCRTCBreaksRateTiesOnSize(t *testing.T) {
	// Two heads cloned to the same rate; only the size tells them apart.
	a := crtcInfo{index: 0, width: 1920, height: 1080, frameNS: 16_666_666, connector: "HDMI-A-1"}
	b := crtcInfo{index: 1, width: 2560, height: 1440, frameNS: 16_666_666, connector: "DP-1"}
	got, err := chooseCRTC([]crtcInfo{a, b}, Target{FrameNS: 16_666_666, Width: 2560, Height: 1440})
	if err != nil {
		t.Fatalf("cloned heads: %v", err)
	}
	if got.index != b.index {
		t.Errorf("chose %s, want %s", got, b)
	}
}

func TestStatsWrongDisplay(t *testing.T) {
	// The 5490 run: reading a 60.0386 Hz pipe while presenting to 59.9514 Hz.
	st := Stats{TargetFrameNS: dp5490.frameNS, MeasuredFrameNS: edp5490.frameNS}
	ppm, ok := st.MismatchPPM()
	if !ok {
		t.Fatal("both periods known, MismatchPPM said otherwise")
	}
	if ppm > -1400 || ppm < -1500 {
		t.Errorf("mismatch %+d ppm, want about -1449", ppm)
	}
	if !st.WrongDisplay() {
		t.Error("a 1449 ppm gap must read as the wrong display")
	}

	// A panel a hundred ppm off its own nominal is normal and must not trip it.
	st = Stats{TargetFrameNS: 16_666_666, MeasuredFrameNS: 16_668_333} // +100 ppm
	if st.WrongDisplay() {
		t.Error("100 ppm is a panel's own crystal, not a different display")
	}

	// Nothing measured yet: "no evidence", not "verified".
	if (Stats{TargetFrameNS: 16_666_666}).WrongDisplay() {
		t.Error("an unmeasured run must not be reported as the wrong display")
	}
}

// TestListActiveCRTCsOnThisMachine exercises the four mode ioctls against the
// real kernel, which is the only way to check the struct layouts and ioctl
// numbers: a wrong size returns EINVAL, and EINVAL is indistinguishable from
// "this machine has no display" unless something asserts otherwise.
//
// Skips where there is no readable DRM node, so it is silent on CI.
func TestListActiveCRTCsOnThisMachine(t *testing.T) {
	var lastErr error
	for card := 0; card < maxCards; card++ {
		p := fmt.Sprintf("/dev/dri/card%d", card)
		fd, err := syscall.Open(p, syscall.O_RDWR|syscall.O_CLOEXEC, 0)
		if err != nil {
			continue
		}
		cands, err := listActiveCRTCs(fd)
		_ = syscall.Close(fd)
		if err != nil {
			lastErr = err
			continue
		}
		for _, c := range cands {
			t.Logf("%s crtc %d: %s", p, c.index, c)
			if c.frameNS < 4_000_000 || c.frameNS > 100_000_000 {
				t.Errorf("%s crtc %d: frame period %d ns is outside 10-250 Hz; "+
					"the mode struct is probably misaligned", p, c.index, c.frameNS)
			}
		}
		return
	}
	t.Skipf("no DRM node with a lit CRTC here (last error: %v)", lastErr)
}

// TestNewDRMBackendRefusesAnUnmatchedDisplay drives the whole selection path on
// the real machine and checks the guard, not just the picker: a caller naming a
// display no lit CRTC is running must get an error, so the Screen falls back to
// present-return anchoring instead of timing some other monitor.
//
// The error is returned before newBackendOn runs, so this needs no SDL.
func TestNewDRMBackendRefusesAnUnmatchedDisplay(t *testing.T) {
	if !anyCardHasLitCRTCs() {
		t.Skip("no DRM node with a lit CRTC here")
	}
	// 75 Hz. Every head on a machine that reaches this point runs something
	// else, and 75 vs 60 is 250000 ppm — no tolerance can span it.
	timer, err := newDRMBackend(Target{FrameNS: 13_333_333, Width: 1280, Height: 1024})
	if err == nil {
		_ = timer.Close()
		t.Fatal("a display no CRTC is driving produced a backend; it must refuse")
	}
	t.Logf("refused, as it should: %v", err)
}

func anyCardHasLitCRTCs() bool {
	for card := 0; card < maxCards; card++ {
		fd, err := syscall.Open(fmt.Sprintf("/dev/dri/card%d", card),
			syscall.O_RDWR|syscall.O_CLOEXEC, 0)
		if err != nil {
			continue
		}
		_, lerr := listActiveCRTCs(fd)
		_ = syscall.Close(fd)
		if lerr == nil {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
