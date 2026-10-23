# MEG Reading Sentences

The participant reads 12-word **sentences** and matched 12-item **nonword**
strings, one word at a time (RSVP), while the MEG records. Every word onset is
marked on the STI channel with an 8-bit code that gives the condition and the
position of the word, so the recording can be epoched on each word.

The trial is that of the EvLab reading localizer
([`../Language-Localizer-Reading-English/`](../Language-Localizer-Reading-English/)),
used here in an **event-related** design: the two conditions are intermixed
rather than blocked, and there is no fMRI schedule.

## Run

```bash
# From the repo root (go.work resolves the workspace).
go run ./examples/MEG-Reading-Sentences -s 3 -practice            # practice, before the MEG
go run ./examples/MEG-Reading-Sentences -s 3 -run 1 -ttl parallel # run 1 of 4
go run ./examples/MEG-Reading-Sentences -s 3 -run 2 -ttl parallel # run 2, and so on to -run 4
go run ./examples/MEG-Reading-Sentences -w -s 999 -run 1 -autostart -ttl null   # windowed test, no hardware
```

Use the **same `-s`** (and `-seed`) for all the runs of a participant: the four
lists are rebuilt from them at every launch, and together they present every
item exactly once.

| Flag | Meaning |
|---|---|
| `-run N` | Run to present, 1 to `-nruns` |
| `-nruns N` | Number of runs the 278 items are dealt into (default 4) |
| `-practice` | Present practice items from the training tables instead of a run |
| `-practice-n N` | Practice items per condition (default 5) |
| `-seed N` | Random seed, combined with the subject ID (default 1) |
| `-resp-key C` | Key sent by the response button (default `1`) |
| `-wait-trigger`, `-trigger C` | Wait on a green cross for key `C` (default `t`) before the run |
| `-autostart` | Skip the instruction and result screens (test runs) |
| `-fontsize N` | Point size of the words (default 72) |
| `-ttl KIND[:PORT]` | TTL device: `parallel[:/dev/parport0]`, `megttlbox:/dev/ttyACM0`, … (see below); omit for none, `null` to exercise the code path |
| `-ttl-ms N` | TTL pulse width (default 5 ms) |
| `-ttl-sentence-base`, `-ttl-nonword-base`, `-ttl-probe`, `-ttl-sync-code` | Trigger codes (see below) |
| `-photodiode=false`, `-photodiode-size N`, `-marker-frames N` | Photodiode square (on by default, 200 px, 1 frame) |
| `-w`, `-d N`, `-s N` | Windowed mode, display index, subject ID |

ESC aborts; the events already presented are in the data file, and the
end-of-run sync code is still sent.

## Design

* **Items.** 139 sentences (`stimuli/sentences_shuffled.csv`) and 139 nonword
  strings (`stimuli/nonwords_shuffled.csv`), all 12 words long. The practice
  items (`stimuli/training_*.csv`, 101 + 101) do not overlap with them.
* **Runs.** Each condition is shuffled and split as evenly as possible over
  the runs: with 4 runs, 35 + 35, 35 + 35, 35 + 35 and 34 + 34 trials. Within a
  run the conditions are intermixed in random order, never more than 3 trials
  of one condition in a row. The lists and the fixation durations come from a
  random generator seeded with `-seed` and the subject ID.
* **Trial**, on absolute onsets from the start of the run (nothing
  accumulates drift):

| Phase | Duration |
|---|---|
| blank screen | 100 ms |
| 12 words, one at a time, no gap | 12 × 450 ms = 5400 ms |
| press-probe: a large disc (press the button) | 400 ms |
| fixation cross | 1000–1500 ms, uniform |

A run opens with 2 s of fixation. A 70-trial run lasts about 8.2 min. Words
are VSYNC-locked (`stimuli.PresentStreamOfStimuliFunc`); 450 ms is a whole
number of frames at 60 Hz (27) and at 120 Hz (54).

## Task

The participant reads attentively and presses the response button whenever
the disc appears. A press is credited to the most recent disc, up
to the next one, so a slow press made during the following item still counts.
The result screen and the console report how many discs were answered
after sentences and after nonwords.

The disc (radius 110 px) replaces the hand photograph of the EvLab
original, as in `../Language-Localizer-Reading-English/`: the photograph has
no recorded licence.

## MEG triggers

Each code is written right after the flip that shows the event, on the same
SDL-clock instant as the `onset_ms` recorded in the data file, and cleared
5 ms later (the library's onset marker, `control.MarkerFlags`).

| Code | Event | Flag |
|---|---|---|
| 1–12 | word at position 1–12 of a **sentence** | `-ttl-sentence-base` (0) + position |
| 21–32 | nonword at position 1–12 of a **nonword string** | `-ttl-nonword-base` (20) + position |
| 50 | disc onset | `-ttl-probe` |
| 100 | start and end of the run, for a clock-drift check against the MEG file | `-ttl-sync-code` (0 = none) |

The program refuses overlapping code ranges and codes outside 1–255.

**The codes have several bits set, so all 8 lines must change at once.** The
DLP-IO8-G and DLP-IO20 write a code line by line, and the STI channel would
record the partial values on the way. The program therefore refuses
`-ttl dlpio8` and `-ttl dlpio20`. Use the parallel port or the NeuroSpin MEG
TTL box (`megttlbox`), which sets its port atomically when its firmware
supports it.

**Not yet measured:** the codes as the STI channel receives them, and the
trigger-to-photon lag on the MEG projector. Both need a session at the rig
with the photodiode on the square in the top-left corner. Until then, take
the photodiode as the reference for onset times.

## Data

One row per event, in `$HOME/goxpy_data`. Times are in ms from the start of
the run, on the SDL clock.

| Column | Contents |
|---|---|
| `run` | run number, or `practice` |
| `event` | `word`, `probe`, `response`, `sync_start`, `sync_end` |
| `trial`, `position` | trial number in the run; word position 1–12 (word rows) |
| `condition` | `S` or `N` |
| `item` | row number of the item in its stimulus table |
| `text` | the word |
| `code` | TTL code sent with the event |
| `scheduled_ms` | onset as designed |
| `onset_ms` | measured onset: the flip that showed it, or the key press time |
| `rt_ms` | for a response: ms from the disc it was credited to |
| `outcome` | for a response: `hit`, `extra` (the disc was already answered) or `false_alarm` (before the first disc) |

**Smoke test, not a timing validation** (2026-10-23; windowed 1024×768 under
the `wayland` video driver, 60.04 Hz; practice list of 2 + 2 items; `-ttl null`):
the 44 within-item word-to-word intervals had a mean of 450.4 ms, range
448.8–467.8 ms (the maximum is one frame late). The probe's first
flip came up to 29 ms after its scheduled time (n = 4). Validate fullscreen
on the MEG display before using these figures.

## Stimuli and licence

The sentences and nonword strings are items of the EvLab language localizer,
published at <https://osf.io/vc2bw> under the [Creative Commons
Attribution-NonCommercial 4.0 International
licence](https://creativecommons.org/licenses/by-nc/4.0/). They are not
covered by the Apache 2.0 licence of the code, and they are embedded in the
compiled binary. See the repository `NOTICE` file.

## References

* Fedorenko, E., Hsieh, P.-J., Nieto-Castañón, A., Whitfield-Gabrieli, S., &
  Kanwisher, N. (2010). New method for fMRI investigations of language: defining
  ROIs functionally in individual subjects. *Journal of Neurophysiology*,
  104(2), 1177–1194.

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/MEG-Reading-Sentences/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
