# Naming Stroop

A vocal-response version of the classic Stroop (1935) colour-naming task. The participant sees a colour word rendered in a coloured font and must **name the ink colour aloud**, as quickly as possible, ignoring the word itself. A microphone voice key measures the time between word onset and the start of the vocal response.

This is the vocal counterpart to [`examples/Stroop_task`](../Stroop_task), which records the same design with key-press responses instead. See that example's README for background on the paradigm.

---

## Trial structure

```
Fixation cross  →  Blank  →  Colour word  →  Vocal response  →  ITI
    500 ms          200 ms     until voice        (voice key)     500 ms
                                onset or 3 s
                                timeout
```

---

## Design

- 4 ink colours (Red, Green, Blue, Yellow) x 4 word meanings = 16 combinations, shuffled each run
- Congruent: word and ink match (e.g. RED written in red)
- Incongruent: word and ink conflict (e.g. RED written in blue)

---

## Prerequisites

- Go 1.25+
- A working microphone (default input device)

---

## Running

```bash
go run main.go -w -s 1
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | off | Windowed mode (1024x768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |
| `-s N` | 0 | Subject ID |
| `-threshold F` | 0.03 | Voice-key amplitude threshold (0-1, F32LE RMS) |
| `-save-wav` | true | Save per-trial WAV files for offline verification |

---

## Output

Two kinds of file are written to `goxpy_data/` (or the `-output` path if set):

- `Naming Stroop_sub-NNN_date-YYYYMMDD-HHMMSS.csv` — trial data (CSV with a metadata header). One row per trial:

  | Column | Description |
  |--------|-------------|
  | `trial` | Trial number |
  | `word` | The displayed word |
  | `ink_color` | The ink colour |
  | `congruent` | `true` if word meaning matches ink colour |
  | `rt_ms` | Reaction time in milliseconds (voice onset − word onset), or `-1` if no response was detected |
  | `detected` | Whether the voice key detected an onset within the response window |

- `sub-NNN_trial-TT_WORD-INK.wav` per trial (when `-save-wav=true`) — raw F32LE mono 44100 Hz PCM covering the full trial: pre-onset silence, the vocal response, and up to 1000 ms of post-onset audio. Each file embeds a WAV cue marker labelled `onset` at the exact sample where the voice key triggered.

---

## Timing model

Reaction time is measured the same way as in [`examples/picture_naming`](../picture_naming):

```
vk.Arm()              ← mic buffer flushed; recording starts
     |
     |   exp.Screen.Clear()
     |   word.Draw(exp.Screen)
     |   exp.Screen.FlipTS() ───────────── wordOnsetNS  (VSYNC-locked)
     |
     |   [participant sees word, starts speaking]
     |
     └── WaitOnset() detects amplitude threshold ── onsetNS
              |                                        |
              |  screen blanked (ClearAndUpdate)       |
              |                                        RT = (onsetNS - wordOnsetNS) / 1 000 000  [ms]
              |
              └── 1000 ms post-onset recording ── WAV saved
```

`vk.Arm()` is called immediately before the screen flip so the microphone is already capturing when the word appears. Both `wordOnsetNS` (returned by `FlipTS`) and `onsetNS` (computed from the capture start timestamp plus sample count) are on the same SDL3 nanosecond clock, so no cross-clock conversion is needed.

---

## Voice key threshold

The threshold is the minimum F32LE RMS amplitude (0-1) over a 128-sample window (~2.9 ms at 44100 Hz) required to declare a voice onset.

- **Too low**: false triggers from breath noise or lip smacks, giving spuriously short RTs (< 100 ms).
- **Too high**: soft or breathy onsets are missed; `detected = false` in the data.

A value of 0.02-0.05 works well in a quiet room. Calibrate by inspecting the saved WAV files: the true onset should be the first large-amplitude region, with flat (near-zero) signal before it. Typical colour-naming latencies are 500-900 ms for congruent trials and somewhat longer for incongruent trials; very short values (< 200 ms) indicate false triggers.

Always spot-check a random subset of the WAV files after each session in an editor that shows WAV cue markers (e.g. [ocenaudio](https://www.ocenaudio.com), [Reaper](https://www.reaper.fm)), looking for a correct onset position, no double-triggers, and a complete recording. The `apparatus.ScanOnset` function can be used in a post-processing script to re-analyse saved WAVs with a different threshold without re-running the experiment.

---

## References

Stroop, J. R. (1935). Studies of interference in serial verbal reactions. *Journal of Experimental Psychology*, 18(6), 643-662. https://doi.org/10.1037/h0054651
