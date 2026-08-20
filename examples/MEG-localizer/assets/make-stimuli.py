#! /usr/bin/env python
# Time-stamp: <2026-08-20 christophe@pallier.org>
"""Clip the AI contact sheets into one image per item.

    ./make-stimuli.py --prefix face  --outdir faces_kept  faces/faces_set*.jpeg
    ./make-stimuli.py --prefix house --outdir houses_kept houses/houses_set*.jpeg

Each tile is cropped out of its sheet and written as it is.  The pixels are
untouched: no aperture, no background removal, no rescaling.  The only thing
measured is where the tiles are, and the grid is detected rather than declared
because the sheets are not perfectly regular.

Every item of a run ends up the same size, so the stimuli are interchangeable
in the stream. Each tile is first cropped to its own largest square -- the
sheets differ in tile size, and cropping them all to the smallest would shave
the edges off the items on the larger sheets -- and that square is then
resampled to the common size. The default target is the smallest tile's
square, so nothing is ever enlarged; pass --size to fix it explicitly, which
is what makes two categories match each other.

Repeated items are dropped -- the generator puts the same face or house on
several sheets even when asked not to.  Pass --keep-repeats to keep them all.

Requires numpy, Pillow and scipy.
"""

import argparse
import glob
import sys
from pathlib import Path

import numpy as np
from PIL import Image
from scipy.cluster.hierarchy import fcluster, linkage
from scipy.spatial.distance import squareform

MIN_GAP = 4              # px; ignore flat runs shorter than this
MIN_BAND = 0.04          # a band narrower than this fraction of an axis is noise
DUPLICATE_RES = 32       # thumbnail side for the repeat check


# --------------------------------------------------------------- grid finding

def flat_runs(profile, thresh):
    """Start/end of the runs where `profile` stays below `thresh`."""
    out, start = [], None
    for i, low in enumerate(profile < thresh):
        if low and start is None:
            start = i
        elif not low and start is not None:
            if i - start >= MIN_GAP:
                out.append((start, i))
            start = None
    if start is not None and len(profile) - start >= MIN_GAP:
        out.append((start, len(profile)))
    return out


def split_at(profile, thresh):
    """Content bands between the flat runs, slivers discarded.

    The relative cut-off drops the thin band a tile's drop-shadow leaves under
    each row, which an absolute one lets through as an extra row of the grid.
    """
    out, prev = [], 0
    for a, b in flat_runs(profile, thresh):
        if a > prev:
            out.append((prev, a))
        prev = b
    if prev < len(profile):
        out.append((prev, len(profile)))
    out = [b for b in out if b[1] - b[0] >= MIN_BAND * len(profile)]
    if not out:
        return out
    typical = np.median([b - a for a, b in out])
    return [b for b in out if b[1] - b[0] >= 0.5 * typical]


def bands(profile, n_expected=None):
    """Cut a profile into its content bands.

    The number of bands is not assumed: a sheet may be 8x4 or anything else.
    Sweep the threshold and take the count that survives the widest range of
    them, since a real grid is stable over many thresholds and a spurious
    split is not.
    """
    floor, span = profile.min(), np.median(profile) - profile.min()
    if span <= 0:
        raise SystemExit("blank profile: no grid here")
    runs = {}
    for frac in np.linspace(0.01, 0.45, 220):
        found = split_at(profile, floor + frac * span)
        if len(found) < (n_expected or 2):
            continue
        w = np.array([b - a for a, b in found], dtype=float)
        cv = w.std() / w.mean()
        if n_expected is None and cv > 0.5:
            continue
        if n_expected is not None and len(found) != n_expected:
            continue
        runs.setdefault(len(found), []).append((cv, found))
    if not runs:
        raise SystemExit("no grid found")
    n = max(runs, key=lambda k: (len(runs[k]), -min(c for c, _ in runs[k])))
    return min(runs[n])[1]


def find_tiles(gray, n_cols=None, n_rows=None):
    """Bounding boxes of every tile, keyed by (row, col).

    Columns are found on the whole image, rows inside each column: the tiles
    drift by a few pixels between columns, so a tile's row is its rank within
    its own column, never its raw y coordinate.
    """
    grid = {}
    for c, (x0, x1) in enumerate(bands(gray.std(axis=0), n_cols)):
        inset = int(0.15 * (x1 - x0))
        rows = bands(gray[:, x0 + inset:x1 - inset].std(axis=1), n_rows)
        if n_rows is None:
            n_rows = len(rows)
        for r, (y0, y1) in enumerate(rows):
            grid[(r, c)] = (x0, y0, x1, y1)
    return grid


