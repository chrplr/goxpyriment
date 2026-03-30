#!/usr/bin/env python3
"""
analyze.py  —  Reproduce the latency-profile figures of Povel & Collard (1982)
from one or more .csv data files produced by the Finger-Tapping experiment.

Usage
-----
    python analyze.py ~/goxpy_data/Patterned_Finger_Tapping_*.csv

When called with multiple files each file is treated as one subject.

Output (written to the current directory)
-----------------------------------------
    fig04_mean_iti.pdf   — mean ITI per pattern              (≈ paper Fig. 4)
    fig_setA.pdf         — latency profiles for set A         (≈ paper Fig. 5)
    fig_setB.pdf         — latency profiles for set B         (≈ paper Fig. 7)
    fig_setC.pdf         — latency profiles for set C         (≈ paper Fig. 8)
    fig_setD.pdf         — latency profiles for set D         (≈ paper Fig. 9)

Analysis follows the paper exactly:
  1. Exclude rep 1 (as in the paper: "all first realizations were disregarded").
  2. For each subject × pattern, average iti_ms over reps 2–6 to get one ITI
     per tap position.
  3. Rank-transform within each subject × pattern (smallest = 1, largest = N).
  4. Average ranks across subjects to obtain the "mean rank" profile.
  5. For the finger-transitions-aligned panel, cyclically shift each pattern's
     profile so that identical finger transitions are at the same x-position.
  6. Report Kendall's W (inter-subject agreement) per pattern.

Requirements:  numpy  pandas  matplotlib  scipy
    pip install numpy pandas matplotlib scipy
"""

import sys
import glob
import pathlib
import warnings

import numpy as np
import pandas as pd
import matplotlib
import matplotlib.pyplot as plt
from scipy.stats import rankdata

matplotlib.rcParams.update({
    "font.family": "serif",
    "axes.spines.top": False,
    "axes.spines.right": False,
    "figure.dpi": 150,
})

# ─────────────────────────────────────────────────────────────────────────────
# Pattern definitions  (Povel & Collard 1982, Table 1)
# 1 = index, 2 = middle, 3 = ring, 4 = little
# ─────────────────────────────────────────────────────────────────────────────

PATTERNS = {
    "A1": [3, 2, 1, 2, 3, 4],
    "A2": [2, 1, 2, 3, 4, 3],
    "A3": [1, 2, 3, 4, 3, 2],
    "B1": [1, 2, 3, 2, 3, 4],
    "B2": [2, 3, 4, 1, 2, 3],
    "B3": [2, 3, 2, 3, 4, 1],
    "C1": [1, 2, 3, 3, 2, 1],
    "C2": [3, 3, 2, 1, 1, 2],
    "C3": [2, 3, 3, 2, 1, 1],
    "D1": [2, 4, 3, 4, 2, 1],
    "D2": [1, 2, 4, 3, 4, 2],
    "D3": [3, 4, 2, 1, 2, 4],
}

SETS = {
    "A": ["A1", "A2", "A3"],
    "B": ["B1", "B2", "B3"],
    "C": ["C1", "C2", "C3"],
    "D": ["D1", "D2", "D3"],
}

# ─────────────────────────────────────────────────────────────────────────────
# I/O helpers
# ─────────────────────────────────────────────────────────────────────────────

def load_xpd(paths):
    """
    Load one or more .csv files (CSV with # comment lines) into a single
    DataFrame.  Each file is assigned a numeric subject ID (1, 2, …).
    """
    frames = []
    for sid, path in enumerate(paths, start=1):
        df = pd.read_csv(path, comment="#")
        df.columns = df.columns.str.strip()
        df["subject"] = sid
        frames.append(df)
    return pd.concat(frames, ignore_index=True)


# ─────────────────────────────────────────────────────────────────────────────
# Core analysis
# ─────────────────────────────────────────────────────────────────────────────

def mean_rank_profile(df, pattern_name):
    """
    Return the mean-rank ITI profile (array of length n_taps) for one pattern,
    following Povel & Collard (1982):
      1. Average iti_ms over reps 2–6 per subject × tap position.
      2. Rank within each subject's profile (smallest=1, largest=N).
      3. Average ranks across subjects.
    """
    n_taps = len(PATTERNS[pattern_name])
    pat = df[
        (df["pattern"] == pattern_name)
        & (df["phase"] == "experiment")
        & (df["rep"] >= 2)
    ].copy()

    if pat.empty:
        return np.full(n_taps, np.nan)

    # Step 1: mean ITI per subject × tap
    grp = pat.groupby(["subject", "tap"])["iti_ms"].mean().reset_index()

    rank_profiles = []
    for _, sdata in grp.groupby("subject"):
        sdata = sdata.sort_values("tap")
        if len(sdata) < n_taps:
            continue
        itis = sdata["iti_ms"].values[:n_taps]
        rank_profiles.append(rankdata(itis))  # smallest=1, largest=N

    if not rank_profiles:
        return np.full(n_taps, np.nan)
    return np.mean(rank_profiles, axis=0)


