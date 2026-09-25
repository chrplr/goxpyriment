#!/usr/bin/env python3
"""Make the presented pictures, ../f1.jpg ... ../f5.jpg, from the full-size cuts here.

The cuts are black outlines on white. The program runs on a mid-grey
background (so that the photodiode square, drawn in white, stands out), so the
white *around* each hand is repainted to that grey, while the white *inside*
the outline stays white: the outside is the light region connected to the
image border, found by flood fill. The hands' outlines are closed, which is
what keeps the fill out; the script stops if the fill takes the whole picture.

The recolouring is done at full size and the result is then scaled down, so
the outline's edge is antialiased against the grey rather than against white.

Usage: python3 make_stimuli.py [--size 187x244] [--grey 128]
"""
import argparse
import sys
from pathlib import Path

import numpy as np
from PIL import Image
from scipy import ndimage

HERE = Path(__file__).resolve().parent
OUT = HERE.parent


def grey_background(img, grey, threshold):
    """Return img with the light region connected to its border set to grey."""
    rgb = np.asarray(img.convert("RGB")).copy()
    light = np.asarray(img.convert("L")) >= threshold
    labels, _ = ndimage.label(light)
    edge = np.concatenate([labels[0], labels[-1], labels[:, 0], labels[:, -1]])
    outside = np.isin(labels, np.unique(edge[edge != 0]))
    inside = light & ~outside
    if inside.mean() < 0.05:
        raise ValueError("the fill reached the inside of the hand: is the outline open?")
    rgb[outside] = grey
    return Image.fromarray(rgb), outside.mean(), inside.mean()


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--size", default="187x244", help="output size WxH (default 187x244)")
    ap.add_argument("--grey", type=int, default=128,
                    help="background grey level; must match control.Gray in main.go (default 128)")
    ap.add_argument("--threshold", type=int, default=128,
                    help="grey level at or above which a pixel counts as background (default 128)")
    args = ap.parse_args()
    w, h = (int(v) for v in args.size.lower().split("x"))

    for n in range(1, 6):
        src = HERE / f"f{n}.jpg"
        try:
            img, out_frac, in_frac = grey_background(Image.open(src), args.grey, args.threshold)
        except ValueError as e:
            sys.exit(f"{src.name}: {e}")
        dst = OUT / src.name
        img.resize((w, h), Image.LANCZOS).save(dst, quality=95)
        print(f"{dst.relative_to(HERE.parent.parent)}: {w}x{h}, "
              f"background {100 * out_frac:.1f}% -> grey {args.grey}, inside {100 * in_frac:.1f}% kept")


if __name__ == "__main__":
    main()
