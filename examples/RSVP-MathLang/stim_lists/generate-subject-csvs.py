"""Generate the per-subject stimulus lists driving the MathLang RSVP paradigm.

Each subject gets one CSV per the whole experiment, holding every sentence of
sentences.tsv exactly once, with three things randomised per subject:

  * which sentence fills each (cond, truth_val) slot;
  * which run (bloc) a sentence belongs to, and its order within that run;
  * the onset schedule, drawn by permuting the inter-sentence intervals in
    SOA.csv.

Conditions are dealt round-robin so every run holds the same number of
sentences of each condition.

Usage:
    python generate-subject-csvs.py                 # 101 subjects, into .
    python generate-subject-csvs.py --subjects 5    # just sub-000 .. sub-004
    python generate-subject-csvs.py --only 42       # regenerate sub-042 alone

(c) Bosco Taddei & Christophe Pallier 2023
"""

import argparse
from pathlib import Path

import numpy as np
import pandas as pd

N_RUNS = 5
START_DELAY_MS = 6000
N_SUBJECTS = 101
SEED = 0


def generate_stim_list(sentences, soas, n_runs, start_delay, rng):
    """Return one subject's stimulus table.

    `sentences` is never modified; a shuffled copy is returned, with the
    columns bloc, order and onset added.
    """
    rows_per_run, remainder = divmod(len(sentences), n_runs)
    if remainder:
        raise ValueError(
            f"{len(sentences)} sentences do not divide evenly into {n_runs} runs"
        )
    if len(soas) != rows_per_run:
        raise ValueError(
            f"{len(soas)} SOAs cannot schedule {rows_per_run} sentences per run"
        )

    df = sentences.copy()

    # Shuffle which sentence fills each slot, keeping every sentence inside its
    # own (cond, truth_val) cell so the design stays balanced.
    df["sentence"] = df.groupby(["cond", "truth_val"], sort=False)["sentence"].transform(
        rng.permutation
    )

    # Deal each condition round-robin across the runs. Doing it per condition
    # (rather than over the row index) keeps the runs balanced whatever order
    # sentences.tsv happens to be in.
    df["bloc"] = df.groupby("cond", sort=False).cumcount() % n_runs + 1

    # Random presentation order within each run.
    df["order"] = df.groupby("bloc", sort=False).cumcount()
    df["order"] = df.groupby("bloc", sort=False)["order"].transform(rng.permutation)
    df = df.sort_values(["bloc", "order"], ignore_index=True)

    # Onsets: the SOA is the gap to the *next* sentence, so the onsets of a run
    # are the running total of the gaps preceding each sentence, after the
    # start delay. The last SOA of a run only pads its tail.
    onsets = [
        start_delay + np.cumsum(np.concatenate([[0], rng.permutation(soas)[:-1]]))
        for _ in range(n_runs)
    ]
    df["onset"] = np.concatenate(onsets)

    return df


def main():
    here = Path(__file__).parent
    p = argparse.ArgumentParser(description=__doc__,
                                formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--sentences", type=Path, default=here / "sentences.tsv")
    p.add_argument("--soa", type=Path, default=here / "SOA.csv")
    p.add_argument("--outdir", type=Path, default=here)
    p.add_argument("--subjects", type=int, default=N_SUBJECTS,
                   help=f"number of subjects to generate (default {N_SUBJECTS})")
    p.add_argument("--only", type=int, metavar="ID",
                   help="regenerate this subject alone (reproduces its batch file)")
    p.add_argument("--runs", type=int, default=N_RUNS)
    p.add_argument("--start-delay", type=int, default=START_DELAY_MS,
                   help=f"delay from trigger to first sentence, ms (default {START_DELAY_MS})")
    p.add_argument("--seed", type=int, default=SEED)
    args = p.parse_args()

    sentences = pd.read_csv(args.sentences, sep="\t")
    soas = np.loadtxt(args.soa, delimiter=",", dtype=int)
    args.outdir.mkdir(parents=True, exist_ok=True)

    subject_ids = [args.only] if args.only is not None else range(args.subjects)
    for subject_id in subject_ids:
        # Seeding per subject makes each list reproducible on its own, rather
        # than only as part of a full batch run.
        rng = np.random.default_rng([args.seed, subject_id])
        df = generate_stim_list(sentences, soas, args.runs, args.start_delay, rng)
        out = args.outdir / f"sub-{subject_id:03}_task-MathLang.csv"
        df.to_csv(out, index=False)
    print(f"wrote {len(list(subject_ids))} stimulus list(s) to {args.outdir}")


if __name__ == "__main__":
    main()