# ------------------------------------------------------------------- repeats

def signatures(images):
    v = []
    for im in images:
        t = np.asarray(im.convert("L").resize((DUPLICATE_RES, DUPLICATE_RES),
                                 Image.Resampling.BOX), dtype=float).ravel()
        t -= t.mean()
        n = np.linalg.norm(t)
        v.append(t / n if n else t)
    return np.array(v)


def distinct(images, r):
    """Indices of one representative per distinct item.

    Cluster by complete linkage, then walk the representatives and keep one
    only if it matches nothing already kept.  Neither step suffices alone:
    the transitive closure merges two items that both resemble a third, while
    complete linkage splits a genuine repeat whose copies differ slightly.
    """
    v = signatures(images)
    if len(v) < 2:
        return list(range(len(v)))
    d = squareform(np.clip(1.0 - v @ v.T, 0, None), checks=False)
    labels = fcluster(linkage(d, method="complete"), 1.0 - r,
                      criterion="distance")
    groups = {}
    for i, lab in enumerate(labels):
        groups.setdefault(lab, []).append(i)

    reps = []
    for g in groups.values():
        sub = v[g]
        reps.append(g[int(np.argmax((sub @ sub.T).sum(axis=1)))])
    keep = []
    for i in sorted(reps):
        if all(v[i] @ v[j] <= r for j in keep):
            keep.append(i)
    return keep


# ---------------------------------------------------------------------- main

def main():
    p = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("sheets", nargs="+")
    p.add_argument("--prefix", required=True, help="output name stem")
    p.add_argument("--outdir", required=True)
    p.add_argument("--size", type=int,
                   help="common output size in pixels (default: the smallest "
                        "tile's square, so nothing is enlarged)")
    p.add_argument("--grid", help="force COLSxROWS instead of detecting it")
    p.add_argument("--duplicate-r", type=float, default=0.975,
                   help="correlation at which two items are the same item")
    p.add_argument("--keep-repeats", action="store_true")
    args = p.parse_args()

    n_cols = n_rows = None
    if args.grid:
        n_cols, n_rows = (int(v) for v in args.grid.lower().split("x"))

    paths = [Path(q) for s in args.sheets for q in sorted(glob.glob(s))]
    if not paths:
        sys.exit("no sheet matched")

    # Locate every tile first, so the common crop is known before cropping.
    sheets = []
    for path in paths:
        img = Image.open(path)
        grid = find_tiles(np.asarray(img.convert("L"), dtype=float),
                          n_cols, n_rows)
        print(f"{path.name}: {max(c for _, c in grid) + 1}x"
              f"{max(r for r, _ in grid) + 1} grid, {len(grid)} items")
        sheets.append((img, grid))

    # Each tile keeps all of itself: crop to its own square, then scale.
    squares = [min(x1 - x0, y1 - y0)
               for _, g in sheets for x0, y0, x1, y1 in g.values()]
    target = args.size or min(squares)
    print(f"tiles are {min(squares)}-{max(squares)} px; "
          f"cropping each to its own square, then resampling to {target}x{target}")

    items = []
    for img, grid in sheets:
        for r, c in sorted(grid):
            x0, y0, x1, y1 = grid[(r, c)]
            side = min(x1 - x0, y1 - y0)
            cx, cy = (x0 + x1) // 2, (y0 + y1) // 2
            tile = img.crop((cx - side // 2, cy - side // 2,
                             cx - side // 2 + side, cy - side // 2 + side))
            if side != target:
                tile = tile.resize((target, target), Image.Resampling.LANCZOS)
            items.append(tile)

    chosen = list(range(len(items)))
    if not args.keep_repeats:
        chosen = distinct(items, args.duplicate_r)
        print(f"{len(chosen)} distinct items, "
              f"{len(items) - len(chosen)} repeats dropped")

    out = Path(args.outdir)
    out.mkdir(parents=True, exist_ok=True)
    pad = len(str(len(chosen)))
    for k, i in enumerate(chosen, start=1):
        items[i].save(out / f"{args.prefix}_{k:0{pad}d}.png")
    print(f"wrote {len(chosen)} files to {out}/")


if __name__ == "__main__":
    main()
