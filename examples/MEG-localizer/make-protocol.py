#! /usr/bin/env python
# Time-stamp: <2026-08-20 christophe@pallier.org>
"""Build stimuli/ and a protocol table for the MEG localizer demo.

    ./make-protocol.py                 # writes protocols/demo.tsv
    ./make-protocol.py --blocks 60 --seed 7

Every block lasts BLOCK_MS and carries one condition:

  image conditions   four items in RSVP -- ON_MS on screen, GAP_MS blank,
                     so an SOA of ON_MS + GAP_MS and 4 * SOA of stimulation,
                     then the rest of the block is fixation cross. PACING
                     overrides this per condition.
  sound conditions   one file, played whole; the clips are ~1.6 s and would
                     not fit four to a block.

Conditions are shuffled so the same one does not repeat back to back, and
items are drawn without replacement within a condition (reshuffling when the
pool runs out) so a block never shows the same item twice.

The stimulus files are copied out of assets/ into stimuli/, which is what
main.go embeds. Filenames are already unique across the categories.
"""

import argparse
import math
import random
import re
import shutil
import wave
from pathlib import Path

HERE = Path(__file__).resolve().parent
ASSETS = HERE / "assets"
STIM = HERE / "stimuli"
PROTOCOLS = HERE / "protocols"

BLOCK_MS = 2000
ON_MS = 350
GAP_MS = 50
PER_BLOCK = 4                     # image items per block, by default

# Per-condition pacing, overriding the defaults above: (items, on_ms, gap_ms).
# An equation has to be read and solved, which four-at-400-ms does not allow,
# so it gets the block to itself.
PACING = {
    "equations": (1, 1200, 0),
}

# condition -> (asset subdirectory, glob)
IMAGE_CONDITIONS = {
    "equations":  ("equations", "*.png"),
    "faces":      ("faces_kept", "*.png"),
    "houses":     ("houses_kept", "*.png"),
    "wedges":     ("wedges-rings", "wedge_*.png"),
    "rings":      ("wedges-rings", "ring_*.png"),
}
# The sentence condition shows the four words of one sentence in order, at the
# same pace as the other RSVP conditions. It replaces the earlier "words"
# condition, which drew four unrelated words: the images are the same files in
# assets/words/, but the order now carries a sentence rather than a word list.
# The sentences live in README.md, which is where they are documented.
SENTENCE_SOURCE = ("words", "*.png")

# The consonant control shows one line of the sentence-matched list: four
# strings that were generated to match a specific sentence in item count, item
# length and total characters. Drawing four unrelated strings from the whole
# pool would control for the word condition that no longer exists, not for the
# sentence condition that replaced it.
CONSONANT_SOURCE = ("consonant-strings", "*.png")

SOUND_CONDITIONS = {
    "sounds": ("natural_sounds", "sound_*.wav"),
}

# Tone sequences. Four tones per block at the same 400 ms SOA as the image
# conditions (the tones are 300 ms, so 100 ms of silence between them).
#
# Both conditions draw the same four tones and differ only in the order they
# are played: ordered (rising or falling) against shuffled. Holding the tone
# set constant is what makes the contrast about sequence structure rather
# than about which pitches happened to occur.
#
# The four are always an evenly spaced run of the scale, so a regular trial has
# a constant pitch step and not merely a rising order. The tones are spaced
# logarithmically (220 Hz to 440 Hz in nine steps, a ratio of 2**(1/8) each),
# so an even step in file index is an even step in pitch. With nine tones and
# four slots the step can be 1 or 2 -- a step of 3 would run off the top.
TONE_POOL = ("tones", "tone_*.wav")
TONE_CONDITIONS = ("tones_regular", "tones_random")
TONES_PER_BLOCK = 4
TONE_ON_MS = 300
TONE_GAP_MS = 100
TONE_STEPS = (1, 2)          # pitch steps that fit four tones in the scale

