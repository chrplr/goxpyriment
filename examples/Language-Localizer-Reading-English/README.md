# Language Localizer — Reading (English)

The EvLab visual language localizer for fMRI, ported from Psychtoolbox to
goxpyriment.

The participant reads 12-word **sentences** and matched 12-item **nonword**
sequences, presented one word at a time (RSVP). The `sentences > nonwords`
contrast localizes the high-level language network — regions supporting
lexico-semantic and combinatorial processing (Fedorenko et al., 2010).

One run localizes the language regions in most participants; two runs are
recommended, so response magnitudes can be estimated with across-run
cross-validation.

## Run

```bash
go run . -s 3                # subject 3, run 1, set 1
go run . -s 3 -run 2         # second run of the same session
go run . -s 3 -set 2         # a participant who has already seen set 1
go run . -w -s 999           # windowed, for testing
go run . -autostart -s 999   # no SPACE prompt, no trigger: start immediately
```

| Flag | Meaning |
|---|---|
| `-run 1\|2` | Run number. The two runs use mirror-image condition orders (run 1: `SNNS NSNS SNSN NSSN`; run 2: `NSSN SNSN NSNS SNNS`) |
| `-set 1..5` | Stimulus set. Give a participant a set they have not seen: set 1 for a first-time participant (both runs), set 2 for their second session, and so on |
| `-trigger KEY` | Key the scanner sends to start the run (default `t`) |
| `-autostart` | Skip the SPACE prompt and the trigger; start the clock immediately. For timing audits |
| `-fontsize N` | Point size of the RSVP words (default 72) |
| `-dlpio8 PORT` | Send EEG/MEG TTL triggers through a DLP-IO8-G on that port (`/dev/ttyUSB0`, `COM3`), or `auto` to detect one. Omit for no triggers |
| `-ttl-pin-sentence N` | Board pin pulsed at each word onset of a sentence trial (default 1) |
| `-ttl-pin-nonword N` | Board pin pulsed at each word onset of a nonword trial (default 2) |
| `-ttl-pin-probe N` | Board pin pulsed at the press-probe (default 3; 0 = no probe marker) |
| `-photodiode` | Flash a white square in the top-left corner for one frame at every word onset and at the probe |
| `-photodiode-size N` | Side of that square in pixels (default 200) |
| `-w`, `-d N`, `-s N` | Windowed mode, display index, subject ID (standard goxpyriment flags) |

The operator presses SPACE at the instruction screen; a **green** fixation cross
then means "waiting for the scanner". The trigger key starts the run and the
cross turns **grey**. ESC aborts at any point, and the trials already presented
are still written to the data file.

## Design

A run lasts exactly **358 s** (5 min 58 s; 179 images at TR = 2 s):

```
FIX  B1 B2 B3 B4  FIX  B5 B6 B7 B8  FIX  B9 B10 B11 B12  FIX  B13 B14 B15 B16  FIX
```

* 16 blocks of 3 trials = 48 trials; 8 blocks per condition; each block 18 s.
* 5 fixation periods of 14 s (one at the start, one after every 12 trials).
* Every trial lasts exactly 6000 ms:

| Phase | Duration |
|---|---|
| blank screen | 100 ms |
| 12 words, one at a time, no gap | 12 × 450 ms = 5400 ms |
| press-probe (a large disc) | 400 ms |
| blank screen | 100 ms |

Trial onsets are **absolute**, measured from the trigger, exactly as in the
Psychtoolbox original: nothing accumulates drift and a slow trial cannot push
the schedule. Each word is VSYNC-locked by `stimuli.PresentStreamOfText`, and
450 ms is a whole number of frames at 60 Hz (27) and at 120 Hz (54).

**Measured fullscreen** (photodiode on the `-photodiode` patch, AD3 at
100 kS/s, complete 48-trial run) under the conditions below:

| | |
|---|---|
| run length | 358 s, as designed |
| word-to-word interval | **450.021 ms, SD 0.025 ms** for 555 of 576 intervals |
| the other 21 | 18 late by exactly one frame, 3 by two frames; **never early, never dropped** |
| trial onsets | followed the absolute schedule; residual 58 ppm against the AD3's timebase, i.e. two crystals disagreeing, not drift in the design |

The one-frame-late intervals are 3.6 % of word onsets. They are not caused by
the trigger code: a control run with `-dlpio8` off gave the same rate (6 of 132),
and forcing `-exclusive-fullscreen on` did not remove them either. They are the
display stack's, and they delay a word by 16.7 ms without ever losing one.

### Conditions these numbers describe

Every figure on this page was recorded on **2026-08-19** with SDL's
**`wayland` video driver** under GNOME — a compositor is in the path — with the
OpenGL renderer on Mesa Intel Arc (Meteor Lake), 5120×2880 at a nominal
59.996 Hz (measured 60.03), and `GOXPY_VBLANK` not enabled. From the session
metadata:

```
# sys video_driver: wayland          # sys renderer: opengl
# sys gl_renderer: Mesa Intel(R) Arc(tm) Graphics (MTL)
# sys vblank_backend: not requested
# sys refresh_nominal_hz: 59.9960    # sys refresh_measured_hz: 60.0315
```

