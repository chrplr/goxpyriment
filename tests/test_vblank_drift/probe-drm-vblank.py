#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

"""Find which DRM card and CRTC can answer DRM_IOCTL_WAIT_VBLANK.

test_vblank_drift needs a kernel vblank clock, and when it cannot get one it
says so and stops. This says WHY, which the test itself cannot: it walks every
/dev/dri/card* and every CRTC 0-3 and prints the errno for each pair.

It is deliberately dependency-free — no repo, no Go toolchain, no build — so it
can be the first thing run on a machine that will not run the test.

    python3 probe-drm-vblank.py

Reading the result:

  some pair OK, sequence advancing
      The hardware is fine and the Go backend is looking in the wrong place.
      media/present/drm_linux.go opens the first card that OPENS and gives up if
      that card's ioctl fails, instead of trying the next one; and it never sets
      the CRTC bits, so it always asks CRTC 0. Either is enough to fail here
      while working on a machine that happens to enumerate favourably.

  every pair EACCES
      DRM authentication, not a bug. Under X11 the X server holds DRM master and
      an unauthenticated client is refused. Needs a different route to the vblank
      clock, which is a genuine constraint rather than something to patch.

  every pair ENOTSUP / EINVAL
      The driver exposes no vblank on those CRTCs. Check that a display is
      actually attached and lit on this card, and compare against
      /sys/class/drm/.

  open fails with EACCES/EPERM
      Plain file permissions. Usually membership of the 'video' group; a local
      login often grants it through a logind ACL instead.

TearFree, and X server configuration generally, has no bearing on any of this:
the ioctl goes straight to the kernel.
"""

import errno
import fcntl
import glob
import os
import struct
import time

# DRM_IOCTL_WAIT_VBLANK = _IOWR('d', 0x3a, union drm_wait_vblank), 24 bytes.
DRM_IOCTL_WAIT_VBLANK = 0xC018643A
DRM_VBLANK_RELATIVE = 0x1

# The CRTC index lives in bits 1-5 of the type field:
#   #define _DRM_VBLANK_HIGH_CRTC_SHIFT 1
#   #define _DRM_VBLANK_HIGH_CRTC_MASK  0x0000003e
HIGH_CRTC_SHIFT = 1
HIGH_CRTC_MASK = 0x3E

MAX_CRTC = 4


def query(fd, crtc):
    """Return (sequence, timestamp_seconds) for the most recent vblank."""
    req_type = DRM_VBLANK_RELATIVE | ((crtc << HIGH_CRTC_SHIFT) & HIGH_CRTC_MASK)
    buf = bytearray(struct.pack("II", req_type, 0) + b"\0" * 16)
    fcntl.ioctl(fd, DRM_IOCTL_WAIT_VBLANK, buf)
    _, seq = struct.unpack_from("II", buf, 0)
    sec, usec = struct.unpack_from("qq", buf, 8)
    return seq, sec + usec / 1e6


def errname(e):
    return errno.errorcode.get(e.errno, str(e.errno))


def main():
    cards = sorted(glob.glob("/dev/dri/card*"))
    if not cards:
        print("no /dev/dri/card* at all — no DRM device on this system")
        return

    working = []
    for path in cards:
        try:
            fd = os.open(path, os.O_RDWR)
        except OSError as e:
            print(f"{path}: open failed: {e.strerror} ({errname(e)})")
            continue
        print(f"{path}: opened")
        try:
            for crtc in range(MAX_CRTC):
                try:
                    seq, ts = query(fd, crtc)
                except OSError as e:
                    print(f"    crtc {crtc}: {e.strerror} ({errname(e)})")
                    continue
                print(f"    crtc {crtc}: OK   seq={seq}  ts={ts:.6f}")
                working.append((path, crtc, seq, ts))
        finally:
            os.close(fd)

    if not working:
        print("\nNo card/CRTC pair answered. See the errno guidance in this file's header.")
        return

    # A sequence that does not advance is not a usable clock, however cleanly the
    # ioctl returned — an idle or blanked CRTC answers without counting.
    print("\nRe-checking after 0.5 s to see which of those actually advance:")
    time.sleep(0.5)
    for path, crtc, seq0, ts0 in working:
        fd = os.open(path, os.O_RDWR)
        try:
            seq1, ts1 = query(fd, crtc)
        finally:
            os.close(fd)
        dseq, dt = seq1 - seq0, ts1 - ts0
        if dseq > 0 and dt > 0:
            print(f"  {path} crtc {crtc}: {dseq} vblanks in {dt:.4f} s -> {dseq / dt:.4f} Hz  USABLE")
        else:
            print(f"  {path} crtc {crtc}: sequence did not advance (dseq={dseq}) — idle or blanked CRTC")


if __name__ == "__main__":
    main()