# Disk sequences, the visual counterpart of the tone sequences. Four disks per
# block at the ordinary RSVP pace, and the two conditions differ only in the
# order the positions are visited.
#
# The eight frames sit at 22.5 deg + k * 45 deg on one orbit, and sorted file
# order runs counterclockwise (disk_frame_1 = 22.5 deg ... disk_frame_8 =
# 337.5 deg). So a regular trial is a constant angular step from a random
# starting position, in one of the two directions -- the disk rotates left or
# right -- and, unlike the tone scale, the circle wraps, so no starting point
# is out of bounds.
#
# A random trial draws four distinct positions instead, rejecting the draws
# that happen to come out as a constant step (in either direction): those are
# regular trials wearing the wrong label. Distinct positions keep the rule that
# a block never shows the same item twice.
DISK_POOL = ("disks", "disk_frame_*.png")
DISK_CONDITIONS = ("disks_regular", "disks_random")
DISKS_PER_BLOCK = 4
DISK_STEP = 1                # 45 deg between successive disks in a regular trial

# The two motor conditions: an instruction to press a button three times,
# either read word by word or heard. They differ from every other condition in
# needing time for the response, and one of the recordings (press-left.wav,
# 2400 ms) is itself longer than a block -- so a motor trial takes two slots,
# and the remainder of the second slot is the response window.
REST = "rest"
REST_SOURCE = ("rest", "rest.png")
MOTOR_CONDITIONS = ("motor_visual", "motor_audio")
MOTOR_VISUAL = ("motor-visual", "*.png")
MOTOR_AUDIO = ("motor-audio-trimmed", "*.wav")

# Spoken equations, the auditory counterpart of the written ones. They run
# 1.7-2.1 s, so the longest does not fit a single block, and they have to be
# solved as well as heard -- two slots, like the motor trials.
EQUATIONS_AUDIO = ("equations-audio-trimmed", "equation_*.wav")

# Spoken sentences, the auditory counterpart of the written sentence condition.
# Only 10 of the 32 have been synthesised so far. Trimmed of the synthesiser's
# padding they run 1.02-1.78 s and fit a single block.
#
# The reversed versions are the low-level control: identical duration and
# identical long-term spectrum, no intelligible speech. Reversal is applied to
# the trimmed files, so a control and its sentence start together.
SENTENCES_AUDIO = ("spoken-sentences-trimmed", "sentence_*.wav")
SENTENCES_REVERSED = ("reversed-sentences", "control_*.wav")

# Rest: fixation cross only, but written as a row so the period is marked.
#
# A gap in the table would look the same to the participant, and did until now,
# but it leaves nothing in the data file to epoch against and nothing to hang a
# trigger on. So a rest trial shows rest.png, which is a picture of the same
# 40 px, 2 px-thick cross the engine draws in an ordinary gap: identical on
# screen, and it produces an onset and an offset like any other row.
REST = "rest"
REST_SOURCE = ("rest", "rest.png")

# Rest takes two slots. With one it was invisible: the two-slot spoken
# conditions already leave 1.6-2.4 s of fixation after their recording ends, so
# a 2 s rest added to the usual 400 ms tail produced a 2.4 s pause -- exactly
# what an ordinary motor_audio trial ends with. At two slots a rest is a 4.4 s
# pause, longer than any tail in the run and unmistakable.
# Trimming the padding put every recording under 2 s, so a condition needs a
# second slot only when the participant has something to do after hearing it:
# press a button, or solve a sum. Listening conditions now fit one block.
SLOTS = {"motor_visual": 2, "motor_audio": 2, "equations_audio": 2, REST: 2}


def collect():
    """Copy every stimulus into stimuli/ and return condition -> [filenames]."""
    if STIM.exists():
        shutil.rmtree(STIM)
    STIM.mkdir()
    pools = {}
    catalogue = {**IMAGE_CONDITIONS, **SOUND_CONDITIONS, "tones": TONE_POOL,
                 "disks": DISK_POOL,
                 "words": SENTENCE_SOURCE, "consonants": CONSONANT_SOURCE,
                 "motor_visual": MOTOR_VISUAL, "motor_audio": MOTOR_AUDIO,
                 "equations_audio": EQUATIONS_AUDIO,
                 "sentences_audio": SENTENCES_AUDIO,
                 "sentences_reversed": SENTENCES_REVERSED, REST: REST_SOURCE}
    for cond, (sub, pattern) in catalogue.items():
        files = sorted((ASSETS / sub).glob(pattern))
        if not files:
            raise SystemExit(f"condition {cond!r}: no files matching "
                             f"{sub}/{pattern}")
        for f in files:
            target = STIM / f.name
            # Conditions may legitimately share a stimulus file -- the word
            # "three" belongs to both the sentence list and the motor
            # instruction, and renders identically. What tells the conditions
            # apart is the label in the table, not the filename. So an
            # identical file is fine; only a genuine clash is an error.
            if target.exists():
                if target.read_bytes() != f.read_bytes():
                    raise SystemExit(
                        f"name collision in stimuli/: {f.name} differs between "
                        f"{cond!r} and an earlier condition")
            else:
                shutil.copy2(f, target)
        pools[cond] = [f.name for f in files]
    return pools


