#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

"""Sanity checks an AD3 capture must pass before any number is taken from it.

Imported by the analysis scripts in this directory; not run directly.

# Why this exists

On 2026-08-18 a 545-second capture of a 4K panel was analysed and produced a
stimulus duration of 212.05 ms where the host had presented exactly 12 frames
(200.02 ms), with an SD of 0.018 ms and no outliers. Everything about it looked
like a clean measurement of a real effect. It was not: channel 1 had been left
on the +/-5 V range while the photodiode swings to 8.2 V, so 38 % of all samples
sat pinned at the ADC ceiling. With the plateau clipped flat, the peak used to
place a 50 % criterion is the clip level, so the threshold lands far too low on
the real waveform -- early on the rise, late on the fall -- and inflates every
duration.

Nothing downstream could detect this. The clipped signal is *more* self
consistent than a real one, so precision statistics look better rather than
worse, and the artefact reads as a discovery about the panel. Hence a hard
refusal here rather than a warning: a capture that has lost its peaks cannot be
rescued at analysis time, and the only correct response is to record it again.
"""


class ClippedCapture(Exception):
    """Raised when a channel spent part of the capture pinned at the ADC rail."""


def clipping(z, channels=(1, 2)):
    """Fraction of samples resting on the single most extreme code, per channel.

    Testing against the nominal int16 rails does not work: this device's
    ceiling code is 32765, not 32767, so a capture with 38 % of its samples
    railed reported zero. What identifies clipping regardless of where the
    converter's ceiling actually falls is that the samples pile up on ONE code.
    A genuine plateau is spread across many neighbouring codes by noise -- in a
    correctly-ranged capture of the same rig, the most populated top code holds
    9 samples out of 54 million.
    """
    out = {}
    for ch in channels:
        key = f"samples_ch{ch}"
        if key not in z:
            continue
        raw = z[key]
        if raw.size == 0:
            continue
        pinned = int((raw == raw.max()).sum() + (raw == raw.min()).sum())
        out[ch] = pinned / raw.size
    return out


def check(z, channels=(1,), tolerance=0.0005, where=""):
    """Raise ClippedCapture if any ANALOGUE channel is clipped beyond `tolerance`.

    Pass only the channels carrying a waveform whose amplitude is used -- the
    photodiode, or an audio line. A logic channel must NOT be included: a TTL
    line rests on the rail whenever it is high, by design, and on this rig
    channel 2 shows 2.7-2.8 % of samples on the top code, which is exactly its
    15 ms pulse in a 500 ms cycle. That is not a range error, and it costs
    nothing, because an edge time is read at a fixed mid-level threshold that
    the transition passes through on its way to the rail.

    The default tolerance allows a handful of samples -- the good captures of
    the same rig show 4 to 13 samples on the extreme code out of 54 million,
    which is a signal momentarily touching its own peak, not a wrong range.
    """
    bad = {ch: f for ch, f in clipping(z, channels).items() if f > tolerance}
    if not bad:
        return
    lines = [f"clipped capture{': ' + where if where else ''}"]
    for ch, frac in sorted(bad.items()):
        rng = float(z[f"range_v_ch{ch}"]) if f"range_v_ch{ch}" in z else float("nan")
        off = float(z[f"offset_v_ch{ch}"]) if f"offset_v_ch{ch}" in z else float("nan")
        lines.append(
            f"  channel {ch}: {100*frac:.2f} % of samples resting on one extreme code "
            f"(range {rng:.2f} V, offset {off:.2f} V "
            f"-> window {off - rng/2:.2f} .. {off + rng/2:.2f} V)")
    lines += [
        "",
        "The peaks are gone, so every level-crossing criterion is biased and the",
        "durations this would report are wrong -- and wrong in the direction that",
        "looks like a clean result. Re-record with a range that contains the whole",
        "signal; the photodiode of this rig reaches about 8.2 V, which needs the",
        "50 V range:",
        "",
        "  ad3-capture --seconds <n> --rate 1e5 --channels 1,2 --save-raw \\",
        "              --range 50,5 --offset 0,2.5 --out <file>.npz",
    ]
    raise ClippedCapture("\n".join(lines))
