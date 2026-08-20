#! /usr/bin/env python
# Time-stamp: <2026-08-20 christophe@pallier.org>
"""Create the wedge and ring checkerboard stimuli of a retinotopic mapping run.

Writes 16 PNG files of `SIZE` x `SIZE` pixels:

  wedge_1.png .. wedge_8.png   45-degree sectors centred on the polar angles
                               listed in `WEDGE_ANGLES_DEG` (same positions as
                               the disks of generate-disks.py), tiling the
                               whole circle.
  ring_1.png .. ring_8.png     concentric annuli, ring 1 innermost.

All 16 apertures show the same underlying polar checkerboard, so a wedge and a
ring that overlap show identical checks there.  Background is black, matching
assets/*.png and the disks -- see the BACKGROUND constant.

Checkerboard geometry follows Lu & Dosher (2014) _Visual Psychophysics_ (p. 40):
the checks are laid out in log-polar coordinates so that they scale with
eccentricity.
"""

import numpy as np
from PIL import Image

# --- canvas -----------------------------------------------------------------
SIZE = 800            # output image is SIZE x SIZE pixels
R_MAX = SIZE / 2      # outer radius of the stimulus (inscribed circle)
R_MIN = 20            # radius of the central hole left free for fixation
SS = 4                # supersampling factor used to antialias the edges

# --- wedges -----------------------------------------------------------------
WEDGE_ANGLES_DEG = [22.5, 67.5, 112.5, 157.5, 202.5, 247.5, 292.5, 337.5]
WEDGE_WIDTH_DEG = 45.0

# --- rings ------------------------------------------------------------------
N_RINGS = 8
RING_SPACING = "log"  # "log" (eccentricity-scaled, as in retinotopy) or "linear"

# --- checkerboard -----------------------------------------------------------
N_ANGULAR = 32        # checks around the full circle (4 per 45-degree wedge)
N_RADIAL = 16         # checks from R_MIN to R_MAX (2 per ring)

# Black, to match assets/*.png and the disks: these frames are interleaved
# item-by-item with the other categories, so any difference in background
# luminance would fire a full-field evoked transient at every category switch.
# The consequence is that the black checks merge into the background and each
# aperture reads as a scatter of white checks.
BACKGROUND = 0
BLACK, WHITE = 0, 255

# Also write the polarity-reversed frames (wedge_1_inv.png, ...) that a
# contrast-reversing flicker alternates with.
SAVE_COUNTERPHASE = False


def ring_edges():
    """Return the N_RINGS + 1 radii delimiting the rings, from R_MIN to R_MAX."""
    if RING_SPACING == "log":
        return np.geomspace(R_MIN, R_MAX, N_RINGS + 1)
    if RING_SPACING == "linear":
        return np.linspace(R_MIN, R_MAX, N_RINGS + 1)
    raise ValueError(f"unknown RING_SPACING: {RING_SPACING!r}")


def radial_fraction(radius):
    """Map R_MIN..R_MAX onto 0..1, following RING_SPACING.

    The radial checks use the same rule as the rings, so that a ring boundary
    always falls on a check boundary (N_RADIAL / N_RINGS checks per ring).
    """
    if RING_SPACING == "log":
        return np.log(np.maximum(radius, 1e-9) / R_MIN) / np.log(R_MAX / R_MIN)
    return (radius - R_MIN) / (R_MAX - R_MIN)


# Polar coordinates of every (supersampled) pixel.  As in generate-disks.py the
# vertical axis is flipped so that angles run counter-clockwise from the
# positive x axis: 90 degrees is up on the screen.
n = SIZE * SS
a = (np.arange(n) + 0.5) / SS - SIZE / 2
x, y = np.meshgrid(a, -a)
r = np.hypot(x, y)
theta = np.degrees(np.arctan2(y, x)) % 360.0

# The checkerboard both apertures reveal.
rad_index = np.floor(radial_fraction(r) * N_RADIAL)
ang_index = np.floor(theta / (360.0 / N_ANGULAR))
checks = np.where((rad_index + ang_index) % 2 == 0, WHITE, BLACK).astype(np.uint8)

annulus = (r >= R_MIN) & (r <= R_MAX)


def save(mask, filename):
    """Paint the checkerboard through `mask`, downsample, and write a PNG."""
    img = np.where(mask & annulus, checks, BACKGROUND).astype(np.uint8)
    big = Image.fromarray(img, mode="L")
    big.resize((SIZE, SIZE), Image.Resampling.BOX).save(filename)
    print(f"Saved: {filename}")
    if SAVE_COUNTERPHASE:
        inv = np.where(mask & annulus, WHITE - checks, BACKGROUND).astype(np.uint8)
        stem = filename.removesuffix(".png")
        Image.fromarray(inv, mode="L").resize(
            (SIZE, SIZE), Image.Resampling.BOX).save(f"{stem}_inv.png")
        print(f"Saved: {stem}_inv.png")


for i, angle in enumerate(WEDGE_ANGLES_DEG):
    half = WEDGE_WIDTH_DEG / 2
    # Distance to the wedge centre, wrapped into [-180, 180].
    delta = (theta - angle + 180.0) % 360.0 - 180.0
    save(np.abs(delta) <= half, f"wedge_{i + 1}.png")

edges = ring_edges()
for i in range(N_RINGS):
    save((r >= edges[i]) & (r < edges[i + 1]), f"ring_{i + 1}.png")