def cyclic_shift(seq, k):
    """Left-rotate list seq by k positions."""
    n = len(seq)
    k = k % n
    return seq[k:] + seq[:k]


def find_cyclic_shift(ref_seq, target_seq):
    """
    Return k ≥ 0 such that cyclic_shift(ref_seq, k) == target_seq.
    Returns 0 if not a cyclic permutation (shouldn't happen for valid data).
    """
    n = len(ref_seq)
    for k in range(n):
        if cyclic_shift(ref_seq, k) == target_seq:
            return k
    return 0


def kendall_W(df, pattern_name):
    """
    Kendall's coefficient of concordance W for the rank profiles of all
    subjects on this pattern.  W = 1 means perfect agreement.
    Requires ≥ 2 subjects; returns NaN otherwise.
    """
    n_taps = len(PATTERNS[pattern_name])
    pat = df[
        (df["pattern"] == pattern_name)
        & (df["phase"] == "experiment")
        & (df["rep"] >= 2)
    ]
    grp = pat.groupby(["subject", "tap"])["iti_ms"].mean().reset_index()

    rank_matrix = []
    for _, sdata in grp.groupby("subject"):
        sdata = sdata.sort_values("tap")
        if len(sdata) == n_taps:
            rank_matrix.append(rankdata(sdata["iti_ms"].values))

    if len(rank_matrix) < 2:
        return np.nan

    R = np.array(rank_matrix)           # shape: (n_subjects, n_taps)
    m, k = R.shape
    S = np.sum((R.sum(axis=0) - m * (k + 1) / 2) ** 2)
    W = 12 * S / (m ** 2 * (k ** 3 - k))
    return W


# ─────────────────────────────────────────────────────────────────────────────
# Figures
# ─────────────────────────────────────────────────────────────────────────────

def plot_fig4(df, out_path="fig04_mean_iti.pdf"):
    """
    Mean ITI (ms) per pattern — equivalent to paper Fig. 4.
    Dashed vertical lines separate stimulus sets.
    """
    pattern_names = list(PATTERNS.keys())
    means = []
    for name in pattern_names:
        sub = df[
            (df["pattern"] == name)
            & (df["phase"] == "experiment")
            & (df["rep"] >= 2)
        ]
        means.append(sub["iti_ms"].mean() if not sub.empty else np.nan)

    fig, ax = plt.subplots(figsize=(7, 4))
    ax.plot(range(len(pattern_names)), means, "o-", color="black",
            linewidth=1.2, markersize=5)
    for boundary in [2.5, 5.5, 8.5]:           # between sets A/B, B/C, C/D
        ax.axvline(boundary, color="black", linestyle="--", linewidth=0.8)
    ax.set_xticks(range(len(pattern_names)))
    ax.set_xticklabels(pattern_names, fontsize=9)
    ax.set_xlabel("STIMULUS NUMBER", labelpad=6)
    ax.set_ylabel("MEAN LATENCY (ms)", labelpad=6)
    ax.set_title("Mean inter-tap interval per pattern  (cf. Fig. 4)", fontsize=10)
    fig.tight_layout()
    fig.savefig(out_path)
    print(f"  {out_path}")
    plt.close(fig)


