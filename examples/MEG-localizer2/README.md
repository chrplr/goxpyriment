MEG-localizer2
==============

A finger and tone localizer for MEG: one trial about every second, drawn
from four pictures of a hand with one finger coloured red, an empty trial
(nothing on screen but the fixation cross) and five tones one octave apart,
for 160 trials (each of the ten 16 times) — a run of about 160 s.

The program is the one from [`examples/MEG-localizer`](../MEG-localizer): the
same table-driven presentation engine, and the same `main.go`, `ttl.go` and
`ttl_js.go` but for a few lines of `main.go`: the embed pattern, the title,
black on mid-grey instead of white on black (grey so that the white
photodiode square stands out; the hand pictures' background is painted the
same grey), one fixation cross kept on screen at the
same size throughout the run, and the `EMPTY` row type. Everything else that
makes this a different experiment is in `stimuli/` and in the schedule under
`protocols/`.


Running it
----------

The protocol table and every stimulus it names are embedded in the binary,
so it runs on its own: double-clicking the executable opens the session-setup
dialog, which asks for the subject code and carries a **Protocol** selector.
`demo` is the only table shipped; any `*.tsv` dropped in a `protocols/`
directory beside the executable is offered as well, and a table on disk takes
the place of the embedded copy of the same name. The file a name resolved to is
logged at startup and written to the session metadata as `protocol_source`.

From this directory:

    go run .                      # fullscreen, the way a session is run
    go run . -w                   # windowed, for looking at the stimuli
    go run . -s 3                 # subject 3 — given -s, no dialog opens
    go run . -p demo              # choose the protocol on the command line
    go run . -dir .               # read protocols/ and stimuli/ from disk
    go run . -skip-wait           # start at once, no instruction screen
    go run . -ttl megttlbox:/dev/ttyACM0   # TTL code at every onset (below)
    go run . -photodiode=false    # no photodiode square in the corner

The run shows an instruction screen, waits for SPACE, then shows a green
fixation cross and waits for **T** to start. The clock starts at that keypress
and every onset is measured from it. ESC, or closing the window, aborts.

| Flag | Meaning |
|---|---|
| `-p NAME` | protocol table to run, from `protocols/NAME.tsv`; default `demo`, and it overrides the dialog's selector |
| `-s ID` | subject identifier, recorded in the data file |
| `-w` | windowed instead of fullscreen |
| `-d N` | display index |
| `-dir DIR` | read `protocols/` and `stimuli/` from `DIR` instead of the copies embedded in the binary |
| `-no-crosshair` | do not superimpose the fixation cross on the pictures (it stays on screen between them and on empty trials) |
| `-skip-wait` | no instruction screen and no start key |
| `-ttl DEVICE[:PORT]` | send each row's code as a TTL at its onset — see below; default none |
| `-ttl-ms N` | TTL pulse width in ms; default 10 |
| `-photodiode` | flash a square in the top-left corner for one frame at every row onset; on by default, `-photodiode=false` disables it |
| `-photodiode-size N` | side of the photodiode square, in pixels; default 100 |

**Record in fullscreen, never windowed** — see the note in
[`MEG-localizer/README.md`](../MEG-localizer/README.md): a compositing desktop
may stop an unfocused window from blocking on VSYNC, and stimulus durations
suffer. `-w` is for looking at stimuli, not for data.


The stimuli
-----------

| Condition | Type | File | What it is | Duration |
|---|---|---|---|---|
| `finger_index` … `finger_little` | `IMAGE` | `f2.jpg` … `f5.jpg` | a white hand with a black outline on grey, with one finger filled red | 500 ms |
| `empty` | `EMPTY` | — | nothing: the fixation cross alone, for the trial's duration | 500 ms |
| `tone_200` … `tone_3200` | `SOUND` | `tone_00200Hz.wav` … `tone_03200Hz.wav` | narrow-band noise centred on 200, 400, 800, 1600, 3200 Hz | 500 ms |

The pictures are 187 × 244 px, shown at the centre of the screen. The
fixation cross — black, 40 px arms, the same one throughout — is on screen
for the whole run: alone between trials and during an empty trial, drawn on
top of the hand during a picture. The screen is mid-grey (128) and each
picture's background is painted the same grey, so it is invisible: what
appears and disappears is the white hand.

The empty trial takes the slot of the thumb picture: `f1.jpg` is still under
`stimuli/` (and embedded), but no row of `demo.tsv` names it. An `EMPTY` row
is timed, logged and triggered like a picture; its `stimuli` field is a label
(`-`) that goes into the data file.

The tones are one octave apart, spanning the tonotopic axis of primary
auditory cortex, and are equalised for loudness on the ISO 226 60-phon
contour (that corrects for the ear, not for the headphones — calibrate the
playback chain separately). They have 10 ms cos² ramps.

The sources and generators sit under `stimuli/` alongside the files the
program embeds (only `stimuli/*.jpg` and `stimuli/*.wav` are embedded; the
subdirectories are not):

| Directory | Contents |
|---|---|
| `stimuli/hands/` | `fingers_all.jpg`, the five hands on one sheet; `split_hands.py`, which cuts it into `f1.jpg` … `f5.jpg`; and those five cuts at full size (568 × 738 px), white background; `make_stimuli.py`, which makes the copies in `stimuli/` from them: the background around the hand (the white connected to the border) painted grey 128, then scaled down to 187 × 244 |
| `stimuli/tones/` | `make_tones.py` and `description.md`; `set1/` is the 1-octave set in use, `set2/` a 0.6-octave alternative (200 … 1056 Hz) |

