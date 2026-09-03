# TODO

## Re-test fullscreen rendering on a Raspberry Pi

The docs used to claim that on a Raspberry Pi (Ubuntu 25.10 + GNOME/Wayland)
fullscreen mode "renders nothing (gray screen)" while windowed mode worked, and
prescribed `SDL_RENDER_DRIVER=software SDL_VIDEODRIVER=wayland` as a workaround
(with an `examples/run_pi.sh` wrapper). That claim was never re-verified and has
now been removed from `CLAUDE.md`, `docs/LinuxVirtualConsoleSDL.md`,
`docs/Fullscreen_high_pixel_density.md`, `tests/Timing-Tests/run-timing-tests.sh`
and `apparatus/screen_newscreen_notjs.go`; `examples/run_pi.sh` is deleted.

**It may have been the clear-only-frame bug all along.** That bug — a frame
carrying no draw calls is not reliably scanned out under a compositor — produces
exactly this symptom (a stale/blank fullscreen image while the program reports
everything fine), appears only when a compositor is running (the Pi had Wayland),
and is renderer-independent, which would explain why switching to the software
renderer changed the behaviour without anyone understanding why. It was fixed in
`apparatus.Screen` in July 2026; see the "Never present a frame with no draw
calls" section of `apparatus/CLAUDE.md`.

**Half-answered on 2026-08-09**: a full `Timing-Tests -test av` run on a Pi 4
(`tests/Timing-Tests/report-rpi4/`) rendered fullscreen throughout — V3D 4.2.14.0,
`video_driver: kmsdrm`, `renderer: opengl`, 1920x1080 at 60 Hz — and produced
photodiode and microphone traces. So fullscreen is not broken on Pi hardware.

But that run had **no compositor**: kmsdrm talks to the display directly, and the
clear-only-frame bug appears only under a compositor. The original claim was
Ubuntu 25.10 + GNOME/Wayland, which is exactly the configuration still untested.

What remains, on a Pi running a compositor:

1. `go run ./examples/demo_hello_world` fullscreen, unmodified. If it renders,
   the old claim is stale and nothing further is needed.
2. `go run ./tests/test_clear_only_frames` — unguarded should fail on an affected
   system, `-guarded` should pass. If unguarded fails on the Pi, the Pi was
   hitting the same bug and the library fix covers it.

Record the Pi model, OS version, kernel, and compositor. If a genuine Pi-specific
fullscreen problem survives both, document it with that evidence rather than
restoring the old text.

## Test frame pacing on a 480 Hz monitor

`apparatus.paceToFrame` sleeps down to the last 2 ms of the frame and spins only
that, so a 60 Hz present loop sits at ~10% CPU duty instead of ~100%. The reason
that matters is the kernel's real-time throttle: `control` requests SCHED_FIFO 50
by default, and a real-time thread at 100% duty is suspended for 50 ms once a
second on a loaded host (measured: 24 stalls in 25 s under `stress-ng`, 51.0 ms
each, one per second exactly; 0 stalls in 25 s idle). See the warning at the end
of `docs/SettingPriorityUnderLinux.md`.

**But it does not sleep when under 3 ms of the wait remains**, and a 480 Hz frame
is 2.083 ms — shorter than the tail alone. Above roughly 330 Hz this reverts to a
pure spin by design, because a sub-millisecond sleep can overshoot the frame
boundary by more than it saves (worst `time.Sleep` overshoot measured here:
0.734 ms at SCHED_FIFO 50). A missed frame is a worse failure than a busy CPU.

So on a 480 Hz panel the exposure comes back — but only in combination with a
driver whose `SDL_RenderPresent` does not block. That combination has not been
observed; a fast panel is usually on a driver that blocks properly, in which case
the wait is empty and none of this is reachable. Untested because there is no
480 Hz monitor to hand (noted 2026-08-08).

To test, when one is available:

