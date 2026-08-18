#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

"""Extract per-trial audio-visual lag from a two-channel AD3 capture.

    ./cmd/extract-av-sync.py capture.npz avsync.npz

Analyses what `tests/Timing-Tests -test av` produced, from a capture taken with
`ad3-capture` (github.com/chrplr/ad3-capture, which this imports and which must
be installed: `pip install -e /path/to/ad3-capture`). It lived in that repo's
predecessor, dlp-io8-g, until 2026-08-18, and the commit history of its three
bug fixes is still there.

One channel carries a photodiode watching a flashing patch, the other a
microphone hearing a tone that is meant to start with it. For every flash this
finds the light onset and the sound onset and writes their interval:

    light_ms  the flash onset on the instrument's clock
    audio_ms  the tone onset, same clock
    lag_ms    audio_ms - light_ms, positive when the sound is late

Both channels are recorded in one acquisition, so the instrument's clock cancels
in `lag_ms` and never has to be reconciled with the host's.

# Why the microphone cannot use rising_edges

A photodiode's signal is a step, so an edge is well defined and extract-onsets.py
finds it directly. A 440 Hz tone is not: it crosses any fixed threshold twice per
cycle, 880 times a second, for as long as it sounds. What has an onset is its
ENVELOPE, so this computes one — the running RMS over `--smooth-ms` — and finds
the crossing there.

The window is CENTRED rather than trailing. A trailing window is causal, which
matters when something must react in real time and not at all when a file is
being read afterwards; what it costs is a delay of half the window, landing
directly in the lag being measured. Centring does not make the onset unbiased —
it converts a late bias into an early one, because a symmetric smear starts
rising before the tone does — but the bias becomes a fixed fraction of the
window instead of a full half of it, and it is identical on every trial.

That distinction is the important one. Whatever the level and window are, they
shift every trial by the same amount, so the SCATTER and the SLOPE of the lag —
the parts that cannot be compensated by scheduling the tone earlier — are
unaffected. The absolute figure is not a calibration and should not be quoted as
one without a known-simultaneous reference.

Measured on a synthetic capture (100 kS/s, a 6 ms panel ramp, a 2 ms tone attack,
a true onset difference of 26.500 ms): recovered 25.590 ms, SD 0.074. The 0.9 ms
shortfall is the sum of the two level choices and this window — a constant,
exactly as described — while the SD shows the method itself adds essentially no
jitter at this sample rate.

# What the synthetic did NOT catch

Three bugs were found by real captures in one afternoon on 2026-08-17, all of
which this file's synthetic test passed: duplicate trial anchors from threshold
chatter on the panel's ramp, the tone's own attack counted as a dropout, and the
crossing search clamped at the anchor so every lag came out 6.68 ms short. None
announced itself; each produced a plausible number.

What caught all three was an impossible one — 15801 "edges" on an audio channel,
an identical 0.57 ms gap in all 481 tones, a 2.61 ms lag under a 5.33 ms buffer.
So the checks that earn their place here are the ones that compare a result
against a physical bound, not the ones that compare it against a fixture.

# Why the level is a fraction of each trial's own plateau

A tone's rise is not instantaneous: the speaker, the room and the microphone
each stretch it. So "the onset" is a choice of level, and a different choice
shifts every number. 10% is the earliest level that is safely clear of the noise
floor, matching extract-onsets.py's choice for the photodiode so the two channels
are treated alike. What matters more than the choice is that it is the same
choice everywhere, so it is recorded in the output.

Both channels are measured against their own trial's baseline and plateau rather
than a capture-wide constant, because neither the room's noise floor nor the
panel's black level is guaranteed to hold still for 500 seconds.

# Gaps in the tone are reported, not smoothed over

An audio buffer the machine cannot keep filled underruns, which puts silence in
the middle of a tone. It is audible as scratching and invisible in every
host-side statistic, because the software hands the tone over on time either
way — measured in the same runs, the software-side SOA read 0.080 ms +- 0.035
throughout.

Measured on a Raspberry Pi 4 (PipeWire, 48 kHz), capturing the line output
directly at 100 kS/s on 2026-08-17: 2.0% of 500 tones torn at 512 frames, 20.7%
of 483 at 256, none in 481 at 1024. Gaps ran from 0.5 to 37.7 ms. A BBTK
microphone channel had reported the 512 case as 23% of tones with gaps whose
median was 22.3 ms; the tearing is real, but a threshold detector watching an
acoustic envelope needs the signal to recover past its threshold before it calls
the tone present again, so it reports gaps far longer than the electrical ones.

Set `--gap-ms` below the shortest glitch worth knowing about: the first one found
here was 1.9 ms, well under the 5 ms default.

# Why trials are dropped

A trial is skipped when its window runs off either end of the capture, when the
light does not rise by at least `--min-amplitude` of the capture's full swing
(the photodiode was not on the patch), or when the tone never crosses its level
inside `--post-ms` (no sound, or a lag longer than the window). Averaging any of
those in would bias the result quietly rather than announce itself, so each is
counted and reported.
"""
import argparse
import os
import sys