They are **not** measurements of a `kmsdrm` run. Running from a Linux virtual
console with `SDL_VIDEODRIVER=kmsdrm` takes the compositor out of the path
entirely (see [`docs/LinuxVirtualConsoleSDL.md`](../../docs/LinuxVirtualConsoleSDL.md)),
and both quantities most likely to change are the ones a compositor owns: the
3.6 % of one-frame-late onsets, and the up-to-one-frame trigger-to-photon lead.
The 450.021 ms / SD 0.025 ms pacing of the frames that *are* on time is a
property of the VSYNC lock and should survive. Re-measure before quoting any of
this for a console-mode setup.

> **Do not judge the timing from a windowed test run.** Windowed on the same
> machine, the first ~10 trials were exact and the compositor then throttled the
> surface to 20 Hz for the rest of the run, stretching every 5400 ms stream to
> ~16350 ms. Fullscreen never showed it. Since the words are paced by VSYNC, a
> throttled surface silently slows the whole design; validate fullscreen and
> read `displayed_onset - scheduled_onset` in the data file.

## Task

> In this task, you will read sentences or sequences of word-like nonwords (like
> BLICKET or FLORP). The materials will be shown one word/nonword at a time.
> Your task is to read the materials attentively as they appear. Please read
> silently to yourself, as you would when reading a book. Don't be stressed if
> the words/nonwords seem to be appearing too quickly at first — you will get
> used to the presentation speed after a few trials. At the end of each sequence
> you'll see a disc; whenever you see it, press the response key. This task is
> included to help you stay alert. Your main task is to read attentively.

The response key is `1`. A press is credited to the most recent probe, up to the
next one — so a press arriving during the following sentence still counts, as in
the original script.

## Stimuli

`stim/langloc_fmri_run<run>_stim_set<set>.csv` — the ten stimulus tables of the
Psychtoolbox distribution, **copied unchanged** and embedded in the binary.
Column `stim1` is the item number, `stim2`…`stim13` the 12 words, `stim14` the
condition (`S` = sentence, `N` = nonwords). Each file holds 48 trials, 24 per
condition, grouped in blocks of 3.