class Bag:
    """Draw without replacement, reshuffling when the pool is exhausted."""

    def __init__(self, items, rng):
        self.items, self.rng, self.rest = list(items), rng, []

    def take(self, n):
        out = []
        while len(out) < n:
            if not self.rest:
                self.rest = self.items[:]
                self.rng.shuffle(self.rest)
            out.append(self.rest.pop())
        return out


def constant_step(indices, n):
    """True if the positions step by a constant angle around the orbit.

    Both directions count, and so does a step of more than one position: what
    makes a trial regular is that the step never changes, not that it is small
    or that the angle increases.
    """
    steps = {(b - a) % n for a, b in zip(indices, indices[1:])}
    return len(steps) == 1


def order(conditions, blocks, rng):
    """A balanced block order that never repeats a condition back to back.

    Shuffled rounds, not independent draws: over a few dozen blocks, picking
    each condition at random gives badly uneven counts -- one run of 40 blocks
    gave `words` seven blocks and `disks` one. A round is one block of every
    condition, so counts can differ by at most one across the whole run.
    """
    seq = []
    while len(seq) < blocks:
        round_ = list(conditions)
        rng.shuffle(round_)
        if seq and round_[0] == seq[-1]:          # avoid a repeat at the seam
            round_.append(round_.pop(0))
        seq.extend(round_)
    return seq[:blocks]


