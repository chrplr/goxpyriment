# test_onset_marker

Checks the library's onset marker — the photodiode square and the TTL code
that `control.MarkerFlags` / `exp.Marker` put on a marked onset — on a real
display, in software, with no instrument attached.

```bash
go run ./tests/test_onset_marker        # 1024x768 window
go run ./tests/test_onset_marker -f     # fullscreen
```

The TTL device is a stand-in that logs every write with its SDL timestamp, and
the square is read back from each finished frame just before it is presented
(`Screen.SetPrePresentHook`). The run takes a few seconds, needs no input, and
prints PASS/FAIL per check (also written to the data file); the exit status is
1 if any check failed.

| Check | Expected |
|---|---|
| `Arm` + `ShowTS` | square white on the onset flip and the next 2, black on the 4th; `ShowTS` presents exactly 4 frames |
| code write | 0 ≤ (write − onset flip timestamp) < 1 ms |
| clear | the lines go back to 0 after the TTL duration (3 frame periods), within 5 ms |
| `ShowFrames(v, 2)` | exactly 2 frames: it never holds |
| `Arm(0)` | square only, no TTL write |
| stream armed from its `FrameCallback` | one code per element, each written < 1 ms after that element's `TimingLog.OnsetNS`; 3 white frames per element |
| `Fire` | code written at once and cleared; no white frame |
| two onsets one frame apart | the flashes merge (`Merged` = 1), the second code overlaps the first (`Overlaps` = 1), one clear after the second |
| `.gv` video, frames 0, 10 and 32 marked with `GvMark` | each code < 1 ms after that frame's logged `FlipNS` and cleared after the TTL duration; the square white on the 3 refreshes from each marked frame's first refresh (the clip is 30 fps, so each frame is held for 2 refreshes at 60 Hz), and the last frame held until the square is black |

Measured on the development laptop (Intel Arc, Wayland, windowed, 60 Hz, 4
runs): all 19 checks passed every time; codes were written 0.7–1.3 µs after
the flip timestamp and cleared 50.0–51.0 ms after the code (TTL duration 49.97
ms; the clearing timer's slack is about a millisecond). With the `.gv` check
added (27 checks, 3 runs): all passed; the video codes were written 0.8–3.6 µs
after their frames' flips, and the square was white on refreshes 0–2, 20–22
and 64–66 of 68 (33 frames × 2 refreshes, plus 2 to finish the last mark).

`wedges.gv` is a copy of `examples/demo_play_gv_videos/wedges.gv`, embedded so
the program runs from any directory.

What this does **not** measure: a real device's write latency (DLP-IO20 `Send`
is ~3.5 ms, the DLP-IO8 ~1 ms) and when the square's photons reach the panel
(one to three frames after the flip, depending on the display stack). Those
need the hardware: a scope on the TTL line and a photodiode on the corner.
