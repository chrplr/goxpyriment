# Hemispheric Differences in Word Processing

A **continuous recognition memory** task with lateralised study items and central
test probes, replicating Federmeier, K. D., & Benjamin, A. S. (2005),
*Hemispheric asymmetries in the time course of recognition memory*,
Psychonomic Bulletin & Review, 12(6), 993–998. (The paper is in this directory.)

The question is whether the usual left-hemisphere advantage for words survives a
delay. Words are studied in one visual field — projecting first to the opposite
hemisphere — and then tested at fixation, so the test probe itself is not
lateralised and any visual-field effect must come from encoding and retention.
The published result is that the RVF/LH advantage in response time shrinks with
lag and **reverses** at the longest lags.

---

## The paradigm

There are no phases and no blocks. The whole experiment is **one stream** in
which study and test presentations are intermingled:

```
 …  STUDY "GRAVEL" (right)  ·  test "NAIL"  ·  test "LACE"  ·  test "TOOTH"
    ·  test "DONKEY"  ·  test "GRAVEL" → old?  ·  STUDY "SQUID" (left)  · …
```

- A **study** item flashes for 200 ms to the left or right of fixation. The
  participant only memorises it; there is no response.
- A **test** item appears at the centre and stays until the participant judges
  it *old* or *new*.
- Each studied word is tested exactly **`lag` presentations** after it was
  studied — lag is counted in words, not in seconds. Lag 1 means the test is the
  very next thing on the screen.
- Unstudied words are tested once each, scattered through the same stream, so
  old and new probes are indistinguishable in advance.

A fixation cross sits **0.5° below** the vertical centre and is on the screen for
the entire run. Participants are told never to look away from it.

### Design (the default run)

| | |
|---|---|
| Lags | 1, 2, 3, 5, 7, 10, 20, 30, 50 words since study |
| Studied words per lag | 16 — 8 studied in the LVF, 8 in the RVF |
| Studied words | 144 (each shown twice: once laterally, once centrally) |
| Unstudied words | 112 |
| Presentations | 144 + 144 + 112 = **400**, from 256 distinct words |
| Duration | ≈ 20 min, uninterrupted |

Timing follows the paper: study word 200 ms then a 2300 ms interstimulus
interval; test word until response, then a 2500 ms interstimulus interval.

A 20-presentation **practice run** using proper names precedes the experiment,
as in the original.

### Geometry

All spatial parameters are specified in degrees of visual angle and converted to
pixels through `units.Monitor`, so they hold on any display once you tell the
program its physical width and your viewing distance:

