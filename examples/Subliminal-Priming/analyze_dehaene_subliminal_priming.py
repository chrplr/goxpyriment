#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Distributed under the GNU General Public License v3.

"""Descriptive statistics for the Dehaene Subliminal Priming experiment.

Reads one or more .csv result files (saved by default in ~/goxpy_data/) and
prints, for every word_duration_ms level:

  • hit rate        — P("seen" | word present)   [visible_word + masked_word]
  • false alarm rate— P("seen" | word absent)    [visible_blank + masked_blank]
  • mean RT         — average response time in ms (all trials)
  • mean / min / max estimated_word_duration_ms  — actual measured target duration
  • d'  (sensitivity) and c (bias) from signal detection theory

Both an overall SDT table and a per-condition breakdown are printed.

Usage:
    python3 analyze_dehaene_subliminal_priming.py [FILE ...]

    If no FILE is given the script searches ~/goxpy_data/ for all
    Dehaene-Subliminal-Priming_*.csv files automatically.
"""

import argparse
import csv
import glob
import math
import os
import sys
from collections import defaultdict


# ── XPD reader ────────────────────────────────────────────────────────────────

def read_xpd(path):
    """Return list of dicts for every data row in an .csv file.

    Comment lines (starting with '#') and the CSV header are handled
    automatically; the header row becomes the dict keys.
    """
    with open(path, newline="", encoding="utf-8") as fh:
        non_comment = (line for line in fh if not line.startswith("#"))
        reader = csv.DictReader(non_comment)
        return list(reader)


# ── Statistics helpers ─────────────────────────────────────────────────────────

def _float(s):
    try:
        return float(s)
    except (ValueError, TypeError):
        return None


def mean(values):
    values = [v for v in values if v is not None]
    return sum(values) / len(values) if values else float("nan")


def _min(values):
    values = [v for v in values if v is not None]
    return min(values) if values else float("nan")


def _max(values):
    values = [v for v in values if v is not None]
    return max(values) if values else float("nan")


def seen_rate(rows):
    """Proportion of rows where response == 'seen'."""
    if not rows:
        return float("nan")
    return sum(1 for r in rows if r["response"].strip() == "seen") / len(rows)


# ── Signal Detection Theory ────────────────────────────────────────────────────

