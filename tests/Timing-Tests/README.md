# Timing-Tests — quick reference

A hardware timing verification suite for goxpyriment experiments. Run these
**before** collecting data, to characterise your system's display and audio
timing and to verify that stimulus presentation behaves as intended.

For background, equipment setup and interpretation see
**[docs/TimingTests.md](../../docs/TimingTests.md)**.

---

## The console numbers are not the answer

Every statistic these tests print comes from software timestamps — when
goxpyriment *believes* a flip happened. Those stayed textbook-perfect throughout
a presentation bug that left a panel showing stale frames for seconds at a time.

**Judge timing from a photodiode and TTL recording.** Treat the console output as
a cross-check on the software side only.

---

## Recommended workflow

```
No hardware needed
  1. check    — can this machine display and make noise at all?
  2. display  — true refresh rate and frame-interval stability
  3. latency  — audio pipeline delay

Photodiode and/or trigger box
  4. av       — THE stimulus timing test: visual + audio + TTL
  5. vrr      — arbitrary durations, not just multiples of a frame
  6. rt       — reaction-time timestamp precision
```

`av` is the default: running with no `-test` flag runs it.

---

## Running

> **Run every test in fullscreen — that is the default. Do not pass `-w`.**
>
> Windowed mode hands your frames to the desktop compositor, which adds a
> variable presentation delay, may drop or duplicate frames, and on some drivers
> stops blocking on VSYNC altogether. Measurements taken windowed describe the
> compositor, not goxpyriment, and are not comparable between machines.
>
> `-w` exists only for developing and debugging the tests themselves. Any number
> reported from a windowed run should be treated as invalid. If a test drops
> frames windowed but not fullscreen, that is the compositor — re-measure before
> investigating anything else.

```bash
# from the repo root (go.work resolves both modules):
go run ./tests/Timing-Tests [flags]

# examples
go run ./tests/Timing-Tests                                          # av, the default
go run ./tests/Timing-Tests -frames-on 12 -frames-off 18 -cycles 100
go run ./tests/Timing-Tests -no-sound -no-ttl                        # visual only, no hardware
go run ./tests/Timing-Tests -soa-ms -50                              # audio leads by 50 ms
go run ./tests/Timing-Tests -frames-on 1 -frames-off 60 -cycles 60   # single-frame flashes
go run ./tests/Timing-Tests -test check
go run ./tests/Timing-Tests -test display -duration-s 30
go run ./tests/Timing-Tests -test latency -audio-frames 256 -drain-reps 20
go run ./tests/Timing-Tests -test vrr     -vrr-max-ms 50 -cycles 5
go run ./tests/Timing-Tests -test rt      -cycles 60
go run ./tests/Timing-Tests -sysinfo                                 # config snapshot, then exit
```

Results go to `~/goxpy_data/` as a `.csv` plus an `-info.txt` header recording the
display and audio configuration actually in use — including **which monitor** the
window opened on. Read geometry from there rather than assuming; on a
multi-monitor machine the displays differ in every relevant parameter.

---

## The `av` test

Every cycle presents all three modalities at once:

| Modality | What happens |
|---|---|
| Visual | Five squares (four corners + centre) bright for `-frames-on` frames, dark for `-frames-off` |
| Audio | Tone of `frames-on × frame-period`, synced to the visual onset (or offset by `-soa-ms`) |
| TTL | A `-trigger-ms` pulse at the visual onset |

`-no-sound` and `-no-ttl` drop a modality. That is how one test covers what used
to be three: `-no-sound` is a pure visual-onset test, and the audio channel of a
recording gives tone-onset jitter over a long session.

**Placing the photodiodes.** An LCD paints top to bottom, so a bottom square
lights close to a full frame period after a top one. Put one photodiode on a top
square and another on a bottom square: the difference is your panel's scan-out
gradient — on the reference rig, 13.15 ms across 80 % of a 1280×1024 screen, or
15.96 µs per pixel row. Two squares on the same row are microseconds apart and
serve as a sanity check. Each display needs its own calibration.

**Discard the warm-up.** The first several cycles run long before settling
(36–39 ms falling to ~27 ms on the reference rig). `-warmup` excludes them from
the test's own statistics, but offline tools do not know about it — quote
steady-state numbers and say which cycles you used.

---

## Recording with a BBTK

`run-timing-tests.sh` can drive `bbtk-capture` for you, so the stimulus always
falls inside the capture window without two terminals and a stopwatch:

```bash
BBTK_CAPTURE=1 BBTK_CAPTURE_BIN=~/00_git/bbtkv3/_build/bbtk-capture \
    ./run-timing-tests.sh av-visual
```

| Variable | Default | Meaning |
|---|---|---|
| `BBTK_CAPTURE` | off | Set to `1` to record the photodiode steps |
| `BBTK_CAPTURE_BIN` | `bbtk-capture` | Path to the binary, if not on `PATH` |
| `BBTK_PORT` | — | Serial port; read by `bbtk-capture` itself |
| `BBTK_MARGIN_S` | 8 | Seconds recorded either side of the stimulus |
| `BBTK_READY_TIMEOUT_S` | 120 | Give up waiting for the device |

