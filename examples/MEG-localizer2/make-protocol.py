#! /usr/bin/env python3
# Time-stamp: <2026-09-15 christophe@pallier.org>
"""Write a protocol table for the finger/tone MEG localizer.

    ./make-protocol.py                        # protocols/demo.tsv
    ./make-protocol.py --seed 7 --name run7   # a different order

The run is a single stream of trials, one stimulus per trial:

  * 5 pictures of a hand with one finger coloured red (f1.jpg = thumb ...
    f5.jpg = little finger), shown for IMAGE_MS;
  * 5 narrow-band noise tones one octave apart (200 ... 3200 Hz), 500 ms
    files played whole.

Each of the 10 stimuli appears REPEATS times, so the run holds 10 * REPEATS
trials. The order is randomised in rounds: every round is a permutation of the
10 stimuli, so the counts stay balanced across the run, and a round may not
start with the stimulus the previous one ended with, so a stimulus never
follows itself.

Successive onsets are SOA_MS apart, plus or minus a uniform jitter of up to
JITTER_MS drawn afresh for each trial -- so the participant cannot anticipate
the onset and the response to one stimulus is not phase-locked to the next.
The jitter is on the SOA, not on a fixed grid, so the run length varies by a
few hundred ms from one seed to the next.

Every row carries a TTL code, sent at the row's onset when the program is run
with -ttl: 1-5 for the fingers (thumb to little finger), 6-10 for the tones
(low to high).

The stimuli are the files already in stimuli/, which is what main.go embeds;
this script only writes the schedule.
"""

import argparse
import random
from pathlib import Path

HERE = Path(__file__).resolve().parent
STIM = HERE / "stimuli"
PROTOCOLS = HERE / "protocols"

REPEATS = 16              # presentations of each stimulus
SOA_MS = 1000             # nominal onset-to-onset interval
JITTER_MS = 100           # SOA is drawn uniformly in [SOA_MS - JITTER_MS, SOA_MS + JITTER_MS]
FIRST_ONSET_MS = 1000     # fixation before the first stimulus
IMAGE_MS = 500            # pictures on screen for as long as the tones last

# One row per stimulus: (type, cond, file, code). The cond labels are what the
# data file and the MEG epochs are sorted by; the file names are what the
# program loads; the code is the TTL value sent at the onset -- 1-5 fingers,
# 6-10 tones, in the order listed.
FINGERS = ("thumb", "index", "middle", "ring", "little")
TONES_HZ = (200, 400, 800, 1600, 3200)
STIMULI = [("IMAGE", f"finger_{name}", f"f{i}.jpg", i)
           for i, name in enumerate(FINGERS, 1)]
STIMULI += [("SOUND", f"tone_{hz}", f"tone_{hz:05d}Hz.wav", 6 + i)
            for i, hz in enumerate(TONES_HZ)]


def sound_ms(path):
    """Duration of a WAV file in ms."""
    import wave
    with wave.open(str(path)) as w:
        return round(1000 * w.getnframes() / w.getframerate())


def order(rng):
    """The run's stimulus sequence: REPEATS rounds, each a permutation of the
    10 stimuli, with no stimulus repeated across a round boundary."""
    seq = []
    for _ in range(REPEATS):
        rnd = STIMULI[:]
        rng.shuffle(rnd)
        while seq and rnd[0] == seq[-1]:
            rng.shuffle(rnd)
        seq.extend(rnd)
    return seq


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--seed", type=int, default=0, help="random seed (default 0)")
    ap.add_argument("--name", default="demo",
                    help="protocol name, written to protocols/NAME.tsv (default demo)")
    args = ap.parse_args()

    for _, _, f, _ in STIMULI:
        if not (STIM / f).exists():
            raise SystemExit(f"missing stimulus file stimuli/{f}")
    durations = {f: (IMAGE_MS if t == "IMAGE" else sound_ms(STIM / f))
                 for t, _, f, _ in STIMULI}

    rng = random.Random(args.seed)
    seq = order(rng)

    PROTOCOLS.mkdir(exist_ok=True)
    out = PROTOCOLS / f"{args.name}.tsv"
    onset = FIRST_ONSET_MS
    with open(out, "w") as fh:
        fh.write("onset_time\tduration\ttype\tcond\tstimuli\tcode\n")
        for stype, cond, f, code in seq:
            fh.write(f"{onset}\t{durations[f]}\t{stype}\t{cond}\t{f}\t{code}\n")
            last = onset
            onset += SOA_MS + rng.randint(-JITTER_MS, JITTER_MS)
    print(f"{out}: {len(seq)} trials, seed {args.seed}, "
          f"last onset at {last / 1000:.1f} s")


if __name__ == "__main__":
    main()
