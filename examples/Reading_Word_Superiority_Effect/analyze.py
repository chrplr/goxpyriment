#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
"""
analyze.py — analyse Reading_Word_Superiority_Effect data files.

Usage
-----
    python3 analyze.py ~/goxpy_data/Word-Superiority-Effect_sub-001_*.csv
    python3 analyze.py --errors FILE.csv          # also list every main-task error

Each file is analysed on its own. The companion -info.txt is read when present
(language, display mode, frame duration). With several files, a group summary
follows: one row per file (with the median measured main-task duration) and
an exact sign test across files.

Per file:
  - session: language, fullscreen mode, frame duration, main-task duration
  - timing: trials whose measured duration is off the requested frames
  - staircase: the track, accuracy at each duration, threshold
  - main task: accuracy per material (Wilson 95 % interval), median RT,
    item-paired exact McNemar tests, accuracy by critical position,
    accuracy by measured stimulus duration (stim_dur_ms in whole frames),
    response bias (top/bottom), first vs second half
  - calibration: staircase accuracy at the main-task duration vs main-task
    letter accuracy (they should agree if the staircase calibrates the task)

Standard library only; no install needed.
"""

import csv
import math
import pathlib
import statistics
import sys
from collections import Counter, defaultdict
from math import comb

MATERIALS = ["word", "anagram", "letter"]
PAIRS = [("word", "letter"), ("word", "anagram"), ("anagram", "letter")]


# ── Statistics ───────────────────────────────────────────────────────────────

def wilson(k, n, z=1.96):
    """Wilson score interval for a binomial proportion."""
    if n == 0:
        return float("nan"), float("nan")
    p = k / n
    d = 1 + z * z / n
    c = (p + z * z / (2 * n)) / d
    h = z * math.sqrt(p * (1 - p) / n + z * z / (4 * n * n)) / d
    return c - h, c + h


def binom_two_sided(k, n):
    """Exact two-sided binomial test of k successes in n against p = 0.5."""
    if n == 0:
        return 1.0
    k = min(k, n - k)
    return min(1.0, 2 * sum(comb(n, j) for j in range(k + 1)) / 2 ** n)


def binom_above_chance(k, n):
    """Exact one-sided P(X >= k) for X ~ Binomial(n, 0.5)."""
    return sum(comb(n, j) for j in range(k, n + 1)) / 2 ** n


def pct(k, n):
    return f"{k}/{n} = {100 * k / n:5.1f}%" if n else "n/a"


def measured_frames(r, frame_ms):
    """Measured stimulus duration of a trial, in whole frames."""
    return round(r["dur"] / frame_ms)


# ── Reading ──────────────────────────────────────────────────────────────────

def read_trials(path):
    with open(path, newline="", encoding="utf-8") as f:
        rows = list(csv.DictReader(f))
    for r in rows:
        r["trial"] = int(r["trial"])
        r["frames"] = int(r["frames"])
        r["dur"] = float(r["stim_dur_ms"])
        r["rt"] = int(r["rt_ms"])
        r["correct"] = r["correct"].strip().lower() == "true"
    return rows


def read_info(csv_path):
    """Return the -info.txt companion as {key: value}, or {} if absent."""
    info_path = csv_path.with_name(csv_path.stem + "-info.txt")
    info = {}
    if not info_path.exists():
        return info
    for line in info_path.read_text(encoding="utf-8").splitlines():
        line = line.lstrip("#").strip()
        # Lines look like "sys fullscreen_mode: desktop" or "language: fr".
        if ":" not in line:
            continue
        key, _, value = line.partition(":")
        key = key.split()[-1] if key.split() else key
        info[key] = value.strip()
    return info


# ── Report ───────────────────────────────────────────────────────────────────

