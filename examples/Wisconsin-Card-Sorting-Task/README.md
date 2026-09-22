# Wisconsin Card Sorting Task

A computerised version of the **Wisconsin Card Sorting Test** (Berg, 1948; Grant & Berg, 1948; Heaton et al., 1993), the classic measure of *set shifting* — the ability to abandon a rule that has stopped working.

Four **key cards** are displayed in a row at the top of the screen:

| 1 | 2 | 3 | 4 |
|---|---|---|---|
| one red triangle | two green stars | three yellow crosses | four blue circles |

On each trial a **test card** appears below them. The participant decides which key card it goes with and says so by pressing `1`–`4` or by clicking the card. The only feedback is **Correct** or **Wrong**.

The sorting rule is never announced. It is one of **number**, **colour** or **shape**, and it changes — without warning, to one of the other two — as soon as the participant has made a run of 6 to 10 consecutive correct sorts (the run length is drawn at random for each category). Once the rule has changed, continuing to sort by the old one produces a **perseverative error**, which is what the test is built to count.

---

## Trial structure

```
ITI (blank)  →  key cards + test card  →  response (1–4 or click)  →  feedback
   400 ms              (self-paced)                                     800 ms
```

The chosen key card is framed in black during the feedback, so the participant can see which card the feedback refers to.

---

## The deck

The stimuli are the 64 cards in `stimuli/` (4 numbers × 4 colours × 4 shapes), embedded in the binary with `//go:embed`.

By default the test cards are drawn from the **24 unambiguous cards** — those whose number, colour and shape each point at a *different* key card. Every response then identifies exactly one rule, which is what makes perseveration readable in the data. `-all-cards` switches to the 60-card deck of the clinical version, where a card can match one key card on two dimensions at once.

Cards are drawn without replacement and the pool is reshuffled when exhausted (never repeating a card across the seam), so no card recurs until all the others have been seen.

---

## Running

```bash
# From the repository root (go.work resolves the workspace)
go run ./examples/Wisconsin-Card-Sorting-Task -s 1        # fullscreen
go run ./examples/Wisconsin-Card-Sorting-Task -s 1 -w     # windowed
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID |
| `-w` | off | Windowed mode (1024×768) |
| `-d N` | -1 | Display index (-1 = primary) |
| `-n` | 128 | Number of trials |
| `-minrun` | 6 | Minimum consecutive correct sorts before the rule changes |
| `-maxrun` | 10 | Maximum consecutive correct sorts before the rule changes |
| `-fb` | 800 | Feedback duration (ms) |
| `-iti` | 400 | Inter-trial interval (ms) |
| `-soa` | 0 | Fixed stimulus-onset asynchrony (ms). 0 = self-paced trials; any positive value puts every trial on a fixed grid (scanner mode, below) |
| `-trigger` | off | Wait for the scanner trigger (the first `t` keypress after the instructions) before starting |
| `-all-cards` | off | Use the full 60-card deck instead of the 24 unambiguous cards |

ESC quits at any point; the data collected so far are saved.

---

## Scanner mode: fixed SOA

With `-soa` the trial is a fixed grid instead of a self-paced sequence, which is what an fMRI (or EEG) design needs:

```
t = k*soa                                            t = (k+1)*soa
   |<--- card: soa - fb - iti --->|<-- fb -->|<- iti ->|
   card onset                     feedback    blank     next card onset
