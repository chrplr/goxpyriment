#! /usr/bin/env python
# Time-stamp: <2026-08-20 christophe@pallier.org>
"""Split faces.jpeg -- an AI-generated 8x4 contact sheet -- into 32 PNG files.

Writes face_01.png .. face_32.png, numbered left to right then top to bottom.

The tile boundaries are *detected*, not assumed: the sheet is laid out on a
uniform light wall, so the gaps between tiles are columns (and rows) of near
zero pixel variance.  Columns are found on the whole image, then the rows are
found separately inside each column, because the tiles are not perfectly
aligned from one column to the next.

Every tile is finally centre-cropped to the same square, so the 32 stimuli have
identical dimensions without any resampling.  Set OUTPUT_SIZE to resize them
(310 would match the existing assets/face*.png).
"""

import numpy as np
from PIL import Image
from scipy import ndimage

SOURCE = "faces.jpeg"
N_COLS, N_ROWS = 8, 4
MIN_GAP = 4           # pixels; ignore flat runs shorter than this
OUTPUT_SIZE = None    # None keeps native pixels; an int resizes to a square
GRAYSCALE = True      # the sheet is black and white; store one channel

# Oval aperture.  Masking each face to an ellipse on black removes the light
# wall behind the sheet -- which would otherwise fire a full-field luminance
# transient every time a face follows a checkerboard in the stream -- and with
# it the shoulders and the hair outline, so the condition is carried by the
# face itself.  Standard practice in face localizers.
OVAL = True
OVAL_W, OVAL_H = 0.74, 0.94   # ellipse axes, as a fraction of the tile
OVAL_SHIFT_Y = -0.03          # move the ellipse up (negative) onto the head
FEATHER = 0.07                # width of the cosine roll-off, in radius units
APERTURE_BG = 0               # black, matching the other stimulus sets

# The ellipse alone is not enough: it is wider than a short-haired head, so the
# light wall survives as a bright crescent *inside* the aperture, and how much
# of one depends on the hairstyle -- luminance variance reintroduced through
# the back door.  Narrowing the ellipse does not help (measured: the bright
# fraction stays ~10% and the worst case gets worse), so the wall is segmented
# out instead: the bright region connected to the tile border is blackened.
REMOVE_BACKGROUND = True
BG_TOLERANCE = 15             # how far below the border level still counts as wall
PROTECT_W, PROTECT_H = 0.50, 0.74   # inner ellipse the segmentation may never eat
EDGE_BLUR = 1.2               # soften the segmentation outline, in pixels


def flat_runs(profile, thresh, min_len=MIN_GAP):
    """Start/end of the runs where `profile` stays below `thresh`."""
    out, start = [], None
    for i, low in enumerate(profile < thresh):
        if low and start is None:
            start = i
        elif not low and start is not None:
            if i - start >= min_len:
                out.append((start, i))
            start = None
    if start is not None and len(profile) - start >= min_len:
        out.append((start, len(profile)))
    return out


def split_at(profile, thresh):
    """The content bands left between the flat runs of `profile`."""
    out, prev = [], 0
    for a, b in flat_runs(profile, thresh):
        if a > prev:
            out.append((prev, a))
        prev = b
    if prev < len(profile):
        out.append((prev, len(profile)))
    return out


def bands(profile, n_expected):
    """Cut `profile` into exactly `n_expected` content bands.

    The separating threshold is not a fixed constant -- the tile gaps sit at
    slightly different contrasts across the sheet.  Sweep upward from the
    profile's floor and take the first threshold that isolates the expected
    number of bands of plausible, similar width.
    """
    floor, span = profile.min(), np.median(profile) - profile.min()
    for frac in np.linspace(0.01, 0.4, 200):
        found = [b for b in split_at(profile, floor + frac * span)
                 if b[1] - b[0] > 0.5 * len(profile) / n_expected]
        if len(found) == n_expected:
            return found
    raise SystemExit(f"could not isolate {n_expected} bands in this profile")


