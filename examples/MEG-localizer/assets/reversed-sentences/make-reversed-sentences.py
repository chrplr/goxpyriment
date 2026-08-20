#! /usr/bin/env python
# Time-stamp: <2026-08-20 christophe@pallier.org>
"""Reverse the spoken sentences to make the low-level auditory control.

    ./make-reversed-sentences.py

Reads ../spoken-sentences-trimmed/sentence_NN.wav and writes control_NN.wav
here. The *trimmed* sentences are the source on purpose: the synthesiser pads
each recording with about a quarter-second of silence, unevenly at the two
ends, and reversing that padding moves the acoustic onset. One file
(sentence_04) had 237 ms of leading silence and none trailing, so its control
would have started 237 ms before the sentence it controls for.

Time reversal is the control this condition wants because of what it leaves
alone: the duration is identical sample for sample, and the long-term
magnitude spectrum is unchanged -- reversing a signal conjugates its Fourier
transform, which leaves |X(f)| exactly as it was. What it destroys is the
temporal fine structure that carries phonetic and lexical information, so the
contrast isolates speech from sound rather than sound from silence.

A short raised-cosine ramp is applied to each end. Reversal turns the
recording's final sample into its first, and a file that was cut mid-decay
would otherwise begin with a step -- an audible click, and a broadband
transient that the original does not have.

Requires numpy and soundfile.
"""

from pathlib import Path

import numpy as np
import soundfile as sf

HERE = Path(__file__).resolve().parent
SOURCE = HERE.parent / "spoken-sentences-trimmed"
RAMP_MS = 10


def ramp(x, rate, ms):
    """Raised-cosine fade in and out."""
    n = min(int(rate * ms / 1000), len(x) // 2)
    if n < 1:
        return x
    w = 0.5 - 0.5 * np.cos(np.pi * np.arange(n) / n)
    y = x.astype(np.float64).copy()
    y[:n] *= w
    y[-n:] *= w[::-1]
    return y


def main():
    sources = sorted(SOURCE.glob("sentence_*.wav"))
    if not sources:
        raise SystemExit(f"no sentence_*.wav in {SOURCE}")

    print(f"{'file':<16}{'ms':>7}{'rms dBFS':>10}  source")
    for path in sources:
        x, rate = sf.read(path, dtype="float64", always_2d=False)
        if x.ndim > 1:                      # collapse an unexpected stereo file
            x = x.mean(axis=1)
        y = ramp(x[::-1], rate, RAMP_MS)

        out = HERE / path.name.replace("sentence_", "control_")
        sf.write(out, y.astype(np.float32), rate, subtype="PCM_16")
        rms = 20 * np.log10(max(np.sqrt(np.mean(y * y)), 1e-12))
        print(f"{out.name:<16}{len(y) / rate * 1000:>7.0f}{rms:>10.1f}  {path.name}")

    print(f"\n{len(sources)} controls written to {HERE.name}/")


if __name__ == "__main__":
    main()
