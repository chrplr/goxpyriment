#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

"""Turn a two-channel AD3 capture into the events CSV that timing-drift reads.

    ./cmd/ad3-to-events.py capture.npz events.csv
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
import sys
import numpy as np

cap, out = sys.argv[1], sys.argv[2]
z = np.load(cap); rate = float(z["rate"])
def v(ch): return z[f"offset_v_ch{ch}"] + z[f"range_v_ch{ch}"]*z[f"samples_ch{ch}"].astype(float)/65536
pd_, ttl = v(1), v(2)

tlo, thi = np.percentile(ttl, (1, 99))
above = ttl >= tlo + 0.5*(thi-tlo)
ttl_on = np.flatnonzero(above[1:] & ~above[:-1])
ttl_off = np.flatnonzero(~above[1:] & above[:-1])
plo, phi = np.percentile(pd_, (1, 99))

PRE, POST = int(0.008*rate), int(0.400*rate)
rows = []
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
