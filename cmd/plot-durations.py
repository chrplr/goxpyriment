#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

"""Histograms of optically measured stimulus duration, one panel per capture.

    ./cmd/plot-durations.py out.png "label=capture.npz" ["label=capture2.npz" ...]

Each capture is a two-channel AD3 recording with the photodiode on CH1 (see
cmd/ad3-to-events.py for the rest of that toolchain). Durations are 50%-to-50%
crossings of each trial's own swing.

# Why a histogram rather than mean and SD

Because these distributions are not Gaussian and the tail is the finding. An X11
run measured here had an SD of 11.6 ms built almost entirely from three trials,
one of them 147 ms long — a whole extra bright phase — while the other 476
repeated to 12 us. Quoting 11.6 ms implies jitter that is not there; quoting the
12 us hides a failure that is. The histogram shows both at once, which is why it
belongs in the paper and the summary statistics belong beside it rather than
instead of it.

Prints SD, range and p5/p95 for each panel: SD because reviewers expect it, the
others because they are what the shape actually supports.
"""
import sys

import numpy as np
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt


def durations(npz):
    """50%-to-50% duration of every complete bright phase in a capture.

    A capture deliberately outlives the stimulus, so its last trials are cut off
    by the run ending rather than by the stimulus offset. Those are not
    measurements and must not be counted: on a 479-trial X11 capture the final
    three inflated the standard deviation from 0.012 ms to 11.6 ms and read, at
    first glance, as the compositor truncating presentations. A trial is
    therefore kept only if the light rises AGAIN afterwards, which is the
    evidence that the cycle it belongs to completed inside the recording.
    """
    z = np.load(npz)
    rate = float(z["rate"])
    pd_ = z["offset_v_ch1"] + z["range_v_ch1"] * z["samples_ch1"].astype(float) / 65536
    lo, hi = np.percentile(pd_, (1, 99))
    above = pd_ >= lo + 0.5 * (hi - lo)
    rises = np.flatnonzero(above[1:] & ~above[:-1])
    out = []
    W, SKIP = int(0.40 * rate), int(0.05 * rate)
    # rises[:-1]: the last rise has no successor, so its phase cannot be shown
    # to have ended for the right reason.
    for i, nxt in zip(rises[5:-1], rises[6:]):
        if i + W >= len(pd_):
            break
        f = np.flatnonzero(~above[i + SKIP:i + W])
        if not f.size:
            continue
        off = i + SKIP + f[0]
        if off >= nxt:          # never went dark before the next flash
            continue
        out.append((off - i) / rate * 1000)
    return np.asarray(out)


def main():
    if len(sys.argv) < 3:
        sys.exit(__doc__)
    out, specs = sys.argv[1], sys.argv[2:]
    plt.rcParams.update({"font.size": 10, "figure.facecolor": "#f7f7f5",
                         "axes.facecolor": "#fbfbf9"})
    fig, axes = plt.subplots(len(specs), 1, figsize=(9, 2.5 * len(specs)), squeeze=False)
    for ax, spec in zip(axes[:, 0], specs):
        label, path = spec.split("=", 1)
        d = durations(path)
        med = np.median(d)
        # A fixed window around the median, with out-of-range trials counted in
        # the annotation rather than silently clipped or allowed to flatten the
        # bulk into one bin.
        half = 1.0
        inside = d[np.abs(d - med) <= half]
        ax.hist(inside, bins=80, color="0.35")
        ax.axvline(med, color="#c1440e", lw=1.2)
        n_out = len(d) - len(inside)
        ax.set_title(
            f"{label} — n={len(d)}, median {med:.3f} ms, SD {d.std(ddof=1):.3f}, "
            f"p5–p95 {np.percentile(d,5):.3f}–{np.percentile(d,95):.3f}, "
            f"range {d.min():.2f}–{d.max():.2f}"
            + (f"   [{n_out} trial(s) outside ±{half:g} ms, not plotted]" if n_out else ""),
            fontsize=9.5, loc="left")
        ax.set_xlabel("measured duration (ms), 50% criterion")
        ax.set_ylabel("trials")
        ax.grid(alpha=0.2)
        for sp in ("top", "right"):
            ax.spines[sp].set_visible(False)
        print(f"{label:24s} n={len(d):5d} median {med:8.3f}  SD {d.std(ddof=1):7.3f}  "
              f"p5-p95 {np.percentile(d,5):.3f}-{np.percentile(d,95):.3f}  "
              f"range {d.min():.2f}-{d.max():.2f}  outside±{half:g}ms {n_out}")
    fig.tight_layout()
    fig.savefig(out, dpi=130)
    print(f"wrote {out}")


if __name__ == "__main__":
    main()