To try `set2`, copy its files over `stimuli/` and change `TONES_HZ` in
`make-protocol.py`.


The schedule
------------

`make-protocol.py` writes `protocols/demo.tsv`:

    ./make-protocol.py                        # protocols/demo.tsv, seed 0
    ./make-protocol.py --seed 7 --name run7   # another order, another table

- Each of the 10 trial types (4 fingers, the empty trial, 5 tones) appears 16
  times: the order is 16 rounds, each a permutation of the 10, so the counts
  stay balanced over the run, and a round may not start with the type the
  previous one ended with, so a stimulus never follows itself.
- Successive onsets are 1000 ms apart ± a uniform jitter of up to 100 ms,
  drawn afresh for each trial (SOA in 900–1100 ms). The jitter is on the SOA,
  not on a fixed grid, so the run length varies by a few hundred ms from one
  seed to the next; with seed 0 the last onset is at 160.2 s.
- The first stimulus comes 1 s after the start key.

The table format is that of MEG-localizer: `onset_time` and `duration` in ms,
`type` `IMAGE`, `SOUND` or `EMPTY`, a `cond` label, the stimulus file name (a
bare label for `EMPTY`), and the TTL `code` sent at the onset.

    onset_time	duration	type	cond	stimuli	code
    1000	500	SOUND	tone_800	tone_00800Hz.wav	8
    1991	500	SOUND	tone_1600	tone_01600Hz.wav	9
    3078	500	IMAGE	finger_index	f2.jpg	2
    8236	500	EMPTY	empty	-	1

Rows play in order and must not overlap; the loader rejects a table where they
do. The fixation cross fills every gap.


Triggers
--------

Every row carries a code — **1 for the empty trial, 2–5 for the fingers**
(index to little finger) and **6–10 for the tones** (200 Hz to 3200 Hz) — and
with `-ttl` that code is
written to the eight TTL lines at the row's onset and cleared `-ttl-ms` later
(10 ms by default; the achieved width is between that and one frame more).
The write is issued from the stream's post-flip onset hook, on the flip thread,
immediately after the VSYNC flip that shows the picture or starts the sound —
the same instant recorded as `actual_ms`. It is not the photon time: the flip
leads the panel by a constant one to three frames depending on the display
stack, to be measured once per rig and subtracted (see
`docs/TriggerJitterForEEGandMEG.md`).

| `-ttl` | Device |
|---|---|
| `megttlbox:/dev/ttyACM0` | NeuroSpin MEG TTL box (Arduino Mega); port required |
| `mmbts:/dev/ttyACM0` | NEUROSPEC MMBT-S; port required. In its factory pulse mode every trigger is 8 ms wide whatever `-ttl-ms` says |
| `dlpio8[:PORT]` | DLP-IO8-G; a port name, or `auto` (the default) to probe for one. Its `Send` writes the lines one by one, so a code takes ~1 ms to settle |
| `parallel[:/dev/parport0]` | parallel port; the first accessible one by default |

A device that was asked for and cannot be opened stops the program before the
window opens, and so does a table without a single code — a run recorded
without its triggers is only found out when the data are useless. Trigger
failures during the run are counted, reported at the end, and written to the
session metadata (`ttl_device=… errors=N`). All lines are driven low before
the first row and after the last. Triggers are not available in the browser
build.

The codes are just a column of the table: `make-protocol.py` writes them, and
a table without the column runs without triggers.

Photodiode
----------

At every row onset a 100×100 px square (`-photodiode-size`) is drawn in the
top-left corner for one frame. It is drawn into the frame whose flip fires the
TTL code, so the photodiode and the trigger channel mark the same flip, and
the difference between them is the display latency. The square is white on
the grey (128) background, so the photodiode sees a grey-to-white step. It
flashes on every row, sound rows included; there it marks the flip after
which the sound is started, not the sound itself, whose latency must be
measured separately (e.g. with a microphone). `-photodiode=false` turns it
off.


The data file
-------------

Two files per session under the data directory: a CSV with one row per event
and a `-info.txt` with the session metadata (display, refresh rate, drivers).
Columns are `subject_id`, `intended_ms`, `actual_ms`, `event`, `cond`,
`stimuli`, `code`, so the scheduled and the achieved onset are both recorded,
per event, on the same clock as any response, next to the TTL code the
recording saw. Events are `IMAGE_ONSET`, `IMAGE_OFFSET`, `EMPTY_ONSET`,
`EMPTY_OFFSET`, `SOUND_ONSET` and `RESPONSE` (any key press, timestamped on
the run clock; code 0).


Not yet done
------------

- **The trigger path has not been exercised on hardware.** The wiring is the
  one from `Language-Localizer-Reading-English`, but no device was connected
  when it was written; the first run on the rig should check the codes and
  their widths on the STI channel.
- **The task.** Nothing tells the participant what to do with the finger
  pictures (look, or move the finger shown). The instruction screen only asks
  them to keep still and fixate; add a line there if a movement is expected.

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/MEG-localizer2/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
