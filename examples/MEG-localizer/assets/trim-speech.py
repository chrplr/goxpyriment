#! /usr/bin/env python
# Time-stamp: <2026-08-20 christophe@pallier.org>
"""Trim the silence the synthesiser padded onto the spoken stimuli.

    ./trim-speech.py            # write trimmed copies
    ./trim-speech.py --dry-run  # report what would be trimmed

Every TTS recording arrives with roughly a quarter-second of silence before
the voice starts and as much again after it ends. Two reasons that has to go:

  * A trigger fires when the row starts, but the sound would start ~250 ms
    later, by a different amount in every file. That jitter would smear any
    auditory evoked response unless the analysis corrected item by item.
  * The padding costs whole slots. Stripped, the sentences and the spoken
    equations run well under 2 s and fit one block instead of two.

The trimmed copies go to a sibling directory with the suffix `-trimmed`; the
synthesised originals are left alone.

Speech is located from a smoothed envelope at THRESHOLD_DB below the file's
peak, then PAD_MS of silence is kept on each side -- enough that the trim never
clips a quiet onset, small enough to be the same for every file.

Requires numpy and soundfile.
"""

import argparse
from pathlib import Path

import numpy as np
import soundfile as sf

HERE = Path(__file__).resolve().parent
SOURCES = ["spoken-sentences", "equations-audio", "motor-audio"]
THRESHOLD_DB = -40
PAD_MS = 20
SMOOTH_MS = 10
RAMP_MS = 10


def speech_bounds(x, rate):
    """First and last sample of speech, by a smoothed-envelope threshold."""
    w = max(1, int(rate * SMOOTH_MS / 1000))
    env = np.convolve(np.abs(x), np.ones(w) / w, mode="same")
    loud = np.where(env > env.max() * 10 ** (THRESHOLD_DB / 20))[0]
    if not len(loud):
        return 0, len(x)
    return int(loud[0]), int(loud[-1]) + 1


def ramp(x, rate, ms):
    """Raised-cosine fade at each end, so a trim cannot leave a step."""
    n = min(int(rate * ms / 1000), len(x) // 2)
    if n < 1:
        return x
    w = 0.5 - 0.5 * np.cos(np.pi * np.arange(n) / n)
    y = x.copy()
    y[:n] *= w
    y[-n:] *= w[::-1]
    return y


def main():
    p = argparse.ArgumentParser(description=__doc__,
                                formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("-n", "--dry-run", action="store_true")
    args = p.parse_args()

    pad = PAD_MS / 1000
    for name in SOURCES:
        src = HERE / name
        dst = HERE / f"{name}-trimmed"
        files = sorted(f for f in src.glob("*.wav"))
        if not files:
            print(f"{name}: no wav files, skipped")
            continue
        if not args.dry_run:
            dst.mkdir(exist_ok=True)
        print(f"\n{name} -> {dst.name}/")
        print(f"  {'file':<20}{'was':>7}{'now':>7}{'cut lead':>10}{'cut trail':>11}")
        for f in files:
            x, rate = sf.read(f, dtype="float64")
            if x.ndim > 1:
                x = x.mean(axis=1)
            a, b = speech_bounds(x, rate)
            a = max(0, a - int(pad * rate))
            b = min(len(x), b + int(pad * rate))
            y = ramp(x[a:b], rate, RAMP_MS)
            if not args.dry_run:
                sf.write(dst / f.name, y.astype(np.float32), rate, subtype="PCM_16")
            print(f"  {f.name:<20}{len(x)/rate*1000:>7.0f}{len(y)/rate*1000:>7.0f}"
                  f"{a/rate*1000:>10.0f}{(len(x)-b)/rate*1000:>11.0f}")


if __name__ == "__main__":
    main()