1. `go run ./tests/test_vsync_blocking` fullscreen on the 480 Hz panel. The
   verdict is the whole question. **BLOCKING** means the wait is empty, the spin
   runs zero iterations, and nothing here is reachable at any refresh rate —
   stop, there is no problem. **NON-BLOCKING** means the pacing spin covers a
   full 2.083 ms frame at ~100% duty and the throttle applies.
2. If NON-BLOCKING, confirm the stalls before believing they matter: run an
   experiment under `stress-ng --cpu 0` and look for frame intervals near 50 ms.
   Note that `test_vsync_blocking`'s `short N/M` counter will *not* show them —
   it counts intervals shorter than 0.9x nominal, and a throttle stall makes an
   interval long, not short. The data file has what the summary does not:
   `test_vsync_blocking` writes every paced interval to the CSV as
   `frame,paced_interval_ms`, so a 50 ms stall is recoverable by reading the
   column. Printing its maximum alongside the median would save that step.
3. If the stalls are real, the fix is not in `paceToFrame` — pick one of
   `sysctl kernel.sched_rt_runtime_us=-1` on that machine, `-no-realtime`, or a
   display mode whose present blocks.

Record the panel, refresh rate, GPU driver, compositor and session type
(X11/Wayland), the three numbers `test_vsync_blocking` prints, and whether the
window was fullscreen or windowed — the blocking behaviour depends on all of
them.

## triggers/dlpio8.go and dlpio20.go duplicate published modules

Both files carry their own copy of the device protocol and their own
`go.bug.st/serial` calls. Standalone clients for the same two devices are now
published and verified against the hardware:

