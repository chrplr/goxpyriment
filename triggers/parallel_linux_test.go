// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build linux

package triggers

import "testing"

// TestPpdevIoctlNumbers pins the ppdev request numbers to the values in
// <linux/ppdev.h>.
//
// These were wrong from the first commit to 2026-08-17 — nr = 11, 12, 4, 3
// instead of 0x8b, 0x8c, 0x86, 0x81 — so PPCLAIM was an ioctl the kernel does
// not implement and opening a port always failed with EINVAL. Nothing caught it
// because the only test that exercises this path needs an LPT port, and the
// machines in the campaign used GPIO or USB.
//
// A unit test cannot claim a port, but it can refuse to let the numbers drift
// again. The expected values are the header's, computed by hand once:
//
//	_IO (p,0x8b)        = 0x0000708b
//	_IO (p,0x8c)        = 0x0000708c
//	_IOW(p,0x86,uchar)  = 0x40017086
//	_IOR(p,0x81,uchar)  = 0x80017081
//	_IOW(p,0x90,int)    = 0x40047090
func TestPpdevIoctlNumbers(t *testing.T) {
	for _, c := range []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"PPCLAIM", ppClaim, 0x0000708b},
		{"PPRELEASE", ppRelease, 0x0000708c},
		{"PPWDATA", ppwData, 0x40017086},
		{"PPRSTATUS", pprStatus, 0x80017081},
		{"PPDATADIR", ppDataDir, 0x40047090},
	} {
		if c.got != c.want {
			t.Errorf("%s = %#08x, want %#08x (see /usr/include/linux/ppdev.h)",
				c.name, c.got, c.want)
		}
	}
}
