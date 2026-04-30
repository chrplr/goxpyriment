// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build linux

package present

import (
	"testing"
	"unsafe"
)

func TestMonotonicNSWorks(t *testing.T) {
	a, err := monotonicNS()
	if err != nil {
		t.Fatalf("monotonicNS: %v", err)
	}
	b, err := monotonicNS()
	if err != nil {
		t.Fatalf("monotonicNS: %v", err)
	}
	if a == 0 {
		t.Error("monotonicNS returned 0")
	}
	if b < a {
		t.Errorf("monotonicNS should be monotonic, got %d then %d", a, b)
	}
}

// TestDrmWaitVblankLayout pins the binary layout of drmWaitVblank to
// the kernel's `union drm_wait_vblank` (24 bytes total). If Go ever
// changes struct padding rules, this test catches the breakage at
// compile/test time rather than at first vsync query in production.
func TestDrmWaitVblankLayout(t *testing.T) {
	const want = 24
	got := unsafe.Sizeof(drmWaitVblank{})
	if got != want {
		t.Errorf("drmWaitVblank size = %d bytes, want %d", got, want)
	}
	// Type and Sequence must occupy the first 8 bytes (matches reply
	// layout where tval_sec starts at offset 8 within Data).
	v := drmWaitVblank{Type: 1, Sequence: 2}
	v.Data[0] = 0xAA
	if v.Type != 1 || v.Sequence != 2 || v.Data[0] != 0xAA {
		t.Error("drmWaitVblank fields are not independent — layout drift")
	}
}