def analyse(path, list_errors=False):
    rows = read_trials(path)
    info = read_info(path)
    main = [r for r in rows if r["phase"] == "main"]
    stair = [r for r in rows if r["phase"] == "staircase"]

    if not rows:
        # A session quit before the first trial leaves a header-only CSV.
        print(f"{path.name}: no trials, skipped")
        return None

    print("=" * 78)
    print(path.name)
    print("=" * 78)

    # Session
    frame_ms = float(info.get("frame_duration_ms", "nan"))
    if math.isnan(frame_ms):
        # No info file: estimate from the practice trials, which are long.
        per = [r["dur"] / r["frames"] for r in rows]
        frame_ms = statistics.median(per)
        print(f"(no -info.txt; frame duration estimated as {frame_ms:.3f} ms)")
    main_frames = main[0]["frames"] if main else None
    subject = rows[0]["subject_id"] if rows else "?"
    mode = f" ({info['fullscreen_mode']})" if "fullscreen_mode" in info else ""
    print(f"subject {subject}, language {info.get('language', '?')}, "
          f"video {info.get('video_driver', '?')}, "
          f"fullscreen {info.get('fullscreen', '?')}{mode}")
    main_dur = (f"main task at {main_frames} frames = {main_frames * frame_ms:.1f} ms"
                if main else "main task not reached")
    print(f"frame {frame_ms:.3f} ms; {main_dur}; session {info.get('duration', '?')}")
    print(f"trials: " + ", ".join(f"{k} {v}" for k, v in Counter(r['phase'] for r in rows).items()))

    # Timing: a quarter of a frame either way is beyond timestamp jitter.
    off = [r for r in rows if abs(r["dur"] - r["frames"] * frame_ms) > frame_ms / 4]
    print(f"\nTiming: {len(off)}/{len(rows)} trials off the requested duration by > 1/4 frame")
    for r in off:
        print(f"  {r['phase']:9s} trial {r['trial']:3d}: {r['frames']} frames requested "
              f"({r['frames'] * frame_ms:.1f} ms), measured {r['dur']:.1f} ms "
              f"({r['dur'] / frame_ms:.2f} frames)")

    # Staircase
    if stair:
        print(f"\nStaircase ({len(stair)} trials; x = error):")
        track = [f"{r['frames']}{'' if r['correct'] else 'x'}" for r in stair]
        for i in range(0, len(track), 20):
            print("  " + " ".join(track[i:i + 20]))
        byf = defaultdict(list)
        for r in stair:
            byf[r["frames"]].append(r["correct"])
        print("  accuracy by frames: " + ", ".join(
            f"{f}: {sum(v)}/{len(v)}" for f, v in sorted(byf.items())))
        bym = defaultdict(list)
        for r in stair:
            bym[measured_frames(r, frame_ms)].append(r["correct"])
        if bym.keys() != byf.keys() or any(len(bym[f]) != len(byf[f]) for f in byf):
            print("  accuracy by measured frames: " + ", ".join(
                f"{f}: {sum(v)}/{len(v)}" for f, v in sorted(bym.items())))
        if "staircase_threshold_frames" in info:
            print(f"  threshold: {info['staircase_threshold_frames']}")
    else:
        print("\nStaircase: skipped (-frames given)")

    if not main:
        print("\nNo main-task trials.")
        return None

    # Accuracy per material
    print(f"\nMain task ({len(main)} trials)")
    print(f"  {'material':8s}  {'correct':>17s}  {'95% CI':>11s}  {'p>chance':>8s}  "
          f"{'RT all':>6s}  {'RT corr':>7s}")
    acc = {}
    for m in MATERIALS:
        R = [r for r in main if r["material"] == m]
        k, n = sum(r["correct"] for r in R), len(R)
        acc[m] = (k, n)
        lo, hi = wilson(k, n)
        rt_all = statistics.median(r["rt"] for r in R) if R else float("nan")
        rt_c = [r["rt"] for r in R if r["correct"]]
        rt_c = statistics.median(rt_c) if rt_c else float("nan")
        print(f"  {m:8s}  {pct(k, n):>17s}  {lo:.2f}–{hi:.2f}  "
              f"{binom_above_chance(k, n):8.3f}  {rt_all:6.0f}  {rt_c:7.0f}")

    # Item-paired comparisons: every item appears once in each material.
    by_item = {(r["item"], r["material"]): r["correct"] for r in main}
    items = sorted({r["item"] for r in main})
    print("\n  Item-paired exact McNemar tests:")
    for a, b in PAIRS:
        both = [i for i in items if (i, a) in by_item and (i, b) in by_item]
        a_only = sum(by_item[(i, a)] and not by_item[(i, b)] for i in both)
        b_only = sum(by_item[(i, b)] and not by_item[(i, a)] for i in both)
        p = binom_two_sided(a_only, a_only + b_only)
        print(f"    {a:7s} vs {b:7s}: {a_only:2d} items right only as {a}, "
              f"{b_only:2d} only as {b}, p = {p:.3f}")

    # By critical position
    print("\n  By critical position:")
    print("    pos  " + "  ".join(f"{m:>8s}" for m in MATERIALS))
    for p in sorted({r["crit_pos"] for r in main}):
        cells = []
        for m in MATERIALS:
            R = [r for r in main if r["crit_pos"] == p and r["material"] == m]
            cells.append(f"{sum(r['correct'] for r in R)}/{len(R)}")
        print(f"    {p:>3s}  " + "  ".join(f"{c:>8s}" for c in cells))

    # By measured duration: the main task requests one number of frames, but a
    # late flip shows the stimulus for longer. Trials are binned to the nearest
    # whole frame of their measured stim_dur_ms.
    bins = defaultdict(list)
    for r in main:
        bins[measured_frames(r, frame_ms)].append(r)
    print("\n  By measured duration (stim_dur_ms, binned to the nearest frame):")
    print(f"    {'frames':>6s}  {'ms (min–max)':>15s}  {'n':>3s}  " +
          "  ".join(f"{m:>8s}" for m in MATERIALS))
    for f in sorted(bins):
        R = bins[f]
        durs = [r["dur"] for r in R]
        cells = []
        for m in MATERIALS:
            Rm = [r for r in R if r["material"] == m]
            cells.append(f"{sum(r['correct'] for r in Rm)}/{len(Rm)}")
        tag = "  (requested)" if f == main_frames else ""
        print(f"    {f:>6d}  {min(durs):6.1f}–{max(durs):6.1f}   {len(R):>3d}  " +
              "  ".join(f"{c:>8s}" for c in cells) + tag)

    # Response bias
    up =sum(r["response"] == "up" for r in main)
    top = [r for r in main if r["top_letter"] == r["target"]]
    bottom = [r for r in main if r["top_letter"] != r["target"]]
    print(f"\n  Response bias: 'up' on {up}/{len(main)} trials "
          f"(p = {binom_two_sided(up, len(main)):.3f} vs 50%)")
    print(f"    accuracy when target on top:    {pct(sum(r['correct'] for r in top), len(top))}")
    print(f"    accuracy when target at bottom: {pct(sum(r['correct'] for r in bottom), len(bottom))}")

    # Halves
    ordered = sorted(main, key=lambda r: r["trial"])
    half = len(ordered) // 2
    print("\n  First vs second half:")
    for name, part in [("first", ordered[:half]), ("second", ordered[half:])]:
        cells = []
        for m in MATERIALS:
            R = [r for r in part if r["material"] == m]
            cells.append(f"{m} {sum(r['correct'] for r in R)}/{len(R)}")
        print(f"    {name:6s}: " + ", ".join(cells))

    # Calibration check
    if stair:
        at = [r for r in stair if r["frames"] == main_frames]
        k, n = acc["letter"]
        print(f"\n  Calibration: staircase at {main_frames} frames "
              f"{pct(sum(r['correct'] for r in at), len(at))}; "
              f"main-task letters {pct(k, n)}")

    # RT outliers
    fast = [r for r in rows if r["rt"] < 200]
    slow = [r for r in rows if r["rt"] > 5000]
    if fast or slow:
        print(f"\n  RT < 200 ms: {len(fast)} trials; RT > 5 s: {len(slow)} trials")

    if list_errors:
        print("\n  Main-task errors:")
        for r in ordered:
            if not r["correct"]:
                print(f"    trial {r['trial']:3d} {r['material']:7s} {r['item']:5s} "
                      f"shown {r['displayed']}  {r['target']} vs {r['foil']}, chose {r['chosen']}")

    return {"file": path.name, "subject": subject, "language": info.get("language", "?"),
            "frames": main_frames, "ms": statistics.median(r["dur"] for r in main),
            "acc": acc}