def plot_set(df, set_name, out_path=None):
    """
    Two-panel latency profile figure for one stimulus set:
      Left  — finger-transitions aligned (cyclic shift to reference sequence)
      Right — codes / element-number aligned (raw positions 1–N)

    Equivalent to paper Figs. 5 / 7 / 8 / 9.
    """
    pat_names = SETS[set_name]
    ref_seq   = PATTERNS[pat_names[0]]
    n         = len(ref_seq)

    # Compute shifts and profiles
    profiles = {name: mean_rank_profile(df, name) for name in pat_names}
    shifts   = {name: find_cyclic_shift(ref_seq, PATTERNS[name]) for name in pat_names}

    # Warn if any profile is all-NaN (pattern not in data yet)
    missing = [name for name in pat_names if np.all(np.isnan(profiles[name]))]
    if missing:
        print(f"    WARNING: no data for {missing} — those lines will be blank.")

    styles  = ["-",  "--", "-."]
    markers = ["o",  "s",  "^"]
    x       = np.arange(1, n + 1)

    fig, axes = plt.subplots(1, 2, figsize=(10, 4), sharey=True)

    # ── Left panel: finger-transitions aligned ────────────────────────────────
    ax = axes[0]
    for i, name in enumerate(pat_names):
        k       = shifts[name]
        aligned = np.roll(profiles[name], k)   # shift right by k → aligns to ref
        ax.plot(x, aligned, styles[i], marker=markers[i], color="black",
                linewidth=1.2, markersize=5, label=name)
    ax.set_xlabel("FINGER NUMBER", labelpad=6)
    ax.set_ylabel("LATENCY  (mean rank)", labelpad=6)
    ax.set_xticks(x)
    ax.set_xticklabels(ref_seq)
    ax.set_ylim(0.5, n + 0.5)
    ax.set_yticks(range(1, n + 1))
    ax.legend(fontsize=8, frameon=False)
    ax.set_title("FINGER TRANSITIONS ALIGNED", fontsize=9, pad=8)

    # ── Right panel: codes / element-number aligned ───────────────────────────
    ax = axes[1]
    for i, name in enumerate(pat_names):
        ax.plot(x, profiles[name], styles[i], marker=markers[i], color="black",
                linewidth=1.2, markersize=5, label=name)
    ax.set_xlabel("ELEMENT NUMBER", labelpad=6)
    ax.set_xticks(x)
    ax.set_xticklabels(x)
    ax.set_ylim(0.5, n + 0.5)
    ax.set_yticks(range(1, n + 1))
    ax.legend(fontsize=8, frameon=False)
    ax.set_title("CODES ALIGNED", fontsize=9, pad=8)

    fig.suptitle(
        f"Stimulus set {set_name}  —  patterns: "
        + ",  ".join(f"{n} = {PATTERNS[n]}" for n in pat_names),
        fontsize=9, y=1.02,
    )
    fig.tight_layout()

    if out_path is None:
        out_path = f"fig_set{set_name}.pdf"
    fig.savefig(out_path, bbox_inches="tight")
    print(f"  {out_path}")
    plt.close(fig)


# ─────────────────────────────────────────────────────────────────────────────
# Summary statistics
# ─────────────────────────────────────────────────────────────────────────────

def print_summary(df):
    """Print per-pattern summary statistics to stdout."""
    n_subjects = df["subject"].nunique()
    print(f"\n{'─'*62}")
    print(f"  Subjects: {n_subjects}")
    total_trials = (
        df[df["phase"] == "experiment"]
        .drop_duplicates(subset=["subject", "pattern", "rep"])
        .shape[0]
    )
    print(f"  Experiment trials (rows in data): {total_trials}")
    print(f"\n  Per-pattern statistics  (reps 2–6, experiment phase):\n")
    print(f"  {'Pattern':<8}  {'N taps':>7}  {'Mean ITI (ms)':>14}  {'SD':>8}  {'W':>6}")
    print(f"  {'─'*8}  {'─'*7}  {'─'*14}  {'─'*8}  {'─'*6}")
    for name in PATTERNS:
        sub = df[
            (df["pattern"] == name)
            & (df["phase"] == "experiment")
            & (df["rep"] >= 2)
        ]
        n    = len(sub)
        m    = sub["iti_ms"].mean()
        s    = sub["iti_ms"].std()
        W    = kendall_W(df, name)
        Wstr = f"{W:.2f}" if not np.isnan(W) else "  N/A"
        mstr = f"{m:.1f}" if not np.isnan(m) else "   N/A"
        sstr = f"{s:.1f}" if not np.isnan(s) else "   N/A"
        print(f"  {name:<8}  {n:>7}  {mstr:>14}  {sstr:>8}  {Wstr:>6}")
    print(f"{'─'*62}\n")


# ─────────────────────────────────────────────────────────────────────────────
# Entry point
# ─────────────────────────────────────────────────────────────────────────────

def main():
    warnings.filterwarnings("ignore")

    # Resolve file paths
    if len(sys.argv) > 1:
        paths = []
        for arg in sys.argv[1:]:
            expanded = glob.glob(arg)
            paths.extend(expanded if expanded else [arg])
    else:
        # Auto-detect in ~/goxpy_data/
        data_dir = pathlib.Path.home() / "goxpy_data"
        paths = sorted(data_dir.glob("Patterned_Finger_Tapping_*.csv"))
        if not paths:
            print(
                "No XPD files found.  Pass the file path(s) explicitly:\n"
                "    python analyze.py ~/goxpy_data/Patterned_Finger_Tapping_*.csv"
            )
            sys.exit(1)
        paths = [str(p) for p in paths]

    print(f"Loading {len(paths)} file(s):")
    for p in paths:
        print(f"  {p}")

    df = load_xpd(paths)

    # Basic sanity check
    required = {"pattern", "phase", "rep", "tap", "iti_ms"}
    missing = required - set(df.columns)
    if missing:
        print(f"ERROR: missing columns in data: {missing}")
        sys.exit(1)

    print_summary(df)

    print("Generating figures:")
    plot_fig4(df)
    for s in ["A", "B", "C", "D"]:
        plot_set(df, s)

    print("\nDone.  Open the PDF files to see the figures.")


if __name__ == "__main__":
    main()
