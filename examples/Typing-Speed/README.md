Typing Performance
==================

A series of trials in which a target sentence is presented on the screen and the
participant must copy it into a framed input box displayed just below. A blinking
cursor sits at the current column, just below the next target character to copy,
so the target never scrolls horizontally. All keypress onsets are recorded with their
time in milliseconds — including mistakes and editing keys (Backspace, arrows,
Delete). The data file contains one line per keystroke, with its onset time, the
interval since the previous key, and a three-way classification: whether it was
the correct character, an incorrect character, or a movement/editing key.

At the end of the experiment, typing-performance statistics are displayed
(and appended to the info file): percent correct keystrokes (movements ignored)
and typing speed in characters per second (cps) for correct keystrokes — mean,
P10, P50 and P90.

Running
-------

    go run examples/Typing-Speed/main.go -s 1        # from the repo root
    cd examples/Typing-Speed && go run . -w -s 1     # windowed

Standard flags apply: `-w` (windowed), `-d N` (display), `-s <id>` (subject).
Example-specific flags:

| Flag | Meaning |
|---|---|
| `-file <path>` | A text file with one target sentence per line (overrides the built-in list; blank lines skipped). |
| `-n <int>` | Number of trials to run (`0` = all available; default `0`). |

Design decisions
----------------

- **Advance condition** — a trial ends only when the typed text is an *exact*
  copy of the target and ENTER is pressed. A non-matching ENTER is recorded (as
  a movement) but ignored, and a hint is shown.
- **Editing model** — linear: characters are appended at the current position
  and BACKSPACE removes the last one. Arrow/Delete/Home/End keys are logged as
  movements but do not edit the text.
- **Timing** — keystroke timestamps come from the SDL3 hardware event clock
  (`KeyboardEvent.Timestamp` / `TextInputEvent.Timestamp`), the same reference
  frame as the target-onset flip, so onsets and inter-key intervals are
  hardware-precise rather than wall-clock estimates. Auto-repeat events are
  ignored so only genuine key onsets are counted.
- **cps** — for each *correct* keystroke that has a preceding keystroke in the
  same trial, instantaneous speed is `1000 / interval_ms`; the summary reports
  the mean and the P10/P50/P90 of those values. (The first keystroke of each
  trial is excluded, as its interval is a time-to-first-keystroke latency, not a
  typing rate.)

Data columns
------------

One row per keystroke (`subject_id` is prepended automatically):

| Column | Meaning |
|---|---|
| `trial` | 1-based trial number. |
| `keystroke` | 1-based index of the key within the trial. |
| `onset_ms` | Milliseconds from the trial's target onset. |
| `interval_ms` | Milliseconds since the previous keystroke in the trial. |
| `input` | The character typed, or the key name for movement keys. |
| `category` | `correct`, `incorrect`, or `movement`. |
| `position` | Cursor index at which the key was applied. |
| `expected` | Target character expected at that position (empty if beyond the target). |
