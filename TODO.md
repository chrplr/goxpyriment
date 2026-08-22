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

## Add support for Eyetrackers

- Eyelink 1000
- Tobii

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
