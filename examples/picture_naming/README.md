# Picture Naming

A picture naming latency task: the participant sees a picture and names it aloud as quickly as possible. The software measures the time between image onset and the participant's vocal onset.

## Usage

```bash
go run main.go -w -s 1
```

Flags:

| Flag | Default | Description |
|---|---|---|
| `-w` | — | Windowed mode (1024×768 instead of fullscreen) |
| `-d N` | primary | Display index |
| `-s N` | 0 | Subject ID |
| `-threshold F` | 0.03 | Voice-key amplitude threshold (0–1, F32LE RMS) |
| `-save-wav` | true | Save per-trial WAV files for offline verification |

## Output

Two files are written to the current directory (or `-output` path if set):

- `Picture Naming_sub-NNN_date-YYYYMMDD-HHMMSS.csv` — trial data with columns `trial`, `label`, `rt_ms`, `detected`
- `sub-NNN_trial-TT_label.wav` per trial (when `-save-wav=true`) — raw F32LE mono 44100 Hz PCM, covering the full trial: pre-onset silence, the naming response, and up to 1 500 ms of post-onset audio. Each file embeds a WAV cue marker labelled `onset` at the exact sample where the voice key triggered; audio editors that support WAV cue chunks (e.g. [ocenaudio](https://www.ocenaudio.com), [Reaper](https://www.reaper.fm)) display it as a named marker on the waveform.

## Timing model

```
vk.Arm()              ← mic buffer flushed; recording starts
     │
     │   exp.Screen.Clear()
     │   picture.Draw(exp.Screen)
     │   exp.Screen.FlipTS() ───────────── imageOnsetNS  (VSYNC-locked)
     │
     │   [participant sees image, starts speaking]
     │
     └── WaitOnset() detects amplitude threshold ── onsetNS
              │                                        │
              │  screen blanked (ClearAndUpdate)       │
              │                                        RT = (onsetNS − imageOnsetNS) / 1 000 000  [ms]
              │
              └── 1 500 ms post-onset recording ── WAV saved
```

`vk.Arm()` is called immediately before the screen flip so the microphone is already capturing when the image appears. Both `imageOnsetNS` (returned by `FlipTS`) and `onsetNS` (computed from the capture start timestamp plus sample count) are on the same SDL3 nanosecond clock, so no cross-clock conversion is needed.

When vocal onset is detected the picture disappears immediately (via `vk.OnOnset`), giving the participant a clear end-of-trial signal. Recording then continues for 1 500 ms so that the saved WAV captures the full naming response, not just the pre-onset silence.

Unlike the shadowing task, there is no audio playback here, so acoustic feedback is not a concern and headphones are not required.

## Voice key threshold

The threshold is the minimum F32LE RMS amplitude (0–1) over a 128-sample window (~2.9 ms at 44100 Hz) required to declare a voice onset.

- **Too low**: false triggers from breath noise or lip smacks, giving spuriously short RTs (< 100 ms).
- **Too high**: soft or breathy onsets are missed; `detected = false` in the data.

A value of 0.02–0.05 works well in a quiet room. Calibrate by inspecting the saved WAV files: the true onset should be the first large-amplitude region, with flat (near-zero) signal before it. Typical picture naming latencies for familiar objects are 600–900 ms; very short values (< 200 ms) indicate false triggers.

## Using real picture files

The demo uses coloured rectangles with text labels as placeholders. Replace them with real images:

```go
// Load from disk:
pic := stimuli.NewPicture("stimuli/apple.png", 0, 0)

// Or embed in the binary:
//go:embed stimuli/*.png
var stimuliFS embed.FS

data, _ := stimuliFS.ReadFile("stimuli/apple.png")
pic := stimuli.NewPictureFromMemory(data, 0, 0)
```

Any SDL-supported image format works (PNG, JPEG, BMP, …). The image is centred at position (0, 0) — the screen centre — by default; call `pic.SetPosition(sdl.FPoint{X: x, Y: y})` to move it.

For timing-critical code, preload all images onto the GPU before the trial loop to avoid lazy-allocation jitter on the first draw:

```go
stimuli.PreloadVisualOnScreen(exp.Screen, pic)
```

## Verifying the saved WAV files

Always spot-check a random subset of the WAV files after each session. Open them in an editor that shows WAV cue markers (e.g. ocenaudio or Reaper) to see the `onset` marker overlaid on the waveform. Look for:

- **Correct onset**: the waveform should be flat for several hundred milliseconds, then show a clear onset aligned with the `onset` marker. If the marker falls within the first 50 ms, the threshold may be too low.
- **No double-trigger**: a single clean onset per file. Multiple peaks suggest the threshold is too low or the participant produced a pre-articulatory breath.
- **Complete recording**: the file should contain the full naming response followed by roughly 1 500 ms of trailing audio. If the response is cut off, increase the `postOnsetMS` argument in the `WaitOnset` call.

The `apparatus.ScanOnset` function can be used in a post-processing script to re-analyse saved WAVs with a different threshold without re-running the experiment.
