# Reading-1 — per-letter visibility sampling with lexical decision

A 5-letter string is flashed for 200 ms, split into four 50 ms windows. In every
window all five letters are on screen simultaneously, but **each letter is drawn
independently at high or low contrast** (p = 0.5). A trial therefore carries
**20 independently randomised visibility values** — 5 letter positions × 4 time
windows — and all 20 are written to the data file, because they are the design
matrix a reverse-correlation (classification-image) analysis regresses the
response on.

The windows are followed by a `#####` mask and a lexical decision:
**F = word, J = non-word.**

```
  ITI (blank, 500 ms)
  fixation cross (500 ms)
  window 1  50 ms   B L I N D    ← each letter high or low contrast,
  window 2  50 ms   B L I N D      redrawn independently
  window 3  50 ms   B L I N D
  window 4  50 ms   B L I N D
  mask      200 ms  # # # # #
  blank, await F / J (deadline 3000 ms from stimulus onset)
```

## Running

From the repo root (so `go.work` resolves the workspace):

```bash
go run ./examples/Reading-1 -w -s 1              # windowed, subject 1
go run ./examples/Reading-1 -s 1                 # fullscreen
go run ./examples/Reading-1 -w -s 999 -n 6 -practice 2 -frame 500
```

The last line is the one to use when checking the display by eye: at 500 ms per
window the contrast pattern is slow enough to watch change.

### Flags

Standard: `-w` (windowed), `-d N` (display), `-s ID` (subject).

| Flag | Default | Meaning |
|---|---|---|
| `-hi` | 255 | High-contrast letter luminance, 0–255 |
| `-lo` | 64 | Low-contrast letter luminance, 0–255 |
| `-frame` | 50 | Duration of one visibility window, ms |
| `-nframes` | 4 | Number of visibility windows per trial |
| `-mask` | 200 | Mask duration, ms |
| `-fix` | 500 | Fixation cross duration, ms |
| `-iti` | 500 | Blank inter-trial interval, ms |
| `-timeout` | 3000 | Response deadline, ms, measured from stimulus onset |
| `-n` | 100 | Experimental trials (capped at the stimulus list size) |
| `-practice` | 8 | Practice trials; 0 skips the practice block |
| `-fontsize` | 64 | Point size of the letters and the mask |
| `-stim` | *(embedded)* | Path to a stimulus CSV replacing the built-in list |

## Items

`stimuli.csv` holds **100 base words**: frequent 5-letter English words whose
five letters are **all different**, each with a precomputed `swap_pos` (1–4) —
the position of the first of the two adjacent letters to transpose.

The list was filtered mechanically against the union of
`/usr/share/dict/american-english`, `british-english` and `words`
(75 169 entries). Every row satisfies all of:

- exactly 5 letters, all different;
- the word itself is in the dictionary;
- the transposed form is **not** in the dictionary;
- `swap_pos` is balanced across the four positions, 25 each — "transposing two
  consecutive letters *anywhere* in the word", including the edges.

Frequency was applied by hand-curation from common English vocabulary, not from
a corpus — no frequency list was available offline. A corpus-controlled list can
be dropped in with `-stim` using the same two-column format.

**Which form each base word takes is counterbalanced across subjects.** Base
word *i* (in file order, which is never shuffled) is shown intact when
`(i + subject_id)` is even and transposed otherwise. So consecutive subject IDs
see every item in the opposite form, while within a session no letter string is
ever repeated: 50 word trials and 50 pseudoword trials, 100 distinct strings.

Practice items are hardcoded in `main.go` and disjoint from `stimuli.csv`, so
the practice block does not consume any experimental item.

## Data

Two files per session under the data directory: a plain `.csv` and a companion
`-info.txt`.

| Column | Meaning |
|---|---|
| `subject_id` | prepended automatically |
| `trial` | 1-based trial number in presentation order |
| `item` | the string actually presented |
| `base_word` | the word it was derived from (same as `item` on word trials) |
| `condition` | `word` or `pseudo` |
| `swap_pos` | 0 on word trials; 1–4 on pseudoword trials |
| `key` | SDL keycode of the response |
| `response` | `word`, `nonword`, or `none` (deadline passed) |
| `correct` | true / false |
| `rt_ms` | ms from **stimulus onset** (first window's flip), −1 on timeout |
| `stim_dur_ms` | measured mask onset − stimulus onset |
| `f1p1` … `f4p5` | the 20 visibility values, 1 = high contrast, 0 = low |

The visibility values are one column each, not a packed string, so R or pandas
reads the design matrix directly:

```r
d <- read.csv("Reading-1_sub-001_....csv")
vis <- as.matrix(d[, grep("^f[0-9]p[0-9]$", names(d))])   # trials x 20
```

`rt_ms` is taken from the SDL event clock (`Keyboard.GetKeyEventTS`) against the
VSYNC flip timestamp of the first window, never from a wall-clock delta.

The `-info.txt` records the session constants: the two luminance levels, the
refresh rate, the **measured** window duration, the requested-vs-achieved
refresh counts, the mask string, the response deadline and the font size.
`refresh_hz_source` says where the rate came from — `Screen.RefreshRate`
reports 0 whenever VSync cannot be queried, which is the normal case in the
browser, so the rate is then derived by inverting the frame duration rather
than recorded as a meaningless zero.
`stim_dur_ms` is per trial, so drift is visible in the data rather than hidden
behind a session constant.

## Timing

Each window is held for `round(-frame / frame_duration)` refreshes, never fewer
than one, and the achieved duration is printed at startup and written to
`-info.txt`:

```
Reading-1: 60.04 Hz refresh, display mode via VSync (16.656 ms/frame)
  visibility window: 50 ms requested -> 3 refreshes = 49.97 ms
  4 windows -> 199.87 ms total stimulus
  mask: 200 ms requested -> 12 refreshes = 199.87 ms
```

At 60 Hz, 50 ms is exactly 3 refreshes. At 144 Hz it is 7 refreshes = 48.6 ms —
the program says so rather than silently accepting it. **Check that line before
running participants on an unfamiliar display.**

Presentation goes through `Experiment.ShowFrames`, which redraws before every
flip: a frame carrying no draw calls is not reliably scanned out under a
compositor. GC is disabled for the whole stimulus + mask sequence and restored
afterwards; nothing inside that scope allocates, since all ten letter textures
per trial are built and uploaded during the preceding fixation cross.

`stim_dur_ms` measures the flip loop, not photons. A photodiode run is the
separate, stronger check.

## Running it in a browser

`web/index.html` is a launcher page for the WebAssembly build:

```bash
make wasm-Reading-1-serve     # build + serve on http://localhost:8080
```

See [`web/README.md`](web/README.md) — in particular the timing caveats, which
matter more here than in the other browser examples.

## A property, not a bug

A letter can come up low-contrast in all four windows — p = 1/16 per position —
leaving it near-invisible for the whole trial. That is inherent to independent
sampling and is exactly what makes the reverse-correlation analysis work. It is
not suppressed.

## Implementation note

`stimuli.TextLine` bakes its colour into the GPU texture and caches it, so
changing `.Color` on a live stimulus has no visible effect. Each trial therefore
builds **ten** TextLines — one per (letter position × contrast level) — preloads
them during the fixation cross, and swaps pointers per window into a `letterRow`
composite. Letter positions come from prefix widths measured with
`Font.StringSize`, so the five separately-drawn letters land exactly where a
single `TextLine` of the whole string would put them, without assuming a
monospace advance.