import numpy as np

from ad3 import logic_levels


def envelope(x, rate, smooth_ms):
    """Running RMS of x about its own mean, over a centred window.

    The mean is removed first because a microphone input sits on whatever DC its
    preamp gives it, and an RMS taken about zero would report that offset as
    signal — a channel with a 0.2 V bias and no sound would look as loud as one
    with a quiet tone.
    """
    n = max(1, int(round(smooth_ms * rate / 1000)))
    ac = x - np.mean(x)
    # 'same' centres the window, which is what makes this delay-free; the first
    # and last n/2 samples are averaged over a short window and are not used
    # (every trial window sits well inside the capture).
    return np.sqrt(np.convolve(ac * ac, np.ones(n) / n, mode="same"))


def crossing(sig, base, plateau, level):
    """First index where sig reaches level of the way from base to plateau.

    Returns None when it never does, which the caller counts rather than
    silently dropping.
    """
    thr = base + level * (plateau - base)
    hit = np.flatnonzero(sig >= thr)
    return int(hit[0]) if hit.size else None


def main():
    p = argparse.ArgumentParser(description=__doc__,
                                formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("capture", help="two-channel .npz with raw samples (ad3-capture.py --save-raw)")
    p.add_argument("out", help="output .npz")
    p.add_argument("--light-ch", type=int, default=1, help="photodiode channel, 1 or 2 (default 1)")
    p.add_argument("--audio-ch", type=int, default=2, help="microphone channel, 1 or 2 (default 2)")
    p.add_argument("--level", type=float, default=0.10,
                   help="fraction of each trial's own rise to call an onset (default 0.10)")
    p.add_argument("--smooth-ms", type=float, default=2.5,
                   help="RMS window for the audio envelope; one period of the tone "
                        "(default 2.5, i.e. 440 Hz)")
    p.add_argument("--pre-ms", type=float, default=30.0,
                   help="window searched before the anchor, and the baseline taken from its "
                        "first third. Must exceed how far the 10%% crossing precedes the 50%% "
                        "anchor — 6.7 ms on a 16 ms panel transition (default 30)")
    p.add_argument("--post-ms", type=float, default=150.0,
                   help="how far after the flash to look for the tone (default 150)")
    p.add_argument("--min-amplitude", type=float, default=0.5,
                   help="reject a trial whose light rise is below this fraction of the "
                        "capture's full swing (default 0.5)")
    p.add_argument("--gap-ms", type=float, default=5.0,
                   help="silence this long inside a tone counts as a dropout (default 5)")
    p.add_argument("--refractory", type=float, default=0.5,
                   help="drop a flash closer to the previous one than this fraction of "
                        "the median interval; threshold chatter on the panel's ramp (default 0.5)")
    p.add_argument("--gap-level", type=float, default=0.3,
                   help="envelope below this fraction of the plateau counts as silence (default 0.3)")
    args = p.parse_args()

    if args.light_ch == args.audio_ch:
        sys.exit("--light-ch and --audio-ch must differ: the two signals are on separate inputs")

    z = np.load(args.capture)
    for ch in (args.light_ch, args.audio_ch):
        if f"samples_ch{ch}" not in z.files:
            sys.exit(f"{args.capture} has no samples_ch{ch}; keys are {', '.join(z.files)}\n"
                     f"re-capture with ad3-capture.py --save-raw (the envelope needs the "
                     f"samples, not the edges)")
    rate = float(z["rate"])

    def volts(ch):
        return (z[f"offset_v_ch{ch}"]
                + z[f"range_v_ch{ch}"] * z[f"samples_ch{ch}"].astype(np.float64) / 65536)

    light, audio = volts(args.light_ch), volts(args.audio_ch)
    env = envelope(audio, rate, args.smooth_ms)

    # Trial anchors: the flashes. ad3-capture already saved the 50% crossings,
    # so reuse them rather than re-deriving a second set that could disagree.
    key = f"rise_ch{args.light_ch}"
    if key in z.files and np.size(z[key]):
        anchors = np.asarray(z[key], dtype=np.float64)
    else:
        lo, hi = logic_levels(light)
        above = light >= lo + 0.5 * (hi - lo)
        anchors = np.flatnonzero(above[1:] & ~above[:-1]) / rate

    # Refractory period. A panel does not switch instantly — 5.5-6.5 ms black to
    # white on the 1905FP — so the threshold is crossed on a ramp, and a few mV
    # of noise on that ramp re-crosses it. Every re-crossing is another "flash":
    # in a synthetic capture with 4 mV of noise, one 6 ms ramp produced two
    # anchors 0.02 ms apart, and the duplicate then reported its neighbour's tone
    # as a second trial — inventing a dropout that was not there.
    #
    # The floor is a fraction of the median interval rather than a constant, so
    # it needs no assumption about the cycle length and adapts to whatever train
    # was recorded. Half a cycle is far above any plausible chatter and far below
    # any real flash-to-flash interval.
    dupes = 0
    if len(anchors) > 2:
        floor_s = args.refractory * float(np.median(np.diff(anchors)))
        kept = [anchors[0]]
        for a in anchors[1:]:
            if a - kept[-1] < floor_s:
                dupes += 1
            else:
                kept.append(a)
        anchors = np.asarray(kept)

    # The rejection floor is a fraction of the whole capture's swing, not of the
    # trial's own, because a trial with no flash in it has no swing to take a
    # fraction of.
    plo, phi = np.percentile(light[::97], (1, 99))
    floor = args.min_amplitude * (phi - plo)

    PRE = int(args.pre_ms * rate / 1000)
    POST = int(args.post_ms * rate / 1000)
    GAP = max(1, int(args.gap_ms * rate / 1000))

    light_ms, audio_ms, gaps, gap_ms = [], [], [], []
    off_end = low = no_light = no_audio = 0

    for t in anchors:
        i = int(round(t * rate))
        a, b = i - PRE, i + POST
        if a < 0 or b >= len(light):
            off_end += 1
            continue

        lseg, aseg = light[a:b], env[a:b]
        # Search the WHOLE window, not just from the anchor onward.
        #
        # The anchor is a 50% crossing and the onsets are 10% crossings, so both
        # happen BEFORE it — on this panel the light reaches 10% a median of
        # 6.68 ms before it reaches 50%, because the transition takes 16 ms.
        # Searching forward from the anchor clamped that to zero and silently
        # turned every lag into audio-minus-light-at-50%, 6.68 ms too small; at
        # a short buffer, where the sound arrives before the panel is half lit,
        # it clamped the audio too and 51 of 483 trials read exactly 0.000.
        # Nothing prevents a lag being negative and the code must not either.
        BASE = max(1, PRE // 3)
        lbase = np.median(lseg[:BASE])
        lpeak = np.percentile(lseg, 90)
        if lpeak - lbase < floor:
            low += 1
            continue
        li = crossing(lseg, lbase, lpeak, args.level)
        if li is None:
            no_light += 1
            continue

        abase = np.median(aseg[:BASE])
        apeak = np.percentile(aseg, 90)
        ai = crossing(aseg, abase, apeak, args.level)
        if ai is None:
            no_audio += 1
            continue

        light_ms.append((a + li) / rate * 1000)
        audio_ms.append((a + ai) / rate * 1000)

        # Dropouts: a run of samples below --gap-level of this trial's plateau,
        # lasting at least --gap-ms.
        #
        # The search starts where the envelope FIRST REACHES that level, not at
        # the onset. Starting at the onset counts the tone's own attack as a
        # gap, because the onset is by definition the 10% crossing and 10% is
        # below any sensible gap level. That stayed invisible while --gap-ms was
        # larger than the attack and then reported 481 of 481 tones torn, each
        # by the same 0.57 ms, the moment a 481-tone capture was analysed at
        # --gap-ms 0.5. Identical "defects" in every trial are the signature of
        # the analysis, not the apparatus.
        quiet_level = abase + args.gap_level * (apeak - abase)
        risen = np.flatnonzero(aseg[ai:] >= quiet_level)
        if not risen.size:
            gaps.append(0)
            gap_ms.append(0.0)
            continue
        body = aseg[ai + int(risen[0]):]
        quiet = body < quiet_level
        n_gap, longest, run = 0, 0, 0
        for q in quiet:
            run = run + 1 if q else 0
            if run == GAP:
                n_gap += 1
            longest = max(longest, run)
        gaps.append(n_gap)
        gap_ms.append(longest / rate * 1000 if longest >= GAP else 0.0)

    if not light_ms:
        sys.exit("  no usable trials: check the channel assignment (--light-ch/--audio-ch), "
                 "the probe attenuation, and that both signals are inside the capture window.")

    light_ms = np.asarray(light_ms)
    audio_ms = np.asarray(audio_ms)
    lag = audio_ms - light_ms
    gaps = np.asarray(gaps)
    gap_ms = np.asarray(gap_ms)

    # Slope over trials: a lag that slides is a different failure from one that
    # scatters, and only the constant part can be compensated by scheduling the
    # tone earlier. Reported in the same shape as timing-drift's.
    idx = np.arange(len(lag), dtype=np.float64)
    slope = np.polyfit(idx, lag, 1)[0] if len(lag) > 2 else 0.0
    detrended = lag - (slope * (idx - idx.mean()) + lag.mean())

    np.savez_compressed(args.out, light_ms=light_ms, audio_ms=audio_ms, lag_ms=lag,
                        gaps=gaps, gap_ms=gap_ms, level=args.level,
                        smooth_ms=args.smooth_ms, gap_level=args.gap_level,
                        gap_min_ms=args.gap_ms, rate=rate,
                        light_ch=args.light_ch, audio_ch=args.audio_ch)

    print(f"  {len(anchors)} flashes ({dupes} duplicate crossings dropped), {len(lag)} usable "
          f"({off_end} off the end, {low} below the amplitude floor, "
          f"{no_light} light never crossed, {no_audio} tone never crossed)")
    print(f"  audio - light at {args.level:.0%} (envelope {args.smooth_ms:.2f} ms RMS):")
    print(f"    median {np.median(lag):8.3f} ms   mean {lag.mean():8.3f}   SD {lag.std(ddof=1):7.3f}")
    print(f"    p5/p95 {np.percentile(lag, 5):8.3f} / {np.percentile(lag, 95):.3f}"
          f"   min/max {lag.min():.3f} / {lag.max():.3f}")
    print(f"    slope  {slope * 1000:+8.3f} us/trial = {slope * len(lag):+.3f} ms over the run")
    print(f"    de-trended SD {detrended.std(ddof=1):.3f} ms  <- what cannot be compensated")
    torn = int((gaps > 0).sum())
    if torn:
        print(f"  DROPOUTS: {torn} of {len(lag)} tones ({100 * torn / len(lag):.1f} %) contain "
              f"silence >= {args.gap_ms:g} ms")
        print(f"    longest gap per torn tone: median {np.median(gap_ms[gaps > 0]):.2f} ms, "
              f"max {gap_ms.max():.2f} ms")
        print(f"    a torn tone is an underrun, not a detector artefact — raise the audio "
              f"buffer and re-run before quoting the lag above.")
    else:
        print(f"  no tone contains silence >= {args.gap_ms:g} ms: the audio path kept up.")
    print(f"  wrote {args.out}")


if __name__ == "__main__":
    main()
