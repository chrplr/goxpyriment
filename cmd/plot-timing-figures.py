#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

"""Regenerate the paper's timing figures from the raw AD3 captures.

    ./cmd/plot-timing-figures.py <capture-dir> <output-dir>

Emits three PDFs:

  timing-durations.pdf   distribution of measured stimulus duration, one small
                         multiple per machine/stack, plotted as deviation from
                         each run's own median so shapes compare directly.
  timing-onsets.pdf      trigger-to-light delay per trial, same normalisation,
                         which is where a compositor separates from bare KMS.
  timing-audio.pdf       tone-onset scatter against audio buffer period.

# Why deviation from the median rather than absolute milliseconds

Because the absolute value is a property of the panel, not of the software: the
same monitor driven by two machines reported medians 5.9 ms apart purely from
liquid-crystal rise/fall asymmetry (see cmd/ad3-to-events.py, which prints that
bias). Overlaying absolute durations would put six panels on six different
x-axes and invite exactly the cross-machine comparison the data does not
support. The spread and the tail are the comparable quantities, so those are
what is drawn.

# Why a log count axis

The finding in these distributions is a handful of trials, not the bulk. A run
whose middle 90 % spans 0.08 ms can still hold 34 stimuli that ran a frame long;
on a linear axis those are invisible against a 900-trial mode.
"""
import os
import sys

import numpy as np
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

# label -> capture file. Order is the order of the panels.
DURATION_RUNS = [
    ("W5700, X11",              "amd-ttl-1823.npz"),
    ("W5700, KMS/DRM",          "amd-kmsdrm-1856.npz"),
    ("5490, Wayland",           "5490-dell-wayland-1208.npz"),
    ("5490, KMS/DRM",           "5490-dell-kmsdrm-1250.npz"),
    ("5490, Wayland, built-in", "5490-wayland-native-0942.npz"),
    ("5490, KMS/DRM, built-in", "5490-kmsdrm-0956.npz"),
]
ONSET_RUNS = DURATION_RUNS[:4]

# Audio: buffer frames at 48 kHz -> (measured SD ms, torn tones, tones measured)
AUDIO = [(256, 2.36, 100, 483), (512, 3.34, 10, 500),
         (1024, 6.17, 0, 481), (2048, 12.27, 0, 60)]

PLOT = {"font.size": 8, "figure.facecolor": "white", "axes.facecolor": "white",
        "axes.grid": True, "grid.alpha": 0.25, "savefig.bbox": "tight"}


def channels(path):
    z = np.load(path)
    rate = float(z["rate"])
    def v(ch):
        return (z[f"offset_v_ch{ch}"]
                + z[f"range_v_ch{ch}"] * z[f"samples_ch{ch}"].astype(float) / 65536)
    return v(1), v(2), rate


def triggers(ttl):
    lo, hi = np.percentile(ttl, (1, 99))
    above = ttl >= lo + 0.5 * (hi - lo)
    return np.flatnonzero(above[1:] & ~above[:-1])


def durations_and_onsets(path):
    """Per-trial (duration, trigger-to-light), both in ms, 50 % / 10 % criteria."""
    pd_, ttl, rate = channels(path)
    lo, hi = np.percentile(pd_, (1, 99))
    W, SETTLE = int(0.35 * rate), int(0.05 * rate)
    dur, lag = [], []
    for e in triggers(ttl):
        if e + W >= len(pd_) or e < int(0.02 * rate):
            continue
        w = pd_[e:e + W]
        base, peak = np.percentile(w, 2), np.percentile(w, 98)
        if peak - base < 0.5 * (hi - lo):
            continue
        n = (w - base) / (peak - base)
        up = np.flatnonzero(n >= 0.5)
        if not up.size:
            continue
        r = up[0]
        down = np.flatnonzero(n[r + SETTLE:] <= 0.5)
        if not down.size:
            continue
        dur.append((r + SETTLE + down[0] - r) / rate * 1000)
        first = np.flatnonzero(n >= 0.10)
        lag.append(first[0] / rate * 1000)
    return np.asarray(dur), np.asarray(lag)


