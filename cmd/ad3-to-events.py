#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

"""Turn a two-channel AD3 capture into the events CSV that timing-drift reads.

    ./cmd/ad3-to-events.py capture.npz events.csv [presented_ms]
    ./_build/timing-drift events.csv <run>.csv

CH1 is the photodiode, CH2 the TTL. Light onsets use the 10 % criterion, the
same one extract-onsets.py uses, so an AD3 session and a BBTK session answer the
same question — the AD3 just resolves it to 10 us rather than 0.25 ms.

# Why bother, when the AD3 already has both channels on one timebase

Because the interval that matters is not TTL-to-light, which is what one
instrument can measure alone. It is FLIP TIMESTAMP to photons: what the
framework told the experiment, against what the participant saw. That needs the
host clock, which arrives with the run's own CSV, and a way to relate it to the
instrument's — which is what timing-drift fits from the trigger-versus-flip
relation, over hundreds of events, so a constant trigger latency cancels.

Measured on a Precision 5490 on 2026-08-18, the difference is not academic: on
one stack the two quantities agreed (TTL-to-light SD 0.076 ms, flip-to-photons
0.070 ms), and on the other the flip timestamps carried a ramp the TTL series
did not make obvious.

Needs `ad3` (github.com/chrplr/ad3-capture) only for the capture itself; this
script reads the .npz with numpy and nothing else.
"""
import os
import sys

import numpy as np

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import ad3_check

cap, out = sys.argv[1], sys.argv[2]
# Optional third argument: the duration the host actually presented, in ms
# (frames_on x frame period, both in the run's -info.txt). Without it the bias
# of a level criterion can be estimated from the edge shapes but not checked;
# with it, it is simply measured.
presented_ms = float(sys.argv[3]) if len(sys.argv) > 3 else None
z = np.load(cap); rate = float(z["rate"])
# Refuse before doing anything else. A clipped capture yields duration figures
# that are wrong AND unusually self-consistent, so nothing further downstream
# would call them into question.
ad3_check.check(z, channels=(1,), where=cap)
def v(ch): return z[f"offset_v_ch{ch}"] + z[f"range_v_ch{ch}"]*z[f"samples_ch{ch}"].astype(float)/65536
pd_, ttl = v(1), v(2)

tlo, thi = np.percentile(ttl, (1, 99))
above = ttl >= tlo + 0.5*(thi-tlo)
ttl_on = np.flatnonzero(above[1:] & ~above[:-1])
ttl_off = np.flatnonzero(~above[1:] & above[:-1])
plo, phi = np.percentile(pd_, (1, 99))

PRE, POST = int(0.008*rate), int(0.400*rate)
rows = []
rise, fall = [], []
dur_at = {"05": [], "50": [], "95": []}
for e in ttl_on:
    if e-PRE < 0 or e+POST >= len(pd_):
        continue
    seg = pd_[e-PRE:e+POST]
    base = np.median(seg[:PRE]); peak = np.percentile(seg[PRE:int(0.15*rate)+PRE], 90)
    if peak-base < 0.5*(phi-plo):
        continue
    thr = base + 0.10*(peak-base)
    h = np.flatnonzero(seg[PRE:] >= thr)
    if not h.size:
        continue
    on = (e-PRE+PRE+h[0])/rate*1000
    # offset: first fall back below the same level after the plateau
    after = seg[PRE+h[0]+int(0.05*rate):]
    f = np.flatnonzero(after < thr)
    dur = (f[0]+int(0.05*rate))/rate*1000 if f.size else 0.0
    rows.append(("Opto1", on, dur))
    # Panel response, for the summary below. Levels are taken per trial so a
    # slow drift in backlight output does not leak into the transition times,
    # and so that a run whose panel is still warming shows it in the spread.
    def cross(level, rising, start):
        t = base + level*(peak-base)
        i = np.flatnonzero(seg[start:] >= t) if rising else np.flatnonzero(seg[start:] <= t)
        return start+i[0] if i.size else None
    r05, r50, r95 = cross(0.05, True, PRE), cross(0.50, True, PRE), cross(0.95, True, PRE)
    if r95 is not None:
        settle = r95 + int(0.05*rate)
        f95, f50, f05 = (cross(0.95, False, settle), cross(0.50, False, settle),
                         cross(0.05, False, settle))
        if None not in (r05, r50, f95, f50, f05):
            # Delay from the start of each transition to the 50 % crossing: the
            # two quantities a 50 % criterion actually differences.
            rise.append((r50-r05)/rate*1000)
            fall.append((f50-f95)/rate*1000)
            # Durations at three criteria. On a panel whose two edges differ in
            # SHAPE rather than only in length, these do not agree, and the
            # disagreement is the point -- see the note printed below.
            for lvl, (a, b) in (("05", (r05, f05)), ("50", (r50, f50)), ("95", (r95, f95))):
                dur_at[lvl].append((b-a)/rate*1000)
