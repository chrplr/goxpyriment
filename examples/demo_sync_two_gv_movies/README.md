# Sync Two GV Movies

Plays two `.gv` movies side by side under a shared `media.MasterClock`
to demonstrate the `media` package's Stage 1-4 features:

- **Shared master clock** — both movies advance from the same `Now()`
  reading per frame; no drift accumulates between the left and right
  copies across pause/resume cycles.
- **Burst-pause command groups** — `mgr.BeginBurst()` /
  `mgr.EndBurst()` around the SPACE handler so both `Pause` calls
  observe the same clock value.
- **Movie[At] look-ahead** — `leftMov.OnAt(media.Frame(N), ...)` fires
  from inside the per-frame decode loop, BEFORE the target frame
  appears on screen.
- **Movie[AtDisplay]** — `leftMov.OnAtDisplay(media.Frame(N), ...)`
  fires from `NotifyFlipped` with the post-vsync timestamp, then pulses
  TTL line 0 for 5 ms.
- **Display[Onset/Offset]** — `mgr.OnDisplayOnset("left", ...)` /
  `mgr.OnDisplayOffset(...)` fire when the named tag changes visibility.
- **Mixed compositing** — a fixation cross is drawn between
  `mgr.DrawWithoutFlip()` and `screen.FlipTS()`, sharing the same
  vsync as the movies.
- **Trigger wiring** — `triggers.AutoDetectDLPIO8` falls back to a
  silent `NullOutputTTLDevice` when no hardware is plugged in, so the
  example runs identically with or without an EEG box.

## Running

```bash
# From inside the example directory (uses the bundled test fixtures)
cd examples/sync_two_gv_movies
go run main.go -w

# From the repo root
go run examples/sync_two_gv_movies/main.go -w

# Custom files
go run main.go -fL /path/to/left.gv -fR /path/to/right.gv -w
```

The default left/right paths look for `PhysicalViolation.gv` and
`PhysicalViolation2.gv` in the current directory, then in
`../../tests/test_playgv/`, then in `tests/test_playgv/`.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-fL` | autodetect | Path to left .gv file |
| `-fR` | autodetect | Path to right .gv file |
| `-at` | `30` | `Movie[At]` (look-ahead) target frame on the LEFT movie |
| `-atd` | `60` | `Movie[AtDisplay]` (post-vsync) target frame on the LEFT movie |
| `-s` | `0` | Participant ID |
| `-w` | off | Windowed mode (1024×768 instead of fullscreen) |
| `-d N` | -1 | Display index where the window opens (-1 = primary) |

### Controls

- `SPACE` — burst pause/resume of BOTH movies (atomic)
- `R` — burst seek both movies back to frame 1
- `ESC` — quit

## What you should see in the log

```
loaded LEFT  .../PhysicalViolation.gv: 1280x720, 7290 frames @ 30 fps
loaded RIGHT .../PhysicalViolation2.gv: 1280x720, 7290 frames @ 30 fps
no TTL device detected; trigger calls are silent no-ops
media: display-onset precision is vsync-period (~16.666ms at this refresh rate). Install a sub-ms presentation backend (Stage 5: Vulkan VK_EXT_present_timing or Metal addPresentedHandler) for hardware-verified timing.
Controls: [SPACE] burst pause/resume both, [R] burst-rewind both, [ESC] quit
[Display.Onset]  left  ts=12345678 source=vsync-estimated
[Display.Onset]  right ts=12345678 source=vsync-estimated
[Movie.At]        left frame 30 decoded (look-ahead, ts=... source=look-ahead)
[Movie.AtDisplay] left frame 60 displayed (ts=... source=vsync-estimated)
```

The single precision warning at startup is required by the project rule
against silent timing changes (`CLAUDE.md` §"No silent changes to
runtime behaviour"). It will go silent automatically once a Stage 5
backend (Vulkan `VK_EXT_present_timing` or Metal `addPresentedHandler`)
ships and `Onset.Source` flips to `hardware-verified`.

## Notes

- **Two file handles, one logical movie?** Even when `-fL` and `-fR`
  point at the same file, two `gvvideo.GVVideo` instances are needed
  because each one holds per-instance read position state. The shared
  `MasterClock` keeps them in step regardless.
- **2 GB fixtures.** `PhysicalViolation.gv` and `PhysicalViolation2.gv`
  in `tests/test_playgv/` are large; the gvvideo decoder streams them
  on demand, but cold-loading both still takes a moment.
- **No audio.** The `media` package is silent by design (see
  `media/Plan.md` §1).