def group_summary(results):
    print("\n" + "=" * 78)
    print(f"Group summary ({len(results)} files)")
    print("=" * 78)
    print(f"  {'subject':>7s}  {'lang':4s}  {'frames':>6s}  {'ms':>6s}  " +
          "  ".join(f"{m:>8s}" for m in MATERIALS))
    rates = {m: [] for m in MATERIALS}
    for res in results:
        cells = []
        for m in MATERIALS:
            k, n = res["acc"][m]
            rates[m].append(k / n)
            cells.append(f"{100 * k / n:7.1f}%")
        print(f"  {res['subject']:>7s}  {res['language']:4s}  {res['frames']:>6d}  "
              f"{res['ms']:6.1f}  " + "  ".join(cells))
    print(f"  {'mean':>7s}  {'':4s}  {'':>6s}  {'':>6s}  " +
          "  ".join(f"{100 * statistics.mean(rates[m]):7.1f}%" for m in MATERIALS))

    print("\n  Exact sign test across files (ties dropped):")
    for a, b in PAIRS:
        diffs = [x - y for x, y in zip(rates[a], rates[b])]
        pos, neg = sum(d > 0 for d in diffs), sum(d < 0 for d in diffs)
        print(f"    {a:7s} > {b:7s} in {pos} files, < in {neg}, "
              f"p = {binom_two_sided(pos, pos + neg):.3f}")


def main(argv):
    list_errors = "--errors" in argv
    paths = [pathlib.Path(a) for a in argv if a != "--errors"]
    if not paths:
        print(__doc__)
        return 2
    results = []
    for p in paths:
        if not p.exists():
            print(f"{p}: no such file", file=sys.stderr)
            return 1
        res = analyse(p, list_errors)
        if res:
            results.append(res)
    if len(results) > 1:
        group_summary(results)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
