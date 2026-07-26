RSVP-multimodal — a table-driven multimodal localizer
=====================================================

Presents a multimodal stimulation script — rapid serial visual presentation of
words, flashing checkerboards, and spoken sentences — on a fixed **absolute
onset schedule** read from a stimulus table.

The bundled protocols implement the French version of the **Pinel functional
localizer**, a five-minute fMRI run contrasting four tasks, each delivered in
both a visual and an auditory modality:

| Task | Conditions |
|---|---|
| Read / listen to sentences | `phraseVideo`, `phraseAudio` |
| Read / listen to subtractions, solved silently | `calculvideo`, `calculaudio` |
| Read / hear "press the left/right button three times", then do it | `clicGvideo`, `clicGaudio`, `clicDvideo`, `clicDaudio` |
| Passively view flashing checkerboards | `CheckBoardH`, `CheckBoardV` |

Contrasting these isolates the language, number, motor, and visual networks in a
single short run.

Usage
-----

    go run . -p instructions    # spoken/written instructions, once, before the scanner
    go run . -p run1 -s 1       # subject 1, run 1
    go run . -w -p run1         # windowed, for testing

Flags: `-p` protocol (`instructions`, `run1`…`run4`), `-s N` subject ID, `-w`
windowed mode, `-d N` display index, `-skip-wait` to start immediately (no
instruction screen, no trigger). ESC or closing the window aborts.

Each run waits for the scanner synchronisation pulse, delivered as a `t`
keypress (the convention for MRI trigger boxes emulating a keyboard); the clock
starts at the pulse, so all onsets are measured from it. The `instructions`
protocol is shown outside the scanner and starts on SPACE instead.

The stimulus table
------------------

Nothing about the paradigm is hardcoded. A run is a tab-separated table in
`protocols/`:

```
onset_time	duration	type	cond	stimuli
0	400	TEXT_STREAM	calculvideo	calculez~seize~moins~huit
8700	400	IMAGE_STREAM	CheckboardH	checkerboard_hpb.bmp~checkerboard_hnb.bmp
11400	400	SOUND	clicDaudio	clic3D.wav
15000	400	SOUND	phraseAudio	ph1.wav
```

`onset_time` is in ms from the start of the run; `duration` is in ms. `cond` is
optional (the instructions protocol omits it) and is carried through to the data
file. The types are:

| Type | Meaning |
|---|---|
| `TEXT` | a line of text |
| `BOX` | a block of text; a literal `\n` in the field starts a new line |
| `IMAGE` | an image file, centered |
| `SOUND` | a `.wav` file |
| `TEXT_STREAM` | words shown one after another (RSVP), `~`-separated |
| `IMAGE_STREAM` | images shown one after another, `~`-separated |
| `SOUND_STREAM` | sounds played one after another, `~`-separated |

In a `*_STREAM` row, `duration` is the default duration of each element; an
element may override it as `name:duration` or `name:duration:gap` (ms).

A fixation cross is shown whenever no stimulus is on screen — including while a
`SOUND` plays, which does not interrupt the display. Stimulus files are looked
up by name in `stimuli/`. Both directories are embedded into the binary, so it
is self-contained and nothing is loaded from disk mid-run.

To use different stimuli, edit or add a table under `protocols/`, drop the files
into `stimuli/`, and rebuild with `go build .`

Timing
------

Every row is presented by a single `stimuli.PresentStreamOfStimuli` call —
VSYNC-locked, with GC disabled for the whole run — preceded by a fixation-cross
element filling the gap until the row's scheduled onset. **That gap is
recomputed from the master clock on every row**, which re-anchors the schedule:
the ~1-frame rounding of each slot is absorbed by the next gap instead of
accumulating, so onset error does not grow with time.

The re-anchor needs slack to work, so it corrects at every row that has a gap —
which, in `run1`–`run4`, is every row: they are seconds apart, and onsets land
within a few ms of schedule. Error can only build up along a *chain of rows with
no gap between them*, where there is nothing to absorb it. The instructions
protocol has one such chain (44 s of back-to-back screens), and the error creeps
to ~96 ms across it before the next gap resets it to ~0 — about 0.2%, the
display's frame-quantization. That is irrelevant for an instruction screen; it
would matter if a table packed timed stimuli back to back for a long stretch.

Because rows play sequentially, a row must finish before the next is due; the
loader rejects a table whose rows overlap.

**Record data in fullscreen, never in windowed mode.** `-w` exists for testing
the stimuli, not for running participants. A compositing desktop is free to
throttle an unfocused or occluded window: on a GNOME/Wayland box this was
measured throttling presents to ~20 Hz while the display still reported 60 Hz.
Since a stimulus lasts a fixed number of *frames*, every slot then stretched by
3× and the schedule slipped — the 100 s instructions protocol took 133 s, and a
five-minute run finished 65 s late. The same run fullscreen finished in exactly
its nominal time, with a mean onset error of 38 ms. This is a property of the
display path, not of the schedule; if the console reports a large onset error,
suspect the window mode or the compositor before the table.

Both the scheduled and the achieved onset are written to the data file, and the
console prints the onset error at the end of the run:

```
presented 80 stimuli; onset error: mean 3.8 ms, worst 5 ms; 6 responses
```

Data
----

One CSV per session (plus an `-info.txt` with session metadata), with one line
per event, in chronological order:

```
subject_id,intended_ms,actual_ms,event,cond,stimuli
1,"0",0,"TEXT_STREAM_ONSET","calculvideo","calculez~seize~moins~huit"
1,"1600",1589,"TEXT_STREAM_OFFSET","calculvideo","calculez~seize~moins~huit"
1,"11400",11405,"SOUND_ONSET","clicDaudio","clic3D.wav"
1,"n/a",13337,"RESPONSE","clicDaudio","Right"
```

`intended_ms` is the scheduled time and `actual_ms` the achieved one, both in ms
from the trigger; the difference is the onset error. A `RESPONSE` has no
scheduled time and carries the key name in `stimuli`, labelled with the
condition in force when it was pressed — for a press during a gap, the row that
asked for it, which is what makes the left/right button task scorable.

`_OFFSET` events are timestamped when the last frame of the stimulus was
flipped, so they read one refresh period (~17 ms) earlier than the nominal
`intended_ms`; that frame is still on screen for its full duration.

Credits
-------

Ported from the [gostim2](https://github.com/chrplr/Pinel-localizer-go)
implementation of the Pinel localizer. The stimuli were designed by Philippe
Pinel at the INSERM U562 "Cognitive Neuroimaging Unit" (http://www.unicog.org).

> Pinel, P., Thirion, B., Meriaux, S., Jobert, A., Serres, J., Le Bihan, D.,
> Poline, J.-B., & Dehaene, S. (2007). Fast reproducible identification and
> large-scale databasing of individual functional cognitive networks.
> *BMC Neuroscience*, 8, 91. https://doi.org/10.1186/1471-2202-8-91

License: Apache License 2.0.