**Licence of the stimuli.** These materials come from the EvLab distribution
published at <https://osf.io/vc2bw>, where they are released under the
[Creative Commons Attribution-NonCommercial 4.0 International
licence](https://creativecommons.org/licenses/by-nc/4.0/) (CC BY-NC 4.0). They
are **not** covered by the Apache 2.0 licence of the goxpyriment code, and they
are embedded in the compiled binary — so a binary built from this example
carries the CC BY-NC 4.0 material too. Attribute the source and do not put the
stimuli to commercial use. See the repository `NOTICE` file.

## EEG/MEG triggers

`-dlpio8` drives a [DLP-IO8-G](https://www.dlpdesign.com/usb/io8.php) so the
recording can be segmented on the stimulus itself. The condition is carried by
*which* pin pulses:

| Pin | Signal | Flag |
|---|---|---|
| 1 | pulsed at every word onset of a **sentence** trial | `-ttl-pin-sentence` |
| 2 | pulsed at every word onset of a **nonword** trial | `-ttl-pin-nonword` |
| 3 | pulsed (10 ms) at the **press-probe**, which ends the trial | `-ttl-pin-probe` (0 = none) |

A trial reads as 12 pulses on pin 1 or on pin 2, followed by one on pin 3. A
word pulse is dropped by the first frame callback at least 10 ms after its
rising edge, which on a 60 Hz display makes it one frame wide — **measured
16.675 ms, SD 0.022 ms** (n = 72, AD3 at 100 kS/s).

Do not clear such a pulse by counting display frames: the render loop runs a
frame ahead of scan-out, so the next frame callback arrives ~0.1 ms after the
onset hook, not 16.7 ms after it. An earlier version of this program counted
frames and emitted **0.087 ms** pulses, which most EEG/MEG inputs would miss
entirely. The photodiode patch, being about what is on screen, is still
specified in frames.

Pins are numbered **1–8 as labelled on the board**, like `tests/test_dlpio8`.
The `triggers` package counts lines from 0, so pin 1 is line 0; the conversion
happens inside this program.

**Why one pin per condition rather than 8-bit codes.** The DLP-IO8-G has no
write-all command: `triggers.DLPIO8.Send` writes one ASCII byte per line, so an
8-bit code arrives as eight sequential edges and a system latching on any edge
would record the intermediate values. Only one pin is ever changed at a time
here.

**Where the pulse falls, measured.** The rising edge is emitted from the
stream's post-flip `OnsetCallback`, at the same SDL-clock instant as the
`TimingLog.OnsetNS` recorded in the data file. Against a photodiode on the
patch (AD3, both channels on one timebase, fullscreen under the `wayland`
video driver — see *Conditions these numbers describe* above):

* the trigger **leads the photons by 9.9–24.4 ms**, i.e. by up to one frame;
* within a trial that lead is **constant to ~0.1 ms** — all 12 words of a trial
  share it;
* it changes from trial to trial, because each trial is re-anchored to an
  absolute host-clock deadline and so starts at an arbitrary phase within the
  frame.

The trigger therefore marks the right *frame* reliably, and within a trial the
offset to the photons is a constant you could subtract — but it is not the same
constant across trials. Where that matters, take the photodiode as the
reference; that is what the patch is for.

**Measured on the DLP-IO8-G itself** (2026-08-19):

* Pins 1, 2 and 3 read back as `1` while driven high and `0` while driven low.
* `SetHigh`/`SetLow` block the caller for a median of 5.6 µs (p95 11 µs, max
  20 µs over 200 calls) — negligible against a 16.7 ms frame, which is what
  makes it safe to write the edge from the onset hook.
* That is the host-side write only. The delay from the write to the pin
  actually changing is the device's own latency and needs an oscilloscope;
  `tests/test_dlpio8` is the program for that.

## Photodiode patch

`-photodiode` flashes a white square (200 px by default, `-photodiode-size`) in
the **top-left corner** of the screen, for one frame, on exactly the frame that
carries each word — and on the probe frame. Tape a photodiode over it and the
scope sees the real photons; put the other channel on the TTL pin and the gap
between the two traces is the display pipeline's latency (scan-out plus panel
response), the one part of the chain software cannot measure.

It costs the presentation nothing. The square is not a stream element: it is
drawn from the stream's `FrameCallback`, on top of the word, in a frame that was
being rendered anyway — one more filled rect. The stream's own element timing is
untouched, which is why this is compatible with `PresentStreamOfStimuli*` rather
than a workaround for it.

Verified by reading the rendered frames back with `SDL_RenderReadPixels`
(1024×768 window, 200 px patch): pixel (195,195) is white and (205,205) black on
frame 0 of every word, and both are black on frames 1 and 2 — the square is
exactly 200×200 in the corner and lasts exactly one frame per onset.

Measured with a photodiode on it (AD3 at 100 kS/s, fullscreen): the light rises
10–90 % in 3.4 ms, and the flash is 29.6 ms wide at half amplitude (SD 0.76,
n = 625) for a square drawn on a single 16.7 ms frame — the excess is this
panel's decay, not extra frames. The onset, which is what a photodiode is used
for, is sharp.

The square is drawn on top of the stimulus, so on the participant's display it
is visible unless the sensor housing covers it. In an EEG/MEG booth that is
normally what the photodiode's mount does.

A port that is named but cannot be opened is a fatal error before the run
starts: a session whose triggers are silently missing is a wasted session.
`auto` is forgiving — if no board is found it logs the fact and runs without
triggers, so the same command line works on a machine without the hardware.
Failures during the run are counted, reported at the end, and written to the
session metadata as `ttl_device=DLP-IO8-G port=… errors=N`.

The browser build rejects `-dlpio8`: the `triggers` package is desktop-only.

## Data

One row per presented trial, in `$HOME/goxpy_data`:

| Column | Contents |
|---|---|
| `run`, `set` | which table was used |
| `trial`, `block` | 1…48 and 1…16 |
| `item` | item number within the set (column `stim1`) |
| `condition` | `S` or `N` |
| `words` | the 12 words, space-separated |
| `scheduled_onset` | trial onset in ms from the trigger, as designed |
| `displayed_onset` | measured VSYNC onset of the **first word**, in ms from the trigger |
| `probe_onset` | measured VSYNC onset of the probe, in ms from the trigger |
| `responded` | 1 if the probe was answered, else 0 |
| `rt` | ms from the probe onset to the press, or `n/a` |

`displayed_onset` is what actually reached the display, so onset error is
`displayed_onset - scheduled_onset - 100` (the 100 ms blank precedes the first
word). A per-condition probe-detection summary is printed at the end of the run.

## Differences from the Psychtoolbox original

* **Probe.** The original shows a 480×480 photograph of a hand pressing a
  button; this port draws a filled disc, avoiding a third-party image with no
  recorded licence. The signal to the participant is the same.
* **Trigger key.** The original waits for `+`/`=`; this port waits for `t` (the
  convention of the other goxpyriment localizers) and accepts `-trigger` for
  scanners that send something else.
* **Response attribution.** The original has an off-by-one in its `r_count`
  bookkeeping, so the first trial's response is overwritten and the last is
  recorded twice. Here each probe owns its own window, from its onset to the
  next probe's.
* **Data format.** A CSV row per trial rather than a MATLAB `.mat` structure,
  and both scheduled and achieved onsets are recorded, so onset error can be
  checked offline.
* Presentation is VSYNC-locked, where the original polls `GetSecs` in a busy
  loop.

## References

* Fedorenko, E., Hsieh, P.-J., Nieto-Castañón, A., Whitfield-Gabrieli, S., &
  Kanwisher, N. (2010). New method for fMRI investigations of language: defining
  ROIs functionally in individual subjects. *Journal of Neurophysiology*,
  104(2), 1177–1194.
* Psychtoolbox implementation `evlab_langloc_2conds.m` — T. Scott, MIT, 2013;
  materials and README from the EvLab `LangLoc_Eng_visual_PTB` distribution.

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/Language-Localizer-Reading-English/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