def ellipse_distance(side, w, h):
    """Radial distance in ellipse units: 1.0 exactly on the ellipse."""
    t = (np.arange(side) + 0.5) / side - 0.5
    u = t / (w / 2)
    v = (t - OVAL_SHIFT_Y) / (h / 2)
    return np.hypot(u[None, :], v[:, None])


def oval_alpha(side):
    """Opacity map of the feathered ellipse: 1 inside, 0 outside, cosine edge.

    A hard-edged ellipse would introduce its own high spatial frequency
    contour, so the edge rolls off over FEATHER instead of stepping.
    """
    a = (1.0 - ellipse_distance(side, OVAL_W, OVAL_H)) / FEATHER
    return np.clip(0.5 - 0.5 * np.cos(np.pi * np.clip(a, 0, 1)), 0, 1)


def background_alpha(gray, protected):
    """Opacity map that blacks out the wall behind the head.

    The wall is the bright region touching the tile border, so it is found by
    connectivity rather than by luminance alone -- a bright forehead is not
    connected to the edge and survives.  `protected` is the inner ellipse the
    mask may never enter, which matters for the one light-haired face whose
    hair is the same luminance as the wall behind it.
    """
    border = np.concatenate([gray[0], gray[-1], gray[:, 0], gray[:, -1]])
    labels, _ = ndimage.label(gray > np.median(border) - BG_TOLERANCE)
    touching = set(np.concatenate(
        [labels[0], labels[-1], labels[:, 0], labels[:, -1]]).tolist()) - {0}
    wall = np.isin(labels, list(touching)) & ~protected
    return ndimage.gaussian_filter((~wall).astype(float), EDGE_BLUR)


def main():
    img = Image.open(SOURCE)
    gray = np.asarray(img.convert("L"), dtype=float)

    cols = bands(gray.std(axis=0), N_COLS)

    # Rows are located inside each column, ignoring the tile's own margins.
    # Each column yields exactly N_ROWS tiles, so a tile's row is its rank in
    # that column -- never its raw y coordinate, which drifts between columns.
    grid = {}
    for c, (x0, x1) in enumerate(cols):
        inset = int(0.15 * (x1 - x0))
        strip = gray[:, x0 + inset:x1 - inset]
        for r, (y0, y1) in enumerate(bands(strip.std(axis=1), N_ROWS)):
            grid[(r, c)] = (x0, y0, x1, y1)

    tiles = list(grid.values())
    side = min(min(x1 - x0, y1 - y0) for x0, y0, x1, y1 in tiles)
    print(f"{len(tiles)} tiles detected, common square {side}x{side} px")

    # The two masks depend only on the tile size, so build them once.
    oval = oval_alpha(side)
    protected = ellipse_distance(side, PROTECT_W, PROTECT_H) <= 1.0

    # Number left to right, then top to bottom.
    for n, (r, c) in enumerate(sorted(grid), start=1):
        x0, y0, x1, y1 = grid[(r, c)]
        cx, cy = (x0 + x1) // 2, (y0 + y1) // 2
        box = (cx - side // 2, cy - side // 2,
               cx - side // 2 + side, cy - side // 2 + side)
        tile = img.crop(box)
        if GRAYSCALE:
            tile = tile.convert("L")
        if OVAL or REMOVE_BACKGROUND:
            arr = np.asarray(tile, dtype=float)
            alpha = np.ones((side, side))
            if REMOVE_BACKGROUND:
                alpha *= background_alpha(
                    np.asarray(tile.convert("L"), dtype=float), protected)
            if OVAL:
                alpha *= oval
            if arr.ndim == 3:                     # colour: mask every channel
                alpha = alpha[:, :, None]
            masked = arr * alpha + APERTURE_BG * (1.0 - alpha)
            tile = Image.fromarray(np.round(masked).astype(np.uint8), tile.mode)
        if OUTPUT_SIZE:
            tile = tile.resize((OUTPUT_SIZE, OUTPUT_SIZE), Image.Resampling.LANCZOS)
        name = f"face_{n:02d}.png"
        tile.save(name)
        print(f"Saved: {name}  row {r+1} col {c+1}  "
              f"tile ({x0},{y0})-({x1},{y1}) {x1-x0}x{y1-y0}")


if __name__ == "__main__":
    main()