def small_multiples(runs, series, capdir, out, xlabel, half, title):
    plt.rcParams.update(PLOT)
    fig, axes = plt.subplots(len(runs), 1, figsize=(6.4, 0.78 * len(runs)),
                             sharex=True, squeeze=False)
    for ax, (label, fname) in zip(axes[:, 0], runs):
        d = series[fname]
        med = np.median(d)
        rel = d - med
        inside = rel[np.abs(rel) <= half]
        n_out = len(rel) - len(inside)
        ax.hist(inside, bins=90, range=(-half, half), color="0.3")
        ax.set_yscale("log")
        ax.set_ylim(0.7, None)
        ax.axvline(0, color="#c1440e", lw=0.9)
        if n_out:
            worst = rel[np.argmax(np.abs(rel))]
            note = f"  +{n_out} beyond axis, to {worst:+.1f} ms"
        else:
            note = ""
        ax.set_title(f"{label} — n={len(d)}, median {med:.2f} ms, "
                     f"p5–p95 {np.percentile(d, 95) - np.percentile(d, 5):.2f} ms{note}",
                     fontsize=7.5, loc="left")
        ax.set_ylabel("trials", fontsize=7)
        # One label per decade at this panel height; the default log locator
        # stacks minor labels on top of each other in 0.8 inches.
        ax.yaxis.set_major_locator(matplotlib.ticker.LogLocator(numticks=3))
        ax.yaxis.set_minor_locator(matplotlib.ticker.NullLocator())
        ax.tick_params(labelsize=6.5)
        for sp in ("top", "right"):
            ax.spines[sp].set_visible(False)
        print(f"  {label:26s} n={len(d):5d} median {med:8.3f}  "
              f"p5-p95 {np.percentile(d, 95) - np.percentile(d, 5):6.3f}  beyond {n_out}")
    axes[-1, 0].set_xlabel(xlabel, fontsize=8)
    fig.suptitle(title, fontsize=8.5, y=1.005, x=0.02, ha="left")
    fig.tight_layout()
    fig.savefig(out)
    print(f"wrote {out}")


def audio_figure(out):
    plt.rcParams.update(PLOT)
    fig, ax = plt.subplots(figsize=(6.4, 2.3))
    period = np.array([f / 48000 * 1000 for f, _, _, _ in AUDIO])
    sd = np.array([s for _, s, _, _ in AUDIO])
    grid = np.linspace(0, period.max() * 1.08, 100)
    ax.plot(grid, grid / np.sqrt(12), color="#c1440e", lw=1.1,
            label=r"uniform over one buffer:  period/$\sqrt{12}$")
    for (frames, s, torn, total), p in zip(AUDIO, period):
        clean = torn == 0
        ax.plot(p, s, "o", ms=7, mfc="0.3" if clean else "white",
                mec="0.3", mew=1.2, zorder=3)
        ax.annotate(f"{frames}" + ("" if clean else f"\n{100*torn/total:.0f}% torn"),
                    (p, s), textcoords="offset points", xytext=(7, -3),
                    fontsize=7, va="top")
    ax.set_xlabel("audio buffer period (ms) at 48 kHz", fontsize=8)
    ax.set_ylabel("measured tone-onset SD (ms)", fontsize=8)
    ax.set_title("Open markers: buffer sizes at which tones tore",
                 fontsize=8, loc="left")
    ax.legend(fontsize=7.5, frameon=False, loc="upper left")
    for sp in ("top", "right"):
        ax.spines[sp].set_visible(False)
    fig.tight_layout()
    fig.savefig(out)
    print(f"wrote {out}")


def main():
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    capdir, outdir = sys.argv[1], sys.argv[2]
    os.makedirs(outdir, exist_ok=True)
    dur, lag = {}, {}
    for _, fname in DURATION_RUNS:
        path = os.path.join(capdir, fname)
        if not os.path.exists(path):
            sys.exit(f"missing capture: {path}")
        dur[fname], lag[fname] = durations_and_onsets(path)

    print("durations:")
    small_multiples(DURATION_RUNS, dur, capdir,
                    os.path.join(outdir, "timing-durations.pdf"),
                    "measured duration − run median (ms), 50 % criterion", 0.5,
                    "Stimulus duration, 12 frames requested")
    print("onsets:")
    small_multiples(ONSET_RUNS, lag, capdir,
                    os.path.join(outdir, "timing-onsets.pdf"),
                    "trigger-to-light delay − run median (ms), 10 % criterion", 1.5,
                    "Trigger-to-light delay")
    audio_figure(os.path.join(outdir, "timing-audio.pdf"))


if __name__ == "__main__":
    main()
