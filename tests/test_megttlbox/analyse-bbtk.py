#!/usr/bin/env python3
"""Analyse a run-bbtk.sh capture: pulse widths per block, and block-D skew."""
import csv
import statistics as st
import sys

path = sys.argv[1]
rows = []
with open(path) as f:
    for r in csv.DictReader(f):
        rows.append((r["Type"], float(r["Onset"]), float(r["Duration"])))

ttl2 = [(o, d) for t, o, d in rows if t == "TTLin2"]  # line 0 / D30
ttl1 = [(o, d) for t, o, d in rows if t == "TTLin1"]  # line 1 / D31

print(f"total events: {len(rows)}   TTLin2: {len(ttl2)}   TTLin1: {len(ttl1)}")
print("expected:     101          TTLin2: 81          TTLin1: 20\n")

marker, rest = ttl2[0], ttl2[1:]
print(f"marker: onset {marker[0]:.2f} ms, width {marker[1]:.2f} ms (requested 100)\n")

blocks = [
    ("A", 5.0, rest[0:20]),
    ("B", 10.0, rest[20:40]),
    ("C", 20.0, rest[40:60]),
    ("D", 10.0, rest[60:80]),
]

print("PULSE WIDTH (millis() truncation means realised width should be in [w-1, w])")
print(f"{'block':<6}{'req':>6}{'n':>4}{'min':>9}{'median':>9}{'max':>9}{'spread':>9}{'mean err':>10}")
for label, req, evs in blocks:
    w = [d for _, d in evs]
    if not w:
        print(f"{label:<6}{req:>6.0f}  NO EVENTS")
        continue
    spread = max(w) - min(w)
    err = st.mean(w) - req
    print(f"{label:<6}{req:>6.0f}{len(w):>4}{min(w):>9.2f}{st.median(w):>9.2f}"
          f"{max(w):>9.2f}{spread:>9.2f}{err:>+10.2f}")

print("\nINTER-STIMULUS INTERVAL (onset to onset; requested 500 ms + pulse width)")
for label, req, evs in blocks:
    ons = [o for o, _ in evs]
    if len(ons) < 2:
        continue
    isis = [b - a for a, b in zip(ons, ons[1:])]
    print(f"  block {label}: min {min(isis):.2f}  median {st.median(isis):.2f}  "
          f"max {max(isis):.2f}  spread {max(isis) - min(isis):.2f}")

print("\nBLOCK D SKEW (same command pulses both lines; port written in one instruction)")
d2 = blocks[3][2]
if not ttl1 or not d2:
    print("  cannot compute — missing events on one channel")
else:
    pairs = []
    for o2, _ in d2:
        o1, _ = min(ttl1, key=lambda e: abs(e[0] - o2))
        pairs.append(o1 - o2)   # TTLin1 (line 1) minus TTLin2 (line 0)
    print(f"  n={len(pairs)}  min {min(pairs):+.2f}  median {st.median(pairs):+.2f}  "
          f"max {max(pairs):+.2f} ms")
    if max(abs(p) for p in pairs) == 0.0:
        print("  every pair identical to the sample -> no measurable skew")
    else:
        print(f"  largest |skew| = {max(abs(p) for p in pairs):.2f} ms")
    w1 = [d for _, d in ttl1]
    print(f"  TTLin1 widths: min {min(w1):.2f}  median {st.median(w1):.2f}  max {max(w1):.2f}")
