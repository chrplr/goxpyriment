#!/usr/bin/env python3
"""Split fingers_all.jpg into one file per hand (f1.jpg … f5.jpg).

The hands are separated by columns containing only white pixels. The script
finds those gaps automatically and cuts the image vertically at the middle of
each gap, so neighbouring hands never overlap in the output files.

Every output file has the same width (--width, default: the widest slice) and
the hand's ink is horizontally centred in it. Pixels are only ever taken from
the hand's own slice; the rest of the canvas is white.

Usage: python3 split_hands.py [fingers_all.jpg] [--width N] [--prefix f] [--threshold 240]
"""
import argparse
import sys

import numpy as np
from PIL import Image


def find_hands(img, threshold):
    """Return [(ink0, ink1, cut0, cut1), …] per hand.

    ink0..ink1 is the column range that contains the hand's pixels;
    cut0..cut1 is the slice allotted to it (cut mid-gap, no overlap).
    """
    gray = np.asarray(img.convert("L"))
    ink = (gray < threshold).any(axis=0)  # True for columns with any non-white pixel

    # Runs of ink columns = hands.
    runs = []
    start = None
    for x, v in enumerate(ink):
        if v and start is None:
            start = x
        elif not v and start is not None:
            runs.append((start, x))
            start = None
    if start is not None:
        runs.append((start, len(ink)))

    # Drop specks narrower than 1% of the width (JPEG noise).
    min_w = img.width // 100
    runs = [r for r in runs if r[1] - r[0] >= min_w]

    # Cut boundaries: image edges and the midpoint of every gap between runs.
    cuts = [0]
    for (_, a1), (b0, _) in zip(runs, runs[1:]):
        cuts.append((a1 + b0) // 2)
    cuts.append(img.width)
    return [(i0, i1, c0, c1) for (i0, i1), (c0, c1) in zip(runs, zip(cuts, cuts[1:]))]


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("image", nargs="?", default="fingers_all.jpg")
    ap.add_argument("--width", type=int, default=0,
                    help="output width in px (default: width of the widest slice)")
    ap.add_argument("--prefix", default="f", help="output name prefix (default: f → f1.jpg …)")
    ap.add_argument("--threshold", type=int, default=240,
                    help="gray level below which a pixel counts as ink (default 240)")
    ap.add_argument("--expect", type=int, default=5, help="expected number of hands (default 5)")
    args = ap.parse_args()

    img = Image.open(args.image).convert("RGB")
    hands = find_hands(img, args.threshold)
    if len(hands) != args.expect:
        sys.exit(f"found {len(hands)} hands, expected {args.expect}: {hands}")

    width = args.width or max(c1 - c0 for _, _, c0, c1 in hands)
    widest_ink = max(i1 - i0 for i0, i1, _, _ in hands)
    if width < widest_ink:
        sys.exit(f"--width {width} is narrower than the widest hand ({widest_ink} px)")

    for n, (i0, i1, c0, c1) in enumerate(hands, 1):
        # Window of `width` columns centred on the hand's ink, clipped to its
        # own slice so nothing from a neighbour is copied.
        centre = (i0 + i1) / 2
        w0 = int(round(centre - width / 2))
        src0, src1 = max(w0, c0), min(w0 + width, c1)
        canvas = Image.new("RGB", (width, img.height), "white")
        canvas.paste(img.crop((src0, 0, src1, img.height)), (src0 - w0, 0))
        out = f"{args.prefix}{n}.jpg"
        canvas.save(out, quality=95)
        print(f"{out}: ink {i0}-{i1} centred, {width}x{img.height}")


if __name__ == "__main__":
    main()