def _probit(p):
    """Inverse normal CDF (probit).

    Uses math.erfinv when available (Python ≥ 3.12), otherwise falls back to
    Peter Acklam's rational approximation (max absolute error < 1.15e-9).
    """
    try:
        return math.sqrt(2) * math.erfinv(2 * p - 1)
    except AttributeError:
        pass
    # Acklam's algorithm — coefficients from
    # https://web.archive.org/web/20151030215612/http://home.online.no/~pjacklam/notes/invnorm/
    a = [-3.969683028665376e+01,  2.209460984245205e+02,
         -2.759285104469687e+02,  1.383577518672690e+02,
         -3.066479806614716e+01,  2.506628277459239e+00]
    b = [-5.447609879822406e+01,  1.615858368580409e+02,
         -1.556989798598866e+02,  6.680131188771972e+01,
         -1.328068155288572e+01]
    c = [-7.784894002430293e-03, -3.223964580411365e-01,
         -2.400758277161838e+00, -2.549732539343734e+00,
          4.374664141464968e+00,  2.938163982698783e+00]
    d = [ 7.784695709041462e-03,  3.224671290700398e-01,
          2.445134137142996e+00,  3.754408661907416e+00]
    p_low, p_high = 0.02425, 1 - 0.02425
    if p < p_low:
        q = math.sqrt(-2 * math.log(p))
        return (((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q+c[5]) / \
               ((((d[0]*q+d[1])*q+d[2])*q+d[3])*q+1)
    elif p <= p_high:
        q = p - 0.5
        r = q * q
        return (((((a[0]*r+a[1])*r+a[2])*r+a[3])*r+a[4])*r+a[5])*q / \
               (((((b[0]*r+b[1])*r+b[2])*r+b[3])*r+b[4])*r+1)
    else:
        q = math.sqrt(-2 * math.log(1 - p))
        return -(((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q+c[5]) / \
                ((((d[0]*q+d[1])*q+d[2])*q+d[3])*q+1)


def _correct_rate(rate, n):
    """Apply the standard 0.5/n correction to avoid ±inf in probit(0 or 1)."""
    if rate == 0.0:
        return 0.5 / n
    if rate == 1.0:
        return (n - 0.5) / n
    return rate


def sdt(hit_rate, fa_rate, n_signal, n_noise):
    """Return (d_prime, c) given hit and false-alarm rates.

    d' = Z(H) - Z(FA)
    c  = -0.5 * (Z(H) + Z(FA))   (criterion, positive = conservative)

    Returns (nan, nan) if either rate is undefined.
    """
    if math.isnan(hit_rate) or math.isnan(fa_rate):
        return float("nan"), float("nan")
    zh = _probit(_correct_rate(hit_rate, n_signal))
    zf = _probit(_correct_rate(fa_rate,  n_noise))
    return zh - zf, -0.5 * (zh + zf)


# ── Trial classification ───────────────────────────────────────────────────────

SIGNAL_CONDITIONS = {"visible_word", "masked_word"}
NOISE_CONDITIONS  = {"visible_blank", "masked_blank"}


def is_signal(row):
    return row["condition"].strip() in SIGNAL_CONDITIONS


def is_noise(row):
    return row["condition"].strip() in NOISE_CONDITIONS


# ── Table printer ─────────────────────────────────────────────────────────────

def print_table(title, headers, rows):
    col_w = [max(len(h), max((len(str(r[i])) for r in rows), default=0))
             for i, h in enumerate(headers)]
    sep = "  ".join("-" * w for w in col_w)
    fmt = "  ".join(f"{{:<{w}}}" for w in col_w)

    print(f"\n{title}")
    print("=" * len(title))
    print(fmt.format(*headers))
    print(sep)
    for row in rows:
        print(fmt.format(*[str(v) for v in row]))
    print()


# ── Main ──────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description="Descriptive statistics for Dehaene Subliminal Priming .csv files."
    )
    parser.add_argument(
        "files", nargs="*", metavar="FILE",
        help=".csv result file(s). Defaults to ~/goxpy_data/Dehaene-Subliminal-Priming_*.csv",
    )
    args = parser.parse_args()

    paths = args.files
    if not paths:
        pattern = os.path.expanduser("~/goxpy_data/Dehaene-Subliminal-Priming_*.csv")
        paths = sorted(glob.glob(pattern))
        if not paths:
            sys.exit(
                "No .csv files found in ~/goxpy_data/. "
                "Pass file path(s) explicitly or run the experiment first."
            )

    all_rows = []
    for path in paths:
        rows = read_xpd(path)
        print(f"Loaded {len(rows):4d} trial(s) from {path}")
        all_rows.extend(rows)

    if not all_rows:
        sys.exit("No data rows found.")

    print(f"\nTotal trials: {len(all_rows)}")

    durations = sorted({r["word_duration_ms"].strip() for r in all_rows},
                       key=lambda x: float(x))

    # ── Table 1: SDT + timing by word_duration_ms ─────────────────────────────
    sdt_rows = []
    for dur in durations:
        dur_rows  = [r for r in all_rows if r["word_duration_ms"].strip() == dur]
        sig_rows  = [r for r in dur_rows if is_signal(r)]
        noi_rows  = [r for r in dur_rows if is_noise(r)]

        hr  = seen_rate(sig_rows)
        far = seen_rate(noi_rows)
        rt  = mean([_float(r["rt_ms"]) for r in dur_rows])
        dp, c = sdt(hr, far, len(sig_rows), len(noi_rows))

        est_vals = [_float(r["estimated_word_duration_ms"]) for r in dur_rows]
        sdt_rows.append((
            dur,
            f"{len(sig_rows)}",
            f"{len(noi_rows)}",
            f"{hr:.2%}" if not math.isnan(hr)  else "n/a",
            f"{far:.2%}" if not math.isnan(far) else "n/a",
            f"{rt:.1f}",
            f"{mean(est_vals):.2f}",
            f"{_min(est_vals):.2f}",
            f"{_max(est_vals):.2f}",
            f"{dp:+.3f}" if not math.isnan(dp) else "n/a",
            f"{c:+.3f}"  if not math.isnan(c)  else "n/a",
        ))

    print_table(
        "SDT + timing by word_duration_ms  (signal = word conditions, noise = blank conditions)",
        ["dur_ms", "n_sig", "n_noi", "hit_rate", "fa_rate",
         "mean_rt_ms", "mean_est_ms", "min_est_ms", "max_est_ms", "d'", "c"],
        sdt_rows,
    )

    # ── Table 2: hit/FA rate breakdown by condition × word_duration_ms ────────
    conditions = sorted({r["condition"].strip() for r in all_rows})
    detail_rows = []
    for cond in conditions:
        for dur in durations:
            rows = [r for r in all_rows
                    if r["condition"].strip() == cond
                    and r["word_duration_ms"].strip() == dur]
            if not rows:
                continue
            rt = mean([_float(r["rt_ms"]) for r in rows])
            detail_rows.append((
                cond,
                dur,
                f"{len(rows)}",
                f"{seen_rate(rows):.2%}",
                f"{rt:.1f}",
            ))

    print_table(
        "Seen rate and RT by condition × word_duration_ms",
        ["condition", "dur_ms", "n", "seen_rate", "mean_rt_ms"],
        detail_rows,
    )

    print("Note: d' > 0 = above-chance detection; c > 0 = conservative (bias toward 'unseen').")
    print("      Extreme rates (0 % or 100 %) are corrected with the 0.5/n rule.\n")


if __name__ == "__main__":
    main()