Only the photodiode steps (`av`, `av-gc`, `av-visual`) are wrapped; `check`,
`display` and `latency` measure nothing the BBTK can see.

**Why it waits.** `bbtk-capture` needs 11–40 s between launch and the device
actually recording — fixed command pacing, plus an internal-memory erase whose
duration depends on whether the box needs a full format. Its own "Capturing
events…" message is printed several seconds *before* recording starts, so the
script instead blocks on the `BBTK-CAPTURE-READY` line that `bbtk-capture` emits
the instant it sends `RUDS`. Budget that startup per step.

Ctrl-C is safe: `bbtk-capture` traps it, stops the capture, and still writes what
the device recorded.

Captures land in `reports-<host>/bbtk-<step>-001-events.csv` alongside the console
reports, with the raw `bbtk-capture` output in `reports-<host>/bbtk-<step>.log`.

---

## Microphone coupling

If you are measuring audio, **max the output volume and put the BBTK microphone
within a few millimetres of the speaker membrane.** Anything less and events are
silently missed — there is no warning, and the run looks like it worked.

Two checks on the resulting `-events.csv`:

- **N(Mic1) should equal N(TTLin1).** Far fewer means bad coupling, not bad
  timing. A run of 197 cycles that yields 6 Mic events is a placement problem.
- **Mic duration should be close to `frames-on × frame period`** (200 ms at the
  defaults). A 20 ms Mic pulse for a 200 ms tone means the threshold is catching
  only the loudest fraction of the tone.

---

## Flags

### Common

| Flag | Default | Meaning |
|---|---|---|
| `-test` | `av` | `av \| vrr \| rt \| check \| display \| latency` |
| `-cycles` | 120 | Number of cycles (`av`, `vrr`, `rt`) |
| `-warmup` | 10 | Leading cycles discarded from statistics |
| `-port` | auto | Serial port for the DLP-IO8-G |
| `-trigger-pin` | 1 | DLP-IO8-G output pin (1–8) |
| `-trigger-ms` | 5 | Trigger pulse duration, ms |
| `-d` | -1 | Display index (-1 = primary) |
| `-w` | false | Windowed mode — debugging only, never for measurement |
| `-sysinfo` | false | Print system information and exit |
| `-gc` | false | Leave the collector running (suspended by default); run twice to measure its effect |

### `av`

| Flag | Default | Meaning |
|---|---|---|
| `-frames-on` | 12 | Bright frames per cycle (12 = 200 ms at 60 Hz) |
| `-frames-off` | 18 | Dark frames per cycle (18 = 300 ms at 60 Hz) |
| `-square-px` | 200 | Side of each of the five squares, renderer px |
| `-level-a` | 0 | Dark luminance 0–255 (surround) |
| `-level-b` | 255 | Bright luminance 0–255 (squares) |
| `-soa-ms` | 0 | Visual-to-audio SOA; negative = audio first |
| `-freq-hz` | 1000 | Tone frequency, Hz |
| `-hz` | 60 | Refresh rate used to derive the tone duration |
| `-no-sound` | false | Do not play the tone |
| `-no-ttl` | false | Do not fire the trigger |
| `-paced-flip` | false | Use `PacedFlip()` instead of `Update()` |
| `-audio-frames` | 0 | Audio buffer, sample frames (0 = SDL default) |

`-audio-frames` sets the floor on audio-onset precision: 256 frames at 44100 Hz
quantises tone onsets to 5.8 ms steps.

### Other tests

| Test | Flags |
|---|---|
| `display` | `-duration-s` (10) |
| `latency` | `-freq-hz` (1000), `-drain-reps` (10) |
| `vrr` | `-vrr-max-ms` (50), `-cycles` |
| `rt` | `-iti-ms` (1000) — mean ITI, jittered ±50 % |

---

## Equipment

| Test | Needs |
|---|---|
| `check`, `display`, `latency` | Nothing |
| `av` | Photodiode + DLP-IO8-G (either alone still works — use `-no-sound` / `-no-ttl`) |
| `vrr` | Photodiode; a VRR-capable display to see any benefit |
| `rt` | Keyboard or USB response box |

A Black Box ToolKit, an oscilloscope, or any multi-channel recorder works for the
photodiode and TTL channels.

---

## Related tests

- [`../test_gv_sync`](../test_gv_sync) — `.gv` playback synchronisation
- [`../test_dlpio8`](../test_dlpio8) — DLP-IO8-G square-wave characterisation
- [`../test_clear_only_frames`](../test_clear_only_frames) — regression test for
  the compositor presentation bug
- [`../test_vsync_blocking`](../test_vsync_blocking) — does `SDL_RenderPresent`
  block on VSYNC here?
