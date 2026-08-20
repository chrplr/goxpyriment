#! /usr/bin/env python
# Time-stamp: <2026-08-20 christophe@pallier.org>
"""Split contact sheets of AI-generated stimuli into one PNG per item.

Generalises the earlier faces/split-faces.py to every sheet in assets/: the
grid is detected rather than declared, several sheets are numbered as one
series, and the face-specific oval aperture became an option.

    ./split-sheets.py --prefix face --oval  faces/faces_set*.jpeg
    ./split-sheets.py --prefix house        houses/houses_set*.jpeg
    ./split-sheets.py --prefix house --start 33 --grid 6x6  houses/houses.jpeg

Each PNG is written next to its sheet unless --outdir says otherwise.  All the
items of a run share one square size, so the stimuli are interchangeable in the
stream, and --check-duplicates reports items that repeat -- the sheets do
contain repeats, both within houses.jpeg and across the face sets.

Requires numpy, Pillow and scipy.
"""

import argparse
import glob
import sys
from pathlib import Path

import numpy as np
from PIL import Image
from scipy import ndimage
from scipy.cluster.hierarchy import fcluster, linkage
from scipy.spatial.distance import squareform

MIN_GAP = 4              # px; ignore flat runs shorter than this
MIN_BAND = 0.04          # a band narrower than this fraction of the axis is noise
OVAL_W, OVAL_H = 0.74, 0.94
OVAL_SHIFT_Y = -0.03
FEATHER = 0.07
PROTECT_W, PROTECT_H = 0.50, 0.74
EDGE_BLUR = 1.2
BG_TOLERANCE = 15
LARGEST_ONLY = True      # keep only the item's own connected blob
# Correlation above which two items are the same stimulus.  The signature is
# the *finished* item -- background removed, aperture applied -- because that
# isolates the thing itself: on the raw tile, a change of shirt or of hair
# against the wall drags two pictures of one person below any usable threshold.
# The shared aperture inflates all correlations, so the cut-off is calibrated
# against that: judged by eye on the 160 face items, pairs from 0.975 to 0.986
# are different people, and genuine repeats start at 0.988.  Recalibrate (or
# pass --duplicate-r) if you change the aperture or run without one.
DUPLICATE_R = 0.987
DISC_BG = 128            # grey inside the aperture, as in minye_zhan_stimuli/
FIT_MARGIN = 0.98        # item fits inside this fraction of the disc radius
DUPLICATE_RES = 32       # thumbnail side used for the comparison


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
    """Content bands left between the flat runs, slivers discarded.

    Two filters, and both are needed.  The absolute one drops noise; the
    relative one drops the thin band a tile's drop-shadow produces just below
    each row, which is wide enough to survive an absolute cut-off and would
    otherwise be counted as an extra row of the grid.
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


def sweep(profile):
    """Every (n_bands, bands) the profile yields as the threshold rises."""
    floor, span = profile.min(), np.median(profile) - profile.min()
    if span <= 0:
        return []
    for frac in np.linspace(0.01, 0.45, 220):
        yield split_at(profile, floor + frac * span)


def bands(profile, n_expected=None):
    """Cut a profile into its content bands.

    With `n_expected` given, take the first threshold that isolates exactly
    that many.  Without it, the count is unknown -- a sheet may be 8x4 or 6x6 --
    so choose the count that survives the widest range of thresholds, breaking
    ties towards the most regular band widths.  A real grid is stable over many
    thresholds and near-uniform in pitch; a spurious split is neither.
    """
    if n_expected is not None:
        best = None
        for found in sweep(profile):
            if len(found) != n_expected:
                continue
            w = np.array([b - a for a, b in found], dtype=float)
            cv = w.std() / w.mean()
            if best is None or cv < best[0]:
                best = (cv, found)
        if best is None:
            raise SystemExit(f"could not isolate {n_expected} bands")
        return best[1]

    runs = {}
    for found in sweep(profile):
        n = len(found)
        if n < 2:
            continue
        w = np.array([b - a for a, b in found], dtype=float)
        cv = w.std() / w.mean()
        if cv > 0.5:             # loose: only to reject degenerate splits
            continue
        runs.setdefault(n, []).append((cv, found))
    if not runs:
        raise SystemExit("no grid found in this profile")
    n = max(runs, key=lambda k: (len(runs[k]), -min(c for c, _ in runs[k])))
    return min(runs[n])[1]


def find_tiles(gray, n_cols=None, n_rows=None):
    """Bounding boxes of every tile, keyed by (row, col).

    Columns come from the whole image; rows are then found inside each column,
    because the tiles drift by a few pixels from one column to the next.  A
    tile's row is its rank within its column, never its raw y coordinate.
    """
    cols = bands(gray.std(axis=0), n_cols)
    grid = {}
    for c, (x0, x1) in enumerate(cols):
        inset = int(0.15 * (x1 - x0))
        strip = gray[:, x0 + inset:x1 - inset]
        rows = bands(strip.std(axis=1), n_rows)
        if n_rows is None and c == 0:
            n_rows = len(rows)          # hold the first column's count
        elif len(rows) != n_rows:
            raise SystemExit(
                f"column {c + 1} has {len(rows)} rows, expected {n_rows}")
        for r, (y0, y1) in enumerate(rows):
            grid[(r, c)] = (x0, y0, x1, y1)
    return grid


# ------------------------------------------------------------------ apertures

def ellipse_distance(side, w, h, shift=OVAL_SHIFT_Y):
    """Radial distance in ellipse units: 1.0 exactly on the ellipse."""
    t = (np.arange(side) + 0.5) / side - 0.5
    return np.hypot((t / (w / 2))[None, :],
                    ((t - shift) / (h / 2))[:, None])


def oval_alpha(side, w, h, shift):
    """Feathered ellipse: 1 inside, 0 outside, cosine roll-off at the edge."""
    a = (1.0 - ellipse_distance(side, w, h, shift)) / FEATHER
    return np.clip(0.5 - 0.5 * np.cos(np.pi * np.clip(a, 0, 1)), 0, 1)


def fit_in_disc(arr, alpha, side, margin, disc_bg):
    """Scale the isolated item so all of it fits inside the disc, then compose.

    The earlier version sized an ellipse to fit inside the tile and masked with
    it, which necessarily cut anything that filled its tile -- fine for a head
    with room around it, wrong for a house photographed edge to edge.  Here the
    item is measured, scaled down until its bounding box fits *within* the
    disc, and laid on a grey disc against black: the convention the existing
    minye_zhan_stimuli/ set uses, and nothing is ever clipped.
    """
    ys, xs = np.where(alpha > 0.5)
    if not len(xs):
        return np.zeros((side, side))
    x0, x1, y0, y1 = xs.min(), xs.max() + 1, ys.min(), ys.max() + 1
    w, h = x1 - x0, y1 - y0

    # The whole bounding box must sit inside the circle, so the box diagonal
    # is what has to match the diameter.
    scale = margin * side / np.hypot(w, h)
    nw, nh = max(1, round(w * scale)), max(1, round(h * scale))
    item = np.asarray(Image.fromarray(arr[y0:y1, x0:x1].astype(np.uint8), "L")
                      .resize((nw, nh), Image.Resampling.LANCZOS), dtype=float)
    op = np.asarray(Image.fromarray((alpha[y0:y1, x0:x1] * 255).astype(np.uint8), "L")
                    .resize((nw, nh), Image.Resampling.LANCZOS), dtype=float) / 255.0

    canvas = np.zeros((side, side))
    opacity = np.zeros((side, side))
    ox, oy = (side - nw) // 2, (side - nh) // 2
    canvas[oy:oy + nh, ox:ox + nw] = item
    opacity[oy:oy + nh, ox:ox + nw] = op

    d = ellipse_distance(side, 1.0, 1.0, 0.0)
    disc = np.clip(0.5 - 0.5 * np.cos(np.pi * np.clip((1.0 - d) / FEATHER, 0, 1)), 0, 1)
    return canvas * opacity + disc_bg * disc * (1.0 - opacity)


def background_alpha(gray, protected, tol=BG_TOLERANCE):
    """Opacity map that blacks out the sheet's background behind the item.

    The background is whatever luminance the tile border sits at, within
    BG_TOLERANCE, and connected to that border -- not simply "the bright
    pixels".  The two-sided test matters: the face sheets are on a white wall,
    houses.jpeg is on mid-grey with *lighter* grid lines, and a one-sided
    brightness rule gets the second case backwards.  `protected` shields the
    centre, for the light-haired face whose hair matches the wall behind it.
    """
    border = np.concatenate([gray[0], gray[-1], gray[:, 0], gray[:, -1]])
    # The *mode* of the border, not its median.  Hair and shoulders reach the
    # tile edge, so a median is dragged well off the wall -- measured at 172 on
    # a tile whose wall is 216, a 44-level error that made the wall fail its
    # own tolerance test.  The wall is what most border pixels agree on.
    hist, edges = np.histogram(border, bins=np.arange(0, 260, 4))
    level = edges[int(np.argmax(hist))] + 2.0
    labels, _ = ndimage.label(np.abs(gray - level) <= tol)
    touching = set(np.concatenate(
        [labels[0], labels[-1], labels[:, 0], labels[:, -1]]).tolist()) - {0}
    back = np.isin(labels, list(touching)) & ~protected
    item = ~back
    if LARGEST_ONLY:
        # Wall trapped between the head and the aperture edge is not connected
        # to the tile border, so it survives as an island of background inside
        # the item.  The item itself is one blob; keep that and drop the rest.
        lab2, n2 = ndimage.label(item)
        if n2 > 1:
            sizes = ndimage.sum(item, lab2, range(1, n2 + 1))
            item = lab2 == (int(np.argmax(sizes)) + 1)
    return ndimage.gaussian_filter(item.astype(float), EDGE_BLUR)


# ----------------------------------------------------------------------- main

def signatures(images):
    """Unit-norm thumbnails: the comparison space for "is this the same item"."""
    v = []
    for im in images:
        t = np.asarray(im.convert("L").resize((DUPLICATE_RES, DUPLICATE_RES),
                                              Image.Resampling.BOX),
                       dtype=float).ravel()
        t -= t.mean()
        n = np.linalg.norm(t)
        v.append(t / n if n else t)
    return np.array(v)


def cluster_items(images, r):
    """Group items that are the same stimulus, by complete linkage.

    Complete linkage, not the transitive closure: a chain A~B~C would merge A
    and C on the strength of B alone, and at these correlations merely similar
    faces do chain.  Requiring every pair inside a cluster to match keeps two
    people who both resemble a third from collapsing into one item.
    """
    v = signatures(images)
    if len(v) < 2:
        return [[0]] if len(v) else []
    d = squareform(np.clip(1.0 - v @ v.T, 0, None), checks=False)
    labels = fcluster(linkage(d, method="complete"), 1.0 - r, criterion="distance")
    out = {}
    for i, lab in enumerate(labels):
        out.setdefault(lab, []).append(i)
    return [sorted(g) for g in out.values()]


def medoid(images, group):
    """The member of `group` most typical of it -- the one kept."""
    if len(group) == 1:
        return group[0]
    v = signatures([images[i] for i in group])
    return group[int(np.argmax((v @ v.T).sum(axis=1)))]


def enforce_distinct(images, chosen, r):
    """Drop items until no two kept items are the same, and prove it.

    Clustering alone does not give the guarantee.  Complete linkage splits a
    genuine repeat whenever the copies differ slightly more than the threshold
    in one direction; the transitive closure instead merges people who merely
    resemble each other.  So after clustering, walk the representatives and
    keep one only if it matches nothing already kept.  That errs towards
    dropping a distinct-but-similar item -- the safe direction when a repeat
    in the final set would show up as a repetition effect.
    """
    v = signatures([images[i] for i in chosen])
    keep, dropped = [], []
    for k in range(len(chosen)):
        if all(v[k] @ v[j] <= r for j in keep):
            keep.append(k)
        else:
            dropped.append(chosen[k])
    return [chosen[k] for k in keep], dropped


def near_duplicates(images, r=DUPLICATE_R):
    """Pairs of items whose 16x16 thumbnails correlate above `r`."""
    v = []
    for im in images:
        t = np.asarray(im.convert("L").resize((DUPLICATE_RES, DUPLICATE_RES),
                                              Image.Resampling.BOX),
                       dtype=float).ravel()
        t -= t.mean()
        n = np.linalg.norm(t)
        v.append(t / n if n else t)
    m = np.array(v) @ np.array(v).T
    return [(i, j, m[i, j]) for i in range(len(v)) for j in range(i + 1, len(v))
            if m[i, j] > r]


def main():
    p = argparse.ArgumentParser(description=__doc__,
                                formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("sheets", nargs="+", help="contact sheet image files")
    p.add_argument("--prefix", required=True, help="output name stem, e.g. face")
    p.add_argument("--outdir", help="default: next to each sheet")
    p.add_argument("--grid", help="force COLSxROWS instead of detecting it")
    p.add_argument("--start", type=int, default=1, help="first item number")
    p.add_argument("--pad", type=int, help="digits in the number (default: fit)")
    p.add_argument("--size", type=int, help="resize every item to this square")
    p.add_argument("--oval", action="store_true", help="apply the oval aperture")
    p.add_argument("--oval-size", default=f"{OVAL_W}x{OVAL_H}", metavar="WxH",
                   help="ellipse axes as a fraction of the tile "
                        f"(default {OVAL_W}x{OVAL_H}, portrait, suits faces; "
                        "houses want a landscape ellipse)")
    p.add_argument("--oval-shift", type=float, default=OVAL_SHIFT_Y,
                   help="move the ellipse up (negative) or down")
    p.add_argument("--protect-size", default=f"{PROTECT_W}x{PROTECT_H}",
                   metavar="WxH",
                   help="inner ellipse the background removal may never enter "
                        f"(default {PROTECT_W}x{PROTECT_H}); it must follow the "
                        "aperture's orientation, or it shields the wrong region")
    p.add_argument("--keep-background", action="store_true",
                   help="do not black out the sheet background")
    p.add_argument("--bg-tolerance", type=float, default=BG_TOLERANCE,
                   help="how far from the border level still counts as "
                        f"background (default {BG_TOLERANCE}); raise it when "
                        "the sheet vignettes its tiles")
    p.add_argument("--colour", action="store_true", help="keep RGB, not grey")
    p.add_argument("--fit", action="store_true",
                   help="scale each item to fit entirely inside a disc and lay "
                        "it on grey, as in minye_zhan_stimuli/ -- nothing is "
                        "clipped, unlike a fixed aperture over a full tile")
    p.add_argument("--fit-margin", type=float, default=FIT_MARGIN,
                   help=f"fraction of the disc the item fills (default {FIT_MARGIN})")
    p.add_argument("--disc-bg", type=float, default=DISC_BG,
                   help=f"grey level inside the disc (default {DISC_BG}); "
                        "0 puts the item straight on black")
    p.add_argument("--max-fill", type=float, default=0.90,
                   help="drop an item whose mask covers more than this "
                        "fraction of its tile (background not separated)")
    p.add_argument("--max-bright", type=float, default=0.08,
                   help="drop an item with more than this fraction of bright "
                        "pixels inside the disc (background the segmentation "
                        "missed, typically sky)")
    p.add_argument("--min-fill", type=float, default=0.02,
                   help="drop an item whose mask is smaller than this")
    p.add_argument("--unique", action="store_true",
                   help="write one file per distinct item, dropping repeats "
                        "(the sheets repeat identities even when the generator "
                        "was asked not to)")
    p.add_argument("--duplicate-r", type=float, default=DUPLICATE_R,
                   help=f"correlation at which two items are the same "
                        f"(default {DUPLICATE_R})")
    p.add_argument("--check-duplicates", action="store_true",
                   help="report items that repeat across the run")
    p.add_argument("-n", "--dry-run", action="store_true",
                   help="report the detected grids and write nothing")
    args = p.parse_args()

    n_cols = n_rows = None
    if args.grid:
        n_cols, n_rows = (int(v) for v in args.grid.lower().split("x"))

    paths = [Path(q) for s in args.sheets for q in sorted(glob.glob(s))]
    if not paths:
        sys.exit("no sheet matched")

    # Pass 1: locate every tile, and the square size they can all supply.
    sheets = []
    for path in paths:
        img = Image.open(path)
        grid = find_tiles(np.asarray(img.convert("L"), dtype=float),
                          n_cols, n_rows)
        rows = max(r for r, _ in grid) + 1
        cols = max(c for _, c in grid) + 1
        side = min(min(x1 - x0, y1 - y0) for x0, y0, x1, y1 in grid.values())
        print(f"{path}: {cols}x{rows} grid, {len(grid)} items, "
              f"tiles down to {side}px")
        sheets.append((img, grid))

    per_sheet = [min(min(x1 - x0, y1 - y0) for x0, y0, x1, y1 in g.values())
                 for _, g in sheets]
    side = args.size if args.size else min(per_sheet)
    total = sum(len(g) for _, g in sheets)
    pad = args.pad or len(str(args.start + total - 1))
    print(f"{total} items, common square {side}x{side} px")
    if args.dry_run:
        return

    ow, oh = (float(v) for v in args.oval_size.lower().split("x"))
    oval = oval_alpha(side, ow, oh, args.oval_shift) if args.oval else None
    pw, ph = (float(v) for v in args.protect_size.lower().split("x"))
    # A shield wider than the item protects background, not content: for the
    # houses a face-sized shield left the white wall inside the aperture.
    protected = (np.zeros((side, side), bool) if pw <= 0 or ph <= 0
                 else ellipse_distance(side, pw, ph, args.oval_shift) <= 1.0)

    items, sources, failed = [], [], []
    for path, (img, grid), sheet_side in zip(paths, sheets, per_sheet):
        # Without --size every item is cropped to one square, so no item is
        # ever resampled.  With --size each sheet keeps its own crop -- which
        # would otherwise clip the wider items of the larger sheets -- and the
        # resize brings them to the common target.
        crop = sheet_side if args.size else side
        for r, c in sorted(grid):
            x0, y0, x1, y1 = grid[(r, c)]
            cx, cy = (x0 + x1) // 2, (y0 + y1) // 2
            tile = img.crop((cx - crop // 2, cy - crop // 2,
                             cx - crop // 2 + crop, cy - crop // 2 + crop))
            if args.size:
                tile = tile.resize((side, side), Image.Resampling.LANCZOS)
            if not args.colour:
                tile = tile.convert("L")

            if oval is not None or not args.keep_background:
                arr = np.asarray(tile, dtype=float)
                alpha = np.ones((side, side))
                if not args.keep_background:
                    alpha *= background_alpha(
                        np.asarray(tile.convert("L"), dtype=float), protected,
                        args.bg_tolerance)
                if oval is not None:
                    alpha *= oval
                if args.fit:
                    # A mask covering nearly the whole tile means the
                    # background was not separated at all -- the "item" is then
                    # the entire photograph, and it composites as a square
                    # rather than an object.  An empty mask means nothing was
                    # found.  Both are failures; drop them rather than ship them.
                    fill = float(alpha.mean())
                    if not args.min_fill <= fill <= args.max_fill:
                        failed.append((f"{path.name}[r{r + 1}c{c + 1}]", fill))
                        continue
                    composed = fit_in_disc(arr, alpha, side, args.fit_margin,
                                           args.disc_bg)
                    # Sky the segmentation missed survives as a bright patch
                    # against the grey; a clean item barely has any.
                    d = ellipse_distance(side, 1.0, 1.0, 0.0) < 0.94
                    bright = float((composed[d] > args.disc_bg + 60).mean())
                    if bright > args.max_bright:
                        failed.append((f"{path.name}[r{r + 1}c{c + 1}] "
                                       f"bright {bright:.0%}", bright))
                        continue
                    tile = Image.fromarray(
                        np.round(np.clip(composed, 0, 255)).astype(np.uint8), "L")
                else:
                    if arr.ndim == 3:
                        alpha = alpha[:, :, None]
                    tile = Image.fromarray(
                        np.round(arr * alpha).astype(np.uint8), tile.mode)
            items.append(tile)
            sources.append(f"{path.name}[r{r + 1}c{c + 1}]")

    if failed:
        print(f"{len(failed)} items dropped as unsegmented "
              f"or with leftover background: "
              + ", ".join(n for n, _ in failed[:8])
              + (" ..." if len(failed) > 8 else ""))

    chosen = list(range(len(items)))
    if args.unique:
        clusters = cluster_items(items, args.duplicate_r)
        chosen = [medoid(items, g) for g in clusters]
        chosen, extra = enforce_distinct(items, chosen, args.duplicate_r)
        print(f"{len(clusters)} clusters, {len(extra)} further repeats removed "
              f"by the pairwise check -> {len(chosen)} distinct items, "
              f"{len(items) - len(chosen)} dropped in total")
        for g in sorted((g for g in clusters if len(g) > 1), key=len, reverse=True):
            keep = medoid(items, g)
            print(f"  kept {sources[keep]}, dropped "
                  + ", ".join(sources[i] for i in g if i != keep))

    out = Path(args.outdir) if args.outdir else paths[0].parent
    out.mkdir(parents=True, exist_ok=True)
    pad = args.pad or len(str(args.start + len(chosen) - 1))
    names = []
    for k, i in enumerate(chosen):
        name = out / f"{args.prefix}_{args.start + k:0{pad}d}.png"
        items[i].save(name)
        names.append(name.name)
    print(f"wrote {len(names)} files to {out}/: {names[0]} .. {names[-1]}")

    if args.check_duplicates:
        kept_raws = [items[i] for i in chosen]
        dups = near_duplicates(kept_raws, args.duplicate_r)
        if not dups:
            print("no near-duplicates found")
            return
        # Group the pairs transitively: A==B and B==C is one repeated item.
        parent = list(range(len(kept_raws)))
        def root(i):
            while parent[i] != i:
                parent[i] = parent[parent[i]]
                i = parent[i]
            return i
        for i, j, _ in dups:
            parent[root(i)] = root(j)
        groups = {}
        for i in range(len(kept_raws)):
            groups.setdefault(root(i), []).append(i)
        repeated = [g for g in groups.values() if len(g) > 1]
        print(f"{len(dups)} duplicate pairs, {len(repeated)} repeated items, "
              f"{len(groups)} distinct items out of {len(kept_raws)}")
        for g in sorted(repeated, key=len, reverse=True):
            print("  same item: " + ", ".join(names[i] for i in g))


if __name__ == "__main__":
    main()
