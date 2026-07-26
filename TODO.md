# TODO

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