- [github.com/chrplr/dlpio8](https://github.com/chrplr/dlpio8)
- [github.com/chrplr/dlpio20](https://github.com/chrplr/dlpio20)

Keeping the copies was a deliberate choice, not an oversight: `triggers/` needs
`OutputTTLDevice`/`InputTTLDevice`, an 8-line window over a 20-channel device,
`AutoDetect*` returning a null device, and it must be excluded from `GOOS=js`
builds. None of that belongs in a device library. But two implementations of one
protocol will drift, and the drift will be found the hard way, so the cost is
recorded here rather than left implicit.

If it is ever revisited, the shape is a thin adapter: `triggers/dlpio20.go`
becomes a type embedding `*dlpio20.Device` and satisfying the TTL interfaces,
with the window mapping staying here. The blockers are that it adds a dependency
to the library module and that `tests/test_dlpio20` and `tests/test_dlpio8` must
be re-run against real hardware to confirm nothing regressed — neither is hard,
but neither is free.

One concrete thing already fixed upstream and worth copying if these files are
touched: resolving the port by globbing `/dev/serial/by-id/*DLP*` matches *both*
devices, so with an IO8 and an IO20 attached the lookup is ambiguous and fails.
Match `*DLP-IO8*` or `*DLP-IO20*` instead. Note also that DLP ships these
modules with a fixed USB serial number, so by-id can never distinguish two of
the same model.

## Movie players 

- improve the movie player for gv format: the gv file should be read
  progressively from the disk in a goroutine, if possible (but beware lz4
  compression).

## Eye tracking: run the EyeLink bridge against real hardware

The `eyetracker/` package and `eyetracker/bridge/eyelink_bridge.py` were written
on 2026-08-31 and have never seen a tracker. Everything below is the first
session with the Host PC.

**Why there is a bridge at all.** SR Research publishes no network protocol: the
only supported API is the C library, wrapped as pylink. Rather than take on CGo
for one device, the SDK runs in its own process and speaks line-delimited JSON
over loopback. See `eyetracker/CLAUDE.md`.

### Do these in order

1. **Confirm pylink imports on the Display PC.** This is the step that can waste
   the whole session, so it comes first:

       python3 eyetracker/bridge/eyelink_bridge.py --check --tracker-host 100.1.1.1

   It reports whether pylink is importable and whether the Host answers, then
   exits. If pylink is missing, install the EyeLink Developer Kit before
   anything else.

2. **Wire a TTL output line to the Host PC's parallel port**, and check the Host
   is configured to keep `INPUT` events in the EDF. The bridge asks for this at
   open (`file_event_filter` includes `INPUT`), but that request has never been
   verified against a Host. If the edges do not appear in the EDF, this is the
   first thing to look at.

3. **Run the test**, bridge in one terminal and experiment in another:

       python3 eyetracker/bridge/eyelink_bridge.py --tracker-host 100.1.1.1
       go run ./tests/test_eyelink -s 999 -trigger parport -device /dev/parport0 \
           -trials 50 -fetch /tmp/goxtest.edf

4. **Read the EDF.** `edf2asc /tmp/goxtest.edf`, then compare each `INPUT` line
   against the `MSG` that follows it. Each trial fires a TTL on the flip thread
   immediately after the onset flip, then sends the same event as a message
   through the bridge. Both land in the same file on the Host's clock, so the
   gap between them *is* the bridge's latency, measured rather than inferred.
   Record it: it is the number that decides whether a message can ever time a
   stimulus, and the expected answer is no.

### What to expect, and what would be surprising

The only figures so far are against the simulator on this machine, 5 trials,
windowed, with no TTL device attached: flip to TTL raise 7.9 us median (call
overhead, nothing more), bridge round trip 219 us median. Both should be worse
against hardware. A bridge round trip in the low milliseconds is normal and
harmless; a `MSG`-minus-`INPUT` gap in the EDF of more than a frame is the
result that matters, and it is the argument for never marking onsets over the
link.

`Sync` is called before and after the run, so the change in offset over the
session gives the tracker-versus-local clock drift. If it is large enough to
matter, alignment has to be interpolated rather than applied as a constant.

### Known gaps, in the order they will bite

**The pylink calls are unexercised.** `EyeLinkTracker` in the bridge is written
from the documented API and has never run. It is the part to distrust when
something fails; the Go side is covered by tests, including one that drives the
real script in `--simulate` (`TestAgainstPythonBridge`).

**Calibration graphics.** `doTrackerSetup` needs somewhere to draw its targets,
pylink's built-in graphics are not available everywhere, and goxpyriment owns
the display. `-calibrate` reports the problem and carries on rather than opening
a second window or hanging on a blank screen. Gaze *positions* are then
meaningless; every timing figure above is unaffected, since none of them depends
on where the eye is. The fix is a calibration routine drawn with `stimuli/` that
reports target positions back over the protocol -- not written.

**Nothing is wired into `control`.** There is no `exp.Tracker`, and the data file
gets the bridge identity and the clock offsets only because
`tests/test_eyelink` writes them itself. Worth doing once the hardware path is
known to work, not before.

### Other trackers

`Tracker` is deliberately vendor-neutral and the protocol is not
EyeLink-specific, so a second tracker is a new bridge script rather than a new Go
package. **Tobii Pro now exists** (see the next section), and adding it needed no
new Go package, which is the first real test of that claim. GazePoint (Open Gaze,
XML over TCP) and Pupil Labs both publish socket protocols and could be pure-Go
clients with no bridge process at all.

## Eye tracking: run the Tobii bridge against real hardware

`eyetracker/bridge/tobii_bridge.py` and `tests/test_tobii` were written on
2026-09-03 and have never seen a tracker. The Go client, the protocol and the
Python are covered by `TestAgainstTobiiBridge`, which drives the real script in
`--simulate`; the `tobii_research` calls are not covered by anything.

**Warn the participant/operator before each of these — they drive the rig.**

### Do these in order

1. **Confirm the SDK imports and the tracker is found.** This is the step that
   can waste the whole session, so it comes first:

       python3 eyetracker/bridge/tobii_bridge.py --check

   It prints model, serial, firmware, current and available sample rates, eye
   tracking modes, display area and capabilities, then exits.

   The SDK is a native extension and is **not** pip-installed, but it is already
   importable here: `~/.bashrc:227` puts `~/tobii_eyetracker_pythonlib` on
   `PYTHONPATH`, and it loads under the system `python3` (3.12) — it does *not*
   need the 3.10 `~/eyelink/` venv that pylink uses. Verified 2026-09-03:
   `tobii_research: importable, SDK version 2.1.0.1`, then `no eye tracker
   found` with nothing attached. On another machine, set `PYTHONPATH` yourself.

2. **Settle the coordinate origin.** The conversion is `x_px = nx*width`,
   `y_px = ny*height`, which assumes normalized (0,0) is the display area's
   TOP-LEFT. Tobii's published documentation says so; the headers shipped with
   the SDK do not, so it is an assumption:

       python3 eyetracker/bridge/tobii_bridge.py --edf-dir /tmp     # terminal 1
       go run ./tests/test_tobii -s 999 -calibrate -corners         # terminal 2

   It shows a target in each corner and the centre, averages the gaze during
   each, and prints the measured normalized position beside the expected one —
   then compares the residual against a mirrored expectation and states the
   verdict. **Put its output in the commit that closes this**, and update the
   note in `eyetracker/CLAUDE.md` and the module docstring, which both currently
   say "assumed". If Y comes back mirrored, fix `gaze_events` and the
   `_row` pixel columns in `tobii_bridge.py` before trusting any recording.

3. **A calibrated run.** goxpyriment draws the targets — the Tobii SDK draws
   nothing:

       go run ./tests/test_tobii -s 999 -calibrate -gaze -trials 50 \
           -fetch /tmp/goxtest_tobii.tsv

   Check: the live gaze dot tracks the eye and is not mirrored; the calibration
   summary in the `-info.txt` names no target with `used == 0`; and the status
   is `calibration_status_success` rather than a `_left_eye`/`_right_eye`
   monocular partial, which the program warns about because a run analysed as
   binocular afterwards would be wrong.

4. **Measured sample rate.** The program counts sample events over a measured
   interval and reports the count and the duration with the rate. Compare with
   `get_gaze_output_frequency()` from step 1, remembering that the bridge emits
   one event **per eye**, so a 600 Hz binocular tracker gives ~1200 events/s.
   `Dropped()` must be 0: at 1200 Hz binocular the socket carries 2400 events/s
   against a client buffer of 8192 samples, i.e. about 3.4 s, so draining once
   per trial is fine and once per block is not.

5. **Clock offset and drift.** `Sync(20)` before and after. Report `DeltaMs`
   with `BestRTT/2` as its uncertainty, and the change across the session as the
   drift.

### The clock question this is really testing

`tr.get_system_time_stamp()` is `CLOCK_MONOTONIC` in microseconds — measured on
this machine, mean offset −3.2 µs against `clock_gettime(CLOCK_MONOTONIC)`,
n=2000, min −19.8, max +4.7 µs. `CLOCK_REALTIME`, `CLOCK_MONOTONIC_RAW` and
`CLOCK_BOOTTIME` are each off by a large constant.

If `sdl.TicksNS()` is also `CLOCK_MONOTONIC`, then with the bridge on the display
machine the tracker clock and goxpyriment's clock are the same counter with
different origins: the offset is a *constant*, measurable to sub-µs by one call
pair, with no round trip and no drift term. That would be a strictly better
timing story than the EyeLink's two free-running oscillators.

**Which clock SDL uses here is not measured, and this must not be assumed.**
`CLOCK_MONOTONIC_RAW` drifted against `CLOCK_MONOTONIC` by 662 µs over that same
2000-call run — NTP slewing, and enough to matter over a session. So the
measurement to make is: after `Initialize()`, interleave `control.TicksNS()`
with `clock_gettime(CLOCK_MONOTONIC)` and `(..._RAW)` a few thousand times and
report mean and spread against each, with n. Until that is done, quote `Sync`'s
offset with its `BestRTT/2` uncertainty exactly as for the EyeLink. A result
agreeing with the prediction above needs the same scrutiny as one that does not.

### Known gaps, in the order they will bite

**The `tobii_research` calls are unexercised.** `TobiiTracker` is written from
the SDK source read on this machine and has never run against a device. It is
the part to distrust when something fails.

**No TTL input.** `EYETRACKER_EXTERNAL_SIGNAL` is Tobii's TTL stream and it
timestamps the edge *at the tracker*, which makes it the right way to mark a
stimulus onset — the same argument as the EyeLink Host PC's parallel-port
`INPUT` events, and the reason `Mark` must never carry an onset whose timestamp
is the measurement. Not wired, not measured.

**Not wired into `control` beyond calibration.** `Experiment.CalibrateTracker`
exists, but there is still no `exp.Tracker`: the data file gets the bridge
identity, the pupil unit and the clock offsets only because `tests/test_tobii`
writes them itself.

**Eye images, user position guide, notifications, time-sync data and eye
openness are all unimplemented.** The subscription constants are listed in
`eyetracker/CLAUDE.md`; none is needed to record gaze.

## Fix the COOP/COEP response-header rule on Cloudflare

`downloads.pallier.org` does not send `Cross-Origin-Opener-Policy: same-origin`
and `Cross-Origin-Embedder-Policy: require-corp`, so the published browser
experiments are **not cross-origin isolated**: SDL timestamps tick at ~100 µs
instead of ~5 µs. Nothing is broken by this — 100 µs is still far finer than any
behavioural response, and each launcher page shows a banner saying so — but the
published RTs are coarser than they need to be.

A rule was created on 2026-08-29 and is not taking effect. Verified absent on
every path, including `/builds/{sha}/index.html` and the `.wasm` files, with
`cf-cache-status: DYNAMIC` — so it is not path scoping and not stale cache:

```
curl -sI https://downloads.pallier.org/builds/latest/index.html | grep -i cross-origin
```

Candidates, in order of likelihood:

1. **Wrong rule list.** *Modify Response Header* lives under Rules → Overview →
   **Response** Header Transform Rules, a different list from the Redirect Rules
   where the working directory-index rule sits. A request-header rule would set
   the headers on the way to the origin, where they do nothing.
2. **The expression never matches.** Loosen it to just
   `http.host eq "downloads.pallier.org"` with no path condition; if the headers
   then appear, it is the expression.
3. **Saved but not deployed** — Cloudflare keeps a draft until Deploy is hit.

The intended rule is recorded in `docs/copy_apps_to_cloudflare_R2.md` under
"Cross-origin isolation for the browser builds". Once it works, the banner on
any experiment page disappears, which is the quickest visual confirmation.

## Browser/WASM port

The port is **complete** (2026-07-13): goxpyriment experiments build with
`GOOS=js GOARCH=wasm` and run in a browser tab — rendering (60 Hz
requestAnimationFrame-locked flips), keyboard input with ~5 µs-resolution
timestamps, audio feedback, URL-parameter session setup (`?s=3&w`), and CSV
download — verified end-to-end with `parity_decision`, both headlessly and
interactively. `docs/WASM.md` is the authoritative documentation (build
commands, measured timing numbers, browser caveats, verification recipes).
Quick start: `make wasm-parity_decision-serve` → http://localhost:8080/?s=1

Open items:

- **Decide:** PR the fork's js/wasm work upstream to Zyko0/go-sdl3, or keep
  maintaining `github.com/chrplr/go-sdl3-wasm`.
- Photodiode/BBTK validation of actual pixel onset in the browser on real
  hardware (the 60 Hz / zero-dropped-frames numbers come from headless
  Chrome's virtual compositor).
- Run more examples in the browser (Number-Comparison,
  Contrast-Detection-QUEST, …) — each may surface a few more stubbed
  go-sdl3 js bindings (`go run ./cmd/gen-wasm-exports/` prints the
  worklist; the fork's COVERAGE.md tracks per-function status).
  Done so far: parity_decision, Stroop_task (2026-07-13; Stroop needed one
  un-stub, TTF_GetFontSize).
- Canvas-fills-viewport option in the js `NewScreen` (fixed 1024×768 now).
- HTML-form replacement for `GetParticipantInfo` on js (experiments calling
  it directly can't run in a browser; `NewExperimentFromFlags` users are
  unaffected).

## Vocal-response Stroop + browser voice recording (planned, investigated 2026-07-13)

The real Stroop task is *naming* the ink color — a vocal response. Plan: a
`Stroop_vocal` example using the existing voice-key infrastructure, working
both on desktop and in the browser. Feasibility was investigated on
2026-07-13; the browser side is ~three small pieces of work away, not a new
subsystem.

### What already exists

- Desktop infrastructure is complete: `apparatus.Microphone` (SDL recording
  stream via `AUDIO_DEVICE_DEFAULT_RECORDING.OpenAudioDeviceStream`),
  `apparatus.VoiceKey` (RMS-threshold onset detection, pure Go),
  `apparatus.WriteWAV` (saves the utterance with cue markers at the detected
  onset), `exp.OpenMicrophone`. `examples/picture_naming` is the working
  template.
- The Emscripten SDL3 blob in the fork **has microphone capture compiled
  in**: the full `getUserMedia` → `createMediaStreamSource` →
  `ScriptProcessorNode` pipeline is present in `sdl.js`, including SDL's
  silence-buffer fallback while the browser permission prompt is pending.
  All needed C symbols are exported (`_SDL_GetAudioStreamData`,
  `_SDL_GetAudioStreamAvailable`, `_SDL_GetAudioRecordingDevices`). No
  Emscripten rebuild needed.
- `VoiceKey` needs **zero changes** for js: pure-Go DSP, and its poll loop
  sleeps when idle so it yields to the browser event loop (same pattern as
  `Sound.Wait`).
- Everything `Microphone` calls except sample readout is already enabled on
  js (`OpenAudioDeviceStream`, `Clear`, `Resume/PauseDevice`, `TicksNS`).

### Work items

1. **Fork: un-stub 2–3 js bindings** (~1 h).
   - `iGetAudioStreamData` — the essential one: reads captured PCM out of
     the wasm heap into a Go buffer. Mirror image of the already-working
     `iPutAudioStreamData`: allocate a wasm-heap buffer (`_malloc`), call,
     copy back with `internal.GetByteSliceFromJSPtr`, free. Length args are
     `size_t` = i32 on wasm32 (not BigInt).
   - `iGetAudioStreamAvailable` — trivial (handle in, i32 out).
   - `iGetAudioRecordingDevices` — only needed by the `DeviceNames`
     enumeration helper; the default-device path works without it. Optional.
2. **`WriteWAV` platform split** (~1–2 h). It currently writes via
   `os.CreateTemp`/`os.Rename`, which fail on js. Refactor the encoder to
   target an `io.Writer`, then: desktop wrapper keeps the current
   file-path + atomic-rename behaviour; js wrapper triggers a browser
   download of the bytes (same pattern as `results/output_file_wasm.go`'s
   CSV download, but binary — feed a `Uint8Array` to the `Blob`). Cue
   markers survive unchanged (they are part of the encoded bytes).
3. **`examples/Stroop_vocal`** (~1–2 h): colored word → `vk.Arm()` →
   `exp.ShowTS` → `vk.WaitOnset` → vocal RT = onsetNS − flipTS; save each
   trial's WAV (onset cue embedded) for offline response scoring; log RT +
   congruency in the CSV. Verify desktop first, then headless Chrome, then
   interactively with a real microphone.

### Browser caveats to document with the example

- **Onset RTs carry a roughly constant offset.** The sample-index → time
  mapping (`captureStartNS + N/rate`) stays valid, but the browser input
  pipeline (mic → getUserMedia → ScriptProcessor, ~2048-sample quanta
  ≈ 43 ms at 48 kHz, plus device/OS latency) delays samples entering the
  stream. Within a session the offset is near-constant, so condition
  *differences* (the Stroop effect proper) survive; absolute RTs are
  shifted. Same class of caveat as browser audio output.
- **Permission + secure context**: opening the microphone fires the
  browser's permission prompt (once, at `OpenMicrophone` — do it during
  setup, not mid-trial), and getUserMedia requires localhost or HTTPS
  (`wasmsdl serve` on localhost qualifies).
- **Headless verification is possible**: Chrome's
  `--use-fake-device-for-media-stream --use-fake-ui-for-media-stream`
  provides a synthetic tone-emitting microphone with the permission dialog
  auto-granted, so the CDP harness can exercise the whole chain including
  onset detection.

## Presentation latency: one to two frames deeper than Psychtoolbox

Measured 8 August 2026, photodiode against the TTL, same panel throughout:

    bare Xorg + openbox, exclusive fullscreen   35.74 ms   sd 0.083
    KMS/DRM, no display server                  18.91 ms   sd 0.113
    Xorg - KMS/DRM = 16.826 ms = 1.010 frames

So `ShowTS` returns one frame before the photons on bare hardware and two frames
before them under X. Bridges et al. (2020) measure 2.35-7.10 ms for every Linux
and Windows package they tested, PsychToolBox on Ubuntu at 4.53 -- their flip
returns essentially at scanout.

This costs nothing scientifically: it is constant to 83 us, so it subtracts out
of any analysis, and the precision on that stack is the best in their table. But
it is a software property, not a hardware one, and it is the only figure in this
whole investigation that looks reducible. Worth understanding before deciding
whether to change anything -- likely candidates are the GL swap-chain depth and
whether SDL is being given the chance to page-flip rather than blit.

Do not "fix" it without re-measuring: a change that lowers the mean and raises
the variance would be a straight loss.

**Someday: see the branch `todo-ptb-swap-completion`.** It records, read from
the Psychtoolbox sources on 22 August 2026, how their flip manages to return at
scanout: `glXSwapBuffersMscOML` schedules the swap against a specific future
vblank, then `glXWaitForSbcOML` blocks until that swap has *completed*. Ours
returns when the driver will accept the *next* frame, which is the whole
one-to-two frame difference. The branch also sketches an untested,
CGo-free way to close it on KMS/DRM using the `DRM_IOCTL_WAIT_VBLANK` reader we
already have in `vblank/drm_linux.go`.

**A third route, cheaper than both, and we have never tried it: SDL's own
`SDL_VIDEO_DOUBLE_BUFFER` hint.** Its documented purpose is exactly the frame at
issue -- the default triple-buffer scheme "wastes no CPU time on waiting for
vsync after issuing a flip, but introduces a frame of latency". No CGo, no new
dependency, no code of ours in the path: it is an environment variable, or one
`sdl.SetHint` next to the `HINT_RENDER_DRIVER` call in
`apparatus/screen_newscreen_notjs.go`. Verified 30 August 2026: `DOUBLE_BUFFER`
appears nowhere in this repository outside `vendor/`, and the only two `SetHint`
calls we make are the Vulkan render driver and the audio sample frames.

It is likely to be live on the stack we measured. The archived kmsdrm captures
record `sys renderer: opengl` with `video driver: kmsdrm` -- so despite the
Vulkan hint those runs came out on SDL's KMSDRM **GLES** swap path, which is the
one that reads this hint. The string is present in the bundled SDL3
(`vendor/github.com/Zyko0/go-sdl3/bin/binsdl/assets/sdl_amd64.so.gz`) and KMSDRM
is built in. What is *not* established is that SDL3's KMSDRM path still consults
it the way SDL2's did -- `strings` on the blob cannot show that.

**How to settle it:** two arms of `tests/test_photodiode_latency` on the bare
console, differing only by `SDL_VIDEO_DOUBLE_BUFFER=1` in the environment.
`-diode topleft -s 1 -no-prompt`, `SCHED_FIFO 50`, AD3 capture. Prediction: the
double-buffered arm is about one frame lower, landing near the 4.9-6.0 ms
residual below. Report `PacingStats` and the frame-interval distribution for both
arms -- double buffering makes a dropped frame cost a whole frame of throughput,
and the warning above applies: a change that lowers the mean and raises the
variance would be a straight loss.

### Desk check: subtracting the queue depth (30 August 2026)

Arithmetic on Table `tab:onset` in `paper/goxpyriment_paper.tex`, no new
measurement. If the mechanism above is right, then

    flip -> photons  =  n x frame  +  residual

where `n` is the presentation queue depth in frames and the residual is
everything below the page flip: scanout from the top of the panel down to the
diode's row, plus the panel's rise to the 10 % criterion. The residual is a
property of the *panel and the diode position*, so it should be the same for
two runs that share both, whatever the stack above does.

Shared Dell 1905FP, nominal 60.0197 Hz -> 16.6612 ms:

| run | flip->photons | n | residual |
|---|---|---|---|
| W5700, kmsdrm | 22.61 ms | 1 | 5.95 ms |
| 5490, kmsdrm | 22.28 ms | 1 | 5.62 ms |
| W5700, X11 | 54.85 ms | 3 | 4.87 ms |
| 5490, Wayland | 25.25 ms | 1 | 8.59 ms |

**The two kmsdrm rows agree to 0.33 ms across two different machines and GPUs on
one panel.** That is the load-bearing result, and 0.33 ms is smaller than the
panel's own session-to-session variation: the same monitor rose (10-90 %) in
16.8 ms in one of these sessions and 10.1 ms in the other
(`app:panel`), which moves the 10 % criterion by about 0.7 ms on its own.

**Within a machine the panel and the diode cancel exactly**, so those
differences are the cleanest test of the frame counting:

    W5700   X11     - kmsdrm =  32.24 ms = 1.935 frames
    5490    Wayland - kmsdrm =   2.97 ms = 0.178 frames

X11 costs two whole frames, 1.08 ms short of exactly two -- and the W5700
kmsdrm run is the one measured before the panel had settled, its rise climbing
0.6 ms across the run, so a shortfall of that size is expected rather than
anomalous. **Wayland does not cost a whole frame**, which the queue-depth
picture alone would not predict: a mailbox present can hand the buffer to the
compositor part-way through a frame. So "each stack layer costs a frame" holds
for X11 and does not hold for Wayland, where the cost is a fraction of a frame
plus most of the jitter.

**What it implies.** Subtracting the queue depth puts the onset at 4.9-6.0 ms
on this panel -- inside the 2.35-7.10 ms band Bridges et al. (2020) report for
every Linux and Windows package they tested, and beside PsychToolbox's 4.53 ms.
That is the quantitative version of the claim above: what separates us from PTB
is the queue depth and nothing else. It is not evidence that closing it is
worth doing -- see the warning about variance above -- only that the accounting
closes.

**Limitation, and it is a real one.** The diode position is not recorded for any
row of `tab:onset`, and the raw captures for those runs are **not in this
repository**: the three archived report directories are 2560x1600, 1536x960 and
1920x1080 render areas, none of them the 1905FP's 1280x1024. `Timing-Tests -test
av` paints five squares (four corners and the centre) and the operator chooses
which one to tape the diode over, so position is an unrecorded free variable
worth up to a full frame (16.7 ms at 60 Hz) -- larger than everything this check
resolves. The agreement between the two kmsdrm rows is therefore the *only*
evidence that the positions matched, which makes this a consistency check and
not a confirmation. Anyone repeating it should record the square.