def main():
    p = argparse.ArgumentParser(description=__doc__,
                                formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--blocks", type=int, default=90,
                   help="number of trials; motor and spoken-equation "
                        "trials take two slots, so the run lasts longer "
                        "than blocks x 2 s")
    p.add_argument("--seed", type=int, default=1)
    p.add_argument("--name", default="demo")
    args = p.parse_args()

    rng = random.Random(args.seed)
    pools = collect()
    bags = {c: Bag(v, rng) for c, v in pools.items()}
    # Tone files are sorted by name, and the names carry an index that runs
    # with frequency (tone_01_220Hz ... tone_09_440Hz), so file order is pitch
    # order and "ascending" is simply sorted order.
    tones = pools["tones"]
    # Disk files sort by the index in their name, and that index runs with
    # polar angle, so file order is position order around the orbit.
    disks = pools["disks"]
    conditions = (list(IMAGE_CONDITIONS) + list(SOUND_CONDITIONS)
                  + list(TONE_CONDITIONS) + list(DISK_CONDITIONS)
                  + list(MOTOR_CONDITIONS)
                  + ["sentences", "consonants", "equations_audio",
                     "sentences_audio", "sentences_reversed", REST])

    # The sentences and their matched consonant strings, taken from the README
    # sections that document them. Both are four items per line.
    lines = (HERE / "README.md").read_text().splitlines()
    heads = [(i, l.strip()) for i, l in enumerate(lines) if l.startswith("###")]

    def four_word_lines(title):
        for k, (i, h) in enumerate(heads):
            if h.startswith("### " + title):
                j = heads[k + 1][0] if k + 1 < len(heads) else len(lines)
                found = [l.strip() for l in lines[i + 1:j]
                         if re.fullmatch(r"([a-z]+ ){3}[a-z]+", l.strip())]
                if found:
                    return found
        raise SystemExit(f"no four-item lines found under {title!r} in README.md")

    sentence_lines = four_word_lines("Sentences")
    consonant_lines = four_word_lines("Consonant strings, matched to the sentences")
    sentence_bag = Bag(sentence_lines, rng)
    consonant_bag = Bag(consonant_lines, rng)

    # The instruction sentences, read from the folder that documents them, so
    # the word order comes from one source rather than being retyped here.
    sentences = [line.split() for line
                 in (ASSETS / "motor-visual" / "README.txt").read_text().splitlines()
                 if line.strip()]

    PROTOCOLS.mkdir(exist_ok=True)
    out = PROTOCOLS / f"{args.name}.tsv"
    soa = ON_MS + GAP_MS
    with open(out, "w") as fh:
        fh.write("onset_time\tduration\ttype\tcond\tstimuli\n")
        onset = 0
        for cond in order(conditions, args.blocks, rng):
            span = SLOTS.get(cond, 1) * BLOCK_MS
            if cond in ("sentences", "consonants"):
                bag = sentence_bag if cond == "sentences" else consonant_bag
                items = bag.take(1)[0].split()
                spec = "~".join(f"{w}.png:{ON_MS}:{GAP_MS}" for w in items)
                fh.write(f"{onset}\t{ON_MS}\tIMAGE_STREAM\t{cond}\t{spec}\n")
            elif cond == REST:
                fh.write(f"{onset}\t{span}\tIMAGE\t{cond}\t{pools[REST][0]}\n")
            elif cond in ("motor_audio", "equations_audio", "sentences_audio",
                          "sentences_reversed"):
                item = rng.choice(pools[cond])
                with wave.open(str(STIM / item)) as w:
                    ms = math.ceil(w.getnframes() / w.getframerate() * 1000)
                if ms > span:
                    raise SystemExit(f"{item} lasts {ms} ms, longer than the "
                                     f"{span} ms allowed for {cond!r}")
                fh.write(f"{onset}\t{ms}\tSOUND\t{cond}\t{item}\n")
            elif cond == "motor_visual":
                words = rng.choice(sentences)
                spec = "~".join(f"{w.strip('.,;:!?').lower()}.png:{ON_MS}:{GAP_MS}"
                                for w in words)
                fh.write(f"{onset}\t{ON_MS}\tIMAGE_STREAM\t{cond}\t{spec}\n")
            elif cond in TONE_CONDITIONS:
                step = rng.choice(TONE_STEPS)
                first = rng.randrange(len(tones) - step * (TONES_PER_BLOCK - 1))
                chosen = [tones[first + k * step] for k in range(TONES_PER_BLOCK)]
                if cond == "tones_regular":
                    chosen.sort(reverse=rng.random() < 0.5)
                else:
                    # A shuffle can come out ordered, which would make the
                    # trial a regular one wearing the wrong label.
                    ordered = sorted(chosen)
                    while chosen in (ordered, ordered[::-1]):
                        rng.shuffle(chosen)
                spec = "~".join(f"{n}:{TONE_ON_MS}:{TONE_GAP_MS}" for n in chosen)
                fh.write(f"{onset}\t{TONE_ON_MS}\tSOUND_STREAM\t{cond}\t{spec}\n")
            elif cond in DISK_CONDITIONS:
                n_pos = len(disks)
                if cond == "disks_regular":
                    first = rng.randrange(n_pos)
                    direction = rng.choice((1, -1))
                    idx = [(first + k * DISK_STEP * direction) % n_pos
                           for k in range(DISKS_PER_BLOCK)]
                else:
                    idx = rng.sample(range(n_pos), DISKS_PER_BLOCK)
                    while constant_step(idx, n_pos):
                        idx = rng.sample(range(n_pos), DISKS_PER_BLOCK)
                spec = "~".join(f"{disks[k]}:{ON_MS}:{GAP_MS}" for k in idx)
                fh.write(f"{onset}\t{ON_MS}\tIMAGE_STREAM\t{cond}\t{spec}\n")
            elif cond in SOUND_CONDITIONS:
                item = bags[cond].take(1)[0]
                # A sound row's duration is how long the row occupies the
                # schedule; the clips are 1.6 s by construction.
                fh.write(f"{onset}\t1600\tSOUND\t{cond}\t{item}\n")
            else:
                n_items, on, gap = PACING.get(cond, (PER_BLOCK, ON_MS, GAP_MS))
                if n_items * (on + gap) > BLOCK_MS:
                    raise SystemExit(
                        f"condition {cond!r}: {n_items} x {on + gap} ms does not "
                        f"fit in a {BLOCK_MS} ms block")
                items = bags[cond].take(n_items)
                if n_items == 1 and gap == 0:
                    # A single item needs no stream; IMAGE is the plainer row.
                    fh.write(f"{onset}\t{on}\tIMAGE\t{cond}\t{items[0]}\n")
                else:
                    spec = "~".join(f"{n}:{on}:{gap}" for n in items)
                    fh.write(f"{onset}\t{on}\tIMAGE_STREAM\t{cond}\t{spec}\n")
            onset += span

    print(f"{out.relative_to(HERE)}: {args.blocks} blocks of {BLOCK_MS} ms "
          f"= {onset / 1000:.0f} s (motor trials take "
          f"{SLOTS['motor_visual']} slots each)")
    print(f"  image blocks: {PER_BLOCK} items, {ON_MS} ms on + {GAP_MS} ms blank "
          f"(SOA {soa} ms) = {PER_BLOCK * soa} ms, then "
          f"{BLOCK_MS - PER_BLOCK * soa} ms of fixation")
    for c, (n_items, on, gap) in PACING.items():
        used = n_items * (on + gap)
        print(f"     except {c}: {n_items} item{'s' if n_items > 1 else ''}, "
              f"{on} ms on"
              + (f" + {gap} ms blank" if gap else "")
              + f" = {used} ms, then {BLOCK_MS - used} ms of fixation")
    print(f"  sound blocks: one 1600 ms clip, then {BLOCK_MS - 1600} ms of fixation")
    tone_span = TONES_PER_BLOCK * (TONE_ON_MS + TONE_GAP_MS)
    print(f"  tone blocks : {TONES_PER_BLOCK} tones, {TONE_ON_MS} ms + "
          f"{TONE_GAP_MS} ms silence (SOA {TONE_ON_MS + TONE_GAP_MS} ms) "
          f"= {tone_span} ms; four evenly spaced tones (step "
          + " or ".join(str(k) for k in TONE_STEPS)
          + "), regular = played in that order up or down, random = shuffled")
    disk_span = DISKS_PER_BLOCK * (ON_MS + GAP_MS)
    print(f"  disk blocks : {DISKS_PER_BLOCK} disks on the orbit, {ON_MS} ms + "
          f"{GAP_MS} ms blank (SOA {ON_MS + GAP_MS} ms) = {disk_span} ms; "
          f"regular = a constant {DISK_STEP * 45} deg step from a random "
          f"start, rotating either way, random = four distinct positions "
          f"that do not form a constant step")
    print(f"  motor       : {SLOTS['motor_visual'] * BLOCK_MS} ms per trial - "
          f"the instruction, then the rest as the response window")
    print(f"  spoken eqns : {SLOTS['equations_audio'] * BLOCK_MS} ms per trial - "
          f"the recording, then the rest to solve it")
    print(f"  spoken sents: {SLOTS.get('sentences_audio', 1) * BLOCK_MS} ms per "
          f"trial - {len(pools['sentences_audio'])} recordings, trimmed")
    print(f"  reversed    : {SLOTS.get('sentences_reversed', 1) * BLOCK_MS} ms "
          f"per trial - the same {len(pools['sentences_reversed'])} recordings "
          f"played backwards, as the low-level control")
    print(f"  rest        : {SLOTS[REST] * BLOCK_MS} ms showing the same "
          f"fixation cross as an ordinary gap, marked by a row of its own")
    print(f"  sentences   : {len(sentence_lines)} four-word sentences from "
          f"README.md, shown word by word in order")
    print(f"  consonants  : {len(consonant_lines)} four-string lines from the "
          f"sentence-matched list, shown one string at a time")
    print(f"  conditions  : " + ", ".join(f"{c}({len(v)})" for c, v in pools.items()))
    print(f"  stimuli/    : {len(list(STIM.iterdir()))} files copied")


if __name__ == "__main__":
    main()