```

- The card stays on screen for the **whole** response window, whether or not the participant has already answered; later presses in the same window are ignored. Feedback onset is therefore on the grid and does not inherit the jitter of the reaction time.
- Every phase is timed against a single session zero, not against the phase before it, so a late frame costs its own phase and nothing downstream: onsets cannot drift over a run.
- **Total duration = `n` × `soa`**, exactly, excluding the instruction screen. `-n 128 -soa 4000` is 512 s = 8 min 32 s.
- The response window is `soa - fb - iti`; it must leave at least 500 ms or the program refuses to start. With the defaults (`-fb 800 -iti 400`), `-soa 4000` gives a 2800 ms window.
- A trial with no response shows **Too slow**, is logged with `response = 0`, `modality = none`, `rt_ms = 0`, `correct = false`, and (like any error) resets the run of consecutive correct sorts.

Measured on this machine with `-n 8 -soa 2000` (windowed, no responses given): onsets fell at 16, 2001, 4001, 6001, 8000, 10000, 12000, 14000 ms against targets 0, 2000 … 14000 — trials 2–8 within 1 ms of the grid, trial 1 one frame late because its flip waits for the first retrace after the session zero. The clock used for scheduling is the Go monotonic clock; reaction times are still taken from the SDL event timestamps against the VSYNC-stamped flip.

### Starting on the scanner trigger

`-trigger` inserts a wait between the instructions and the first card:

```
instructions (SPACE)  →  "Waiting for scanner TTL…"  →  first 't'  →  trial 1
```

The screen shows **Waiting for scanner TTL…** until the first `t` keypress arrives — the pulse most scanners send at the start of each volume, delivered as a keystroke by the usual interfaces. Subsequent `t` pulses during the run are ignored: they are not response keys.

Trial onsets are counted from the **pulse**, not from the moment the program noticed it. The trigger is detected by a 1 ms polling loop, so the session clock is reset a millisecond or two late; the SDL event timestamp of the keypress says by how much, and the grid is shifted back by that lag. The lag is printed at startup and written into the `-info.txt`.

Verified with a simulated pulse placed 3 ms in the past (`-n 4 -soa 2000 -trigger`): the run reported a 3 ms detection lag and trial onsets landed at 16, 1998, 3997, 5997 ms against targets −3, 1997, 3997, 5997 — i.e. on the trigger-anchored grid, the first trial again one frame late because of the flip. The waiting screen itself was checked by frame capture. The real `t`-keypress path uses the standard `Keyboard.GetKeyEventTS` call and has not been exercised against a scanner.

ESC works on the waiting screen, so a run can be abandoned before the first pulse.

---

## Responses

- **Keyboard:** `1`, `2`, `3`, `4` (digit row or keypad) for the key card at that position, left to right.
- **Mouse:** left-click anywhere on a key card. Clicks outside the four cards are ignored.

The cursor is made visible at startup for the mouse route. On display back-ends that cannot create a cursor shape (KMSDRM on a bare console), a warning is printed and the keyboard route still works.

---

## Output

One `.csv` per session in the data directory (plus the companion `-info.txt` with the session metadata):

| Column | Meaning |
|---|---|
| `trial` | Trial number, 1-based |
| `category` | Index of the current rule block (1-based) |
| `rule` | Rule in force during this trial: `number`, `color` or `shape` |
| `trial_in_category` | Position of the trial within the current rule block |
| `run_correct` | Consecutive correct sorts after this trial |
| `test_card` | e.g. `3-blue-triangle` |
| `number`, `color`, `shape` | The test card's attributes |
| `match_number`, `match_color`, `match_shape` | The key card (1–4) this test card belongs under by each rule |
| `response` | Key card chosen, 1–4 (0 = no response before the deadline, fixed-SOA mode only) |
| `modality` | `key`, `mouse`, or `none` (no response) |
| `rt_ms` | Time from the VSYNC-stamped card onset to the response event |
| `correct` | Whether the response matched the rule in force |
| `matched_dims` | Dimensions the chosen card actually shared with the test card (`color+number`, empty if none) |
| `perseverative` | True when an error sorts by the rule abandoned at the last change (Heaton's perseveration to the previous principle) |
| `rule_changed_after` | True when this trial completed a category and the rule changed |

`rule`, `category` and `trial_in_category` describe the state *during* the trial; a change triggered by the trial is reported by `rule_changed_after` and takes effect on the next row.

---

## References

Berg, E. A. (1948). A simple objective technique for measuring flexibility in thinking. *The Journal of General Psychology*, 39(1), 15–22. https://doi.org/10.1080/00221309.1948.9918159

Grant, D. A., & Berg, E. A. (1948). A behavioral analysis of degree of reinforcement and ease of shifting to new responses in a Weigl-type card-sorting problem. *Journal of Experimental Psychology*, 38(4), 404–411. https://doi.org/10.1037/h0059831

Heaton, R. K., Chelune, G. J., Talley, J. L., Kay, G. G., & Curtiss, G. (1993). *Wisconsin Card Sorting Test Manual: Revised and Expanded*. Psychological Assessment Resources.

Wikipedia: https://en.wikipedia.org/wiki/Wisconsin_Card_Sorting_Test

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/Wisconsin-Card-Sorting-Task/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
