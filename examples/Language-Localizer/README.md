# Bilingual auditory language localizer (fMRI)

A goxpyriment port of an Expyriment experiment (E. Lin, 2016). It presents
spoken sentences on a fixed onset schedule during an fMRI scan so that the BOLD
response to the participant's native language can be contrasted with the
response to unknown languages, localizing the language network.

## Design

- Sentences are grouped into **three-sentence blocks**, each block in a single
  language: **French (`fr`)**, **Mandarin Chinese (`ch`)**, or **Wolof
  (`wol`)**. Blocks alternate across the run.
- Every subject hears the same sentence pool in a **counterbalanced order**.
  The order and the exact onset times are read from a per-subject stimulus
  table, `stim/bilingue_localizer_subj<N>.csv`.
- The participant does not respond — they simply fixate a cross and listen.
- The run ends with a 1 s silent probe (block `99`).

## Running

The subject ID selects the stimulus table:

```bash
go run . -s 2            # uses stim/bilingue_localizer_subj2.csv
go run . -w -s 2         # windowed mode, for testing without a scanner
go run . -training -s 2  # short practice run (bilingue_localizer_training.csv)
```

Flags: `-w` windowed, `-d N` display index, `-s N` subject ID,
`-training` runs the short practice list (10 sentences, ~50 s) shared by all
subjects instead of the subject's table. The subject ID is still recorded in
the data file.

### Sequence

1. Instruction screen — the operator presses **SPACE** when ready.
2. A **green** fixation cross while the program waits for the scanner
   synchronisation pulse, delivered as a **`t`** keypress (the convention for
   MRI trigger boxes that emulate a keyboard). When testing off-scanner, just
   press `t` yourself.
3. A **grey** fixation cross for the rest of the run. The timing clock starts at
   the trigger, and each sentence is played the moment the clock reaches its
   scheduled onset (a sleep followed by a short busy-wait gives sub-millisecond
   accuracy).

## Data

One row per sentence is written to the data file, with columns:

| column | meaning |
|---|---|
| `subj` | subject ID |
| `nbloc` | block number (`99` = terminal silent probe) |
| `langue` | language of the block (`fr` / `ch` / `wol`) |
| `sent_onset` | scheduled onset, ms from the trigger |
| `real_sentence_onset_before` | clock reading (ms) just before `Play()` |
| `real_sentence_onset_after` | clock reading (ms) just after `Play()` |
| `sent_dur` | sentence duration (ms) |
| `filename` | WAV file played |

The `sent_onset` vs. `real_sentence_onset_*` columns let you verify onset
jitter offline.

## Assets

`sound_files/` (the sentence WAVs) and `stim/` (the per-subject tables) are
embedded into the binary with `//go:embed`, so it is self-contained.

## Notes on the port

- The Expyriment version took the stimulus-table path as a command-line
  argument; here the goxpyriment `-s N` flag selects the embedded table,
  matching the framework's convention.
- The MRI trigger (keyboard `t`), the fixation-cross sizes/colours, the fixed
  onset schedule, and the recorded data columns all mirror the original.