| Parameter | Value |
|---|---|
| Viewing distance | 100 cm (the paper's) — `-viewing-distance` |
| Lateral word position | nearest edge 2° from the horizontal centre |
| Fixation cross | 0.5° **below** the vertical centre — deliberately off-centre |
| Word width | fitted so a five-letter word subtends ≈ 2.5° (paper: 2–3°) |
| Colours | black words on a uniform white background |

The lateral offset is computed **per word from its measured width**, so the inner
edge of `HORN` and of `TUNNEL` sit at the same eccentricity.

The off-centre fixation cross is the paper's, not a bug: *"a black fixation
cross, presented at the horizontal center and 0.5° of visual angle below the
vertical center... remained on the screen throughout the experiment"* (p. 995).

### The geometry has to fit

At 100 cm one degree subtends about 1.75 cm, so a lateralised word needs roughly
8 cm of screen either side of centre. **A small window on a high-density display
cannot hold that**, and SDL will happily draw the words off the edge. The program
therefore measures the widest word in the stream at startup and refuses to run if
it would be cropped, naming the pixels needed, the pixels available, and the
screen width that would be required.

Two ways past it:

- Run **fullscreen** (drop `-w`) and pass your display's true `-screen-width`.
  On a 14″ laptop panel that is about 30 cm, not the 52 cm default.
- Pass **`-fit-window`** for development. Every visual angle is shrunk by a
  common factor so the stimuli fit; the run then remains a *preview, not a
  replication*, and both the terminal and the `-info.txt` say so. The info file
  reports the angles **actually presented**, not the design values they came
  from — under `-fit-window` a nominal 2° eccentricity may be recorded as
  `0.87 deg presented`.

The point size actually chosen and the width it produced in degrees are written
into the session's `-info.txt`, so the presented size is on the record rather
than assumed.

The paper also specifies a 0.6° letter height. **This program does not set or
measure letter height**: it sizes the font by rendered *width*, which is the
only text extent this framework can measure directly. The `-info.txt` reports
the font's full line box (ascender to descender) alongside the width, labelled
as such — it is a larger number than a cap height and must not be read as one.
Since the font is monospaced, fixing the width also fixes the height, but the
resulting letter height follows from the typeface's proportions rather than
being verified here.

---

## Counterbalancing

The paper used 16 experimental lists plus 16 matched lists in which every study
item's visual field was reversed. Here that is reproduced from the subject ID,
using three independent bits so that visual field and response hand are not
confounded with each other:

| Subject ID | Effect |
|---|---|
| bit 0 | visual fields swapped — gives the matched list |
| bit 1 | which hand says *old* |
| bits 2+ | which of the 16 item-to-condition assignments is used |

The random number generator is seeded with the **list index**, not the subject
ID, so subjects 0 and 1 see the identical stream with every visual field
flipped. Over participants, each word therefore appears in each visual field.
`-seed N` overrides the list index for testing.

---

## Response keys

`F` (left hand) and `J` (right hand). Which one means *old* alternates across
participants (see above); the instructions screen states the mapping for the
current subject, and the `key` column in the data file records which physical
key was pressed on every trial.

---

## Running

```bash
# From the repository root (go.work resolves the workspace).
go run ./examples/Hemispheric-differences-word-processing -s 1

# Windowed, and the short version, for development.
go run ./examples/Hemispheric-differences-word-processing -s 999 -w -short
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (also selects the list and the response mapping) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | `-1` | Display index (-1 = primary) |
| `-short` | off | 2 studied words per lag instead of 16 — 66 presentations, ≈ 3.5 min. All nine lags are kept. |
| `-no-practice` | off | Skip the practice run |
| `-fit-window` | off | Shrink every visual angle so the stimuli fit a small window — a preview, not a replication |
| `-screen-width` | `52` | Physical width of the display, in cm |
| `-viewing-distance` | `100` | Viewing distance, in cm |
| `-stimuli` | — | Word-list file replacing the embedded `words.txt` |
| `-practice-stimuli` | — | Practice-name file replacing `practice_words.txt` |
| `-seed N` | `-1` | Use list N instead of the one derived from the subject ID |

---

## Stimuli

`words.txt` holds 508 words and `practice_words.txt` 34 proper names, one
uppercase word per line (`#` starts a comment). Both are embedded in the binary,
so the browser build works without a filesystem; `-stimuli` overrides them on
the desktop. Duplicates are rejected at load time, since a word appearing twice
could be drawn as both a studied and an unstudied item.

**The word list is an approximation, not the paper's.** Federmeier and Benjamin
drew 567 items from the MRC Psycholinguistic Database — singular nouns, four to
six letters, imageability 500–700, concreteness 500–700, Kučera-Francis
frequency 2–60. `words.txt` is a hand-picked set of common concrete singular
nouns of the right length; the MRC norms are *not* reproduced in it, and no
norm values are invented. To run the real thing, pass an MRC-derived list with
`-stimuli`.

---

## Output

Two files per session in `goxpy_data/`: a plain `.csv` and a companion
`-info.txt` holding the session metadata (design, list index, response mapping,
monitor geometry, achieved font size, machine and display facts).

One row per presentation. `subject_id` is prepended automatically; fields that
do not apply to a row are written `NA`.

| Column | Description |
|--------|-------------|
| `subject_id` | Participant ID (prepended by the framework) |
| `block` | `practice` or `main` |
| `stream_pos` | 1-based position in the stream |
| `event` | `study` (lateralised) or `test` (central) |
| `word` | The presented word |
| `pair_id` | Links a study row to its test row; `NA` for unstudied words |
| `vf` | `LVF` / `RVF` for a studied word's two rows; `NA` for unstudied |
| `lag` | Presentations since study, on the test of a studied word |
| `is_old` | On a test row: was the word studied earlier in the stream? |
| `response` | `old` or `new` |
| `key` | The physical key pressed (`F` / `J`) |
| `rt_ms` | Flip timestamp to key-event timestamp, both from the SDL event clock. The key queue is cleared immediately before each probe is flipped, so a press made during the preceding interval cannot be consumed as a response to the next word. Computed signed: a negative value would mean a key event preceded its own stimulus and the trial should be discarded. |
| `correct` | Whether the response matched `is_old` |
| `onset_ms` | Stimulus onset relative to the first presentation of the block |

The analysis of interest is hit rate and RT for correct *old* responses, as a
function of `vf` × `lag` (the paper's Figures 1 and 2).

---

## Departures from the original

- **No eye-movement monitoring.** The paper recorded the electro-oculogram and
  discarded study trials containing a saccade to the lateralised word — on
  average about 10 % of items. This version cannot, so hit rates will be
  somewhat diluted relative to the published figures.
- **Word list** — see above.
- **Keyboard rather than a response button in each hand.** `F` and `J` are used
  instead, one per hand.
- The font is the bundled Inconsolata (a monospaced sans serif) rather than the
  paper's unnamed sans serif, and is sized by rendered width rather than by the
  paper's letter height — width is the only text extent measurable on every
  target, WebAssembly included. See the Geometry section.

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/Hemispheric-differences-word-processing/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