k = min(len(ttl_on), len(ttl_off))
for a, b in zip(ttl_on[:k], ttl_off[:k]):
    if b > a:
        rows.append(("TTLin1", a/rate*1000, (b-a)/rate*1000))
rows.sort(key=lambda r: r[1])
with open(out, "w") as fh:
    fh.write("Type,Onset,Duration,DurationCorrected\n")
    for t, on, d in rows:
        fh.write(f"{t},{on:.4f},{d:.4f},{d:.4f}\n")
print(f"{out}: {sum(1 for r in rows if r[0]=='Opto1')} Opto1, "
      f"{sum(1 for r in rows if r[0]=='TTLin1')} TTLin1")

# Panel response is not a curiosity: it sets how much of the measured duration
# is the panel rather than the software. Any fixed-level criterion applied to an
# asymmetric transition biases the interval by roughly (fall-rise)/2, so the SAME
# monitor driven by two machines reported durations 5.3 ms apart on 2026-08-18
# while the trigger-to-trigger SOA agreed to 3 decimals. Print it every time, so
# a cross-machine duration comparison can be corrected instead of believed.
if rise:
    r, f_ = np.array(rise), np.array(fall)
    # A 50 % criterion times the ONSET at t_on + D_rise and the OFFSET at
    # t_off + D_fall, so the interval it returns is the presented one plus
    # (D_fall - D_rise). Those two delays are what is measured here: from the
    # start of each transition (5 % up, 95 % down) to its 50 % crossing.
    #
    # This replaced (fall - rise)/2 over the 10-90 %/90-10 % times, which is a
    # proxy for the same thing and a poor one.
    est_bias = f_.mean() - r.mean()
    print(f"panel: rise 5->50% {r.mean():.2f} ms, fall 95->50% {f_.mean():.2f} ms, n={len(r)}")
    if all(dur_at[k] for k in dur_at):
        d05, d50, d95 = (np.array(dur_at[k]) for k in ("05", "50", "95"))
        print(f"panel: duration at  5 % {d05.mean():8.3f} ms |"
              f"  50 % {d50.mean():8.3f} ms |  95 % {d95.mean():8.3f} ms")
        if presented_ms is not None:
            # The only rigorous version: the presented duration is known, so
            # each criterion's bias is measured rather than modelled.
            print(f"panel: presented {presented_ms:.3f} ms -> bias at "
                  f"5 % {d05.mean()-presented_ms:+.2f}, "
                  f"50 % {d50.mean()-presented_ms:+.2f}, "
                  f"95 % {d95.mean()-presented_ms:+.2f} ms")
            unexplained = (d50.mean() - presented_ms) - est_bias
            print(f"panel: edge shapes predict {est_bias:+.2f} ms of the 50 % bias; "
                  f"{unexplained:+.2f} ms is not")
            if abs(unexplained) > 0.5:
                # The gap is dead time: an interval after the drive changes
                # during which the panel has not yet visibly moved. No threshold
                # on the luminance trace can see it, because there is nothing to
                # see. Measured on a Dell U2720Q, 6.8 ms of the fall.
                print("panel:         that gap is dead time before the panel starts to move,")
                print("panel:         which the luminance trace cannot show. Quote the")
                print("panel:         criterion with the duration; do not correct for it.")
        else:
            print(f"panel: edge shapes predict a 50 % criterion reads {est_bias:+.2f} ms long")
            print("panel:         (pass the presented duration in ms as a 3rd argument to")
            print("panel:          measure the bias instead of estimating it)")
    # Settling check on the measured duration itself, not on an edge time. A
    # panel that has just changed mode drifts, and the 50 % duration is where
    # that shows: the W5700 run of 2026-08-17 ramped 0.68 ms across 2.3 minutes
    # while its 5->50 % rise delay moved only 0.19 ms, so watching the edge
    # would have called that run settled when it was not.
    if dur_at["50"]:
        d = np.array(dur_at["50"])
        third = len(d)//3
        if third:
            drift = d[-third:].mean()-d[:third].mean()
            note = "still settling - let it warm" if abs(drift) > 0.3 else "steady"
            print(f"panel: duration drift first-to-last third {drift:+.2f} ms ({note})")
