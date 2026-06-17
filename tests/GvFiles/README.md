# WhiteSquareSync (gv)

goxpyriment translation of the PsyScope script
[`WhiteSquareSync_oscill.psyscript`](../../..). Plays sequences of three
`.gv` movies per trial, flashing a white square (photodiode pickup) and
pulsing DLP-IO8 line 0 at the same frame markers PsyScope uses, so a
photodiode + oscilloscope pair can verify display-onset synchrony with
the TTL trigger.

## What it does (per trial)

1. Plays **M1** (.gv movie) to completion.
2. Plays **M2** (.gv movie) to completion.
3. Plays **M3** (.gv movie) to completion.
4. 1000 ms blank inter-trial interval, then advance to the next trial.

For every movie:

- **At frame 1** — show the white square (400 ms) **AND** raise DLP-IO8 line 0.
- **At frame 10** — lower DLP-IO8 line 0.
- (M1 only) **At frame 186** — show the white square (400 ms) **AND** raise the line.
- (M1 only) **At frame 189** — lower the line.

The square is drawn in the same vsync as the target movie frame because
the OnAt look-ahead callback fires from `MovieManager.DrawWithoutFlip`
*before* `Screen.FlipTS`. The TTL pulse is timed by `OnAtDisplay`, which
fires after `FlipTS` with the OS-measured first-pixel-visible
timestamp — same precision contract as PsyScope's `Movie[AtDisplay]`
using Metal `addPresentedHandler`. See
[../../docs/MediaMovies.md §7](../../docs/MediaMovies.md#7-external-trigger-synchronisation-with-frames-on-screen)
for the full timing contract.

## Running

```bash
# From inside this directory (movies are already here)
cd examples/GvFiles
go run . -w                                 # windowed, default trials
go run . -w -trials 3movies.txt             # explicit trials list
go run . -w -n 3                            # cap at 3 trials
go run . -w -dir /elsewhere/with/.gv/files  # different movie folder

# From the repo root
go run examples/GvFiles/main.go -w
```

If a `DLP-IO8-G` USB device is connected, line 0 (= PsyScope pin 1) is
toggled live. Without a device, the trigger calls are silent no-ops and
the visual half of the test still works for photodiode-only validation.

## Trials list

Three sources, in priority order:

1. `-trials FILE` flag (whitespace-separated, 3 columns per line, `#`
   for comments).
2. Default: `<dir>/3movies.txt` (a sample is shipped in this folder).
3. Auto-pair: glob `M1-*.gv`, `M2-*.gv`, `M3-*.gv`, sort each set,
   cycle-pair them. Number of trials = max group size.

`-n N` caps the total at the first `N` trials.

The shipped `3movies.txt` is a 5-trial sample using files already in
this folder; extend or replace as needed.

## Flags

| Flag | Default | Description |
|---|---|---|
| `-dir` | `.` (current directory) | Where to look for `.gv` movies and the optional `3movies.txt` |
| `-trials FILE` | (autodetect) | Explicit trials list path |
| `-n N` | `0` (no limit) | Cap on trial count |
| `-sqW`, `-sqH` | `200`, `200` | White square size in screen pixels |
| `-sqX`, `-sqY` | `0`, `380` | White square center in screen-center-relative pixels (+Y = up) |
| `-nameY` | `320` | Movie-name overlay center Y (+Y = up) |
| `-s` | `0` | Participant ID (standard goxpyriment flag) |
| `-w` | off | Windowed mode (1024×768 instead of fullscreen) |
| `-d N` | -1 | Display index (-1 = primary) |

Place the photodiode over the square's actual on-screen position and
adjust `-sqX` / `-sqY` to match your physical photodiode mount. The
default puts a 200×200 px white square 380 px above screen center — for
a 1080p display, that's roughly the upper-center of the screen.

## Controls

- **Any key** — start the experiment from the "Press a key when ready"
  screen.
- **Q** or **ESC** — quit immediately (mid-trial). Closing the window
  also works. The TTL line is forced LOW on each movie's exit so an
  abort doesn't leave the trigger asserted.

## Data file

Output: `goxpy_data/<expname>_sub-<NNN>_date-<YYYYMMDD>-<HHMMSS>.csv`
(standard goxpyriment data file location).

Columns:

| Column | Meaning |
|---|---|
| `trial` | 1-based trial index |
| `movie_idx` | 1, 2, or 3 (M1 / M2 / M3) |
| `movie` | `.gv` basename |
| `event` | `square_on`, `ttl_high`, `ttl_low`, `movie_start`, `movie_done` |
| `frame` | Target movie frame for this event (0 for `movie_start` / `movie_done`) |
| `ts_ns` | SDL ticks at the event time |
| `source` | `look-ahead` (square triggers), `hardware-verified` / `vsync-estimated` (TTL triggers), `wall-clock` (movie start/end) |

Subtract `ts_ns` values directly to compute inter-event delays — they
all share the `sdl.TicksNS` reference frame. `source = hardware-verified`
on macOS / Linux means the timestamp is OS-measured first-pixel-visible
(sub-ms precision); `vsync-estimated` (Windows fallback) is post-Present
FlipTS (~vsync-period precision).

## PsyScope mapping

| PsyScope construct | goxpyriment equivalent (this script) |
|---|---|
| `Templates: starttemp MovieTemplate` (Weights "1 144") | `showStartMsg` once, then `for trial := range trials { ... }` |
| `Movielist` factor list reading `3movies.txt` | `loadTrials(path)` (whitespace-sep, 3 cols) |
| `Startmsg` event with `SerialOut "1"` / `"Q"` brackets | `showStartMsg`: TTL HIGH on first flip, LOW after keypress |
| `M1` / `M2` / `M3` Movie events with `Duration: Movie[Done]` | `playOneMovie` per movie, exits when `mov.IsDone()` |
| `Conditions[ Movie[ At THISMOVIE "f:N" ] ] => Actions[ RunEvent[squarepic] ]` | `mov.OnAt(media.Frame(N), ...)` |
| `Conditions[ Movie[ AtDisplay THISMOVIE "f:N" ] ] => Actions[ SerialOut "1" ]` | `mov.OnAtDisplay(media.Frame(N), ttl.SetHigh)` |
| `Conditions[ Movie[ AtDisplay THISMOVIE "f:N" ] ] => Actions[ SerialOut "Q" ]` | `mov.OnAtDisplay(media.Frame(N), ttl.SetLow)` |
| `squarepic` Picture event, Duration 400 | `stimuli.NewRectangle(...)` drawn while `now < squareDeadlineNS` |
| `M1Name` / `M2Name` / `M3Name` Text events, Duration 500 | `stimuli.NewTextLine(...)` drawn while `now < nameDeadlineNS` |
| `NewEvent` NULL Duration 1000 (ITI) | `exp.Blank(1000)` after each trial |
| `PortB` SerialOut → DLP-IO8 (or compatible serial TTL) | `triggers.AutoDetectDLPIO8()` (auto-detected; falls back to no-op) |

## Notes

- Movies are played at `ScaleFit` so they fill the screen while
  preserving aspect ratio. To restore the original PsyScope MoviePort
  geometry (1024×768 at fixed offset), swap `media.WithScale(...)` for
  `media.WithSize(1024, 768)` and a `media.WithPosition(...)` of the
  desired offset.
- The white square is opaque (alpha 255) and drawn after the movie
  textures in each frame, so it cleanly overpaints the movie wherever
  it lands. With the default `-sqX 0 -sqY 380` it sits well above the
  movie area on a typical 1080p display, so the square and movie are
  visually separated.
- The TTL pulse pattern matches PsyScope verbatim: `'1'` HIGH on line 0
  at frame 1, `'Q'` LOW on line 0 at frame 10. For M1 the pattern
  repeats at 186 / 189. With a 30 fps movie, that's a ~300 ms pulse
  starting at first frame and a ~100 ms pulse around frame 186.
