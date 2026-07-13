# TODO

## Browser/WASM port

Status snapshot, 2026-07-13. Authoritative details live in `docs/WASM.md`;
this section is the short working list. The go-sdl3 side of the work lives in
the fork `~/00_git/go-sdl3-wasm` (branch `wasm-render-fixes`).

### Done

- **Phase 0 — one source of truth.** `wasm-support` merged into
  `wasm-render-fixes`; the vendor-only panic-removal patches ported into the
  fork; goxpyriment `vendor/` re-synced from the fork (`GOWORK=off go mod
  vendor`). The `sdl.wasm` rebuild recipe is
  `.docker/emscripten-build/Dockerfile` in the fork.
- **Phase 1 — hello_world renders in Chrome** (verified headlessly).
  In-memory IOStream path (`IOFromDynamicMem`/`Write`/`Seek`) for embedded
  fonts; ~30 js bindings un-stubbed (display/system info, cursors, text
  metrics, `HasEvent`, `GetKeyboardState`, …); `SDL_Delay` no-op on js.
- **Phase 2 — parity_decision runs end-to-end in the browser.**
  Instructions → fixation → 10 RT trials with keyboard responses → CSV +
  info-file browser download, driven headlessly via CDP key events.
  Two load-bearing pieces:
  - `sdl.RunLoop` on js was redesigned: requestAnimationFrame only signals a
    channel; the experiment logic runs on the goroutine that called `Run`.
    (Blocking inside a `js.FuncOf` callback wedges the tab's main thread.)
  - Browser session setup: flags come from URL query params (`?s=3&w`,
    `control/platform_js.go`); the participant dialog never opens on js;
    `exp.Audio` gets a zero-Device manager so playback calls are silent
    no-ops instead of nil-pointer crashes.

### Next steps (in order)

1. ~~**Measure event-timestamp granularity**~~ **Done 2026-07-13.** Found and
   fixed a 1 ms quantization of *all* SDL timestamps (SDL fell back to
   `Date.now()` via a rejected `CLOCK_MONOTONIC_RAW` probe; patched
   `emscripten_date_now` in the fork's sdl.js + Dockerfile). Measured after
   the fix: ~100 µs on a plain page, ~5 µs cross-origin isolated; `wasmsdl
   serve` now sends COOP/COEP headers to get the 5 µs clock. Numbers and
   details in `docs/WASM.md` "Timing in the browser".
2. ~~**Phase 3 — frame pacing.**~~ **Done 2026-07-13.** On js,
   `Screen.Update` now presents and parks until the next
   requestAnimationFrame (`apparatus/screen_present_js.go` +
   `sdl.WaitAnimationFrame` in the fork); `PacedFlip`'s busy-wait is a no-op
   on js (it would freeze the tab). Measured: 16.666 ms mean (60.00 Hz),
   SD 0.12 ms, zero dropped frames over 299. Numbers in `docs/WASM.md`.
   Still open: photodiode/BBTK validation of actual pixel onset on real
   hardware.
3. ~~**Phase 4 — audio.**~~ **Done 2026-07-13.** `platformInitAudio` on js
   opens the default device like desktop (un-stubbed ~12 audio bindings in
   the fork); SDL's Emscripten backend auto-resumes the gesture-suspended
   AudioContext on first keypress. Verified: buzzer feedback plays in
   parity_decision (48 kHz stereo, AudioContext running). Caveat documented:
   ~43 ms buffer latency; PlaySyncedWithFlip's desktop guarantees don't
   transfer.
4. **Phase 5 — packaging, docs, CI.**
   - Replace the stale `wasm-%` Makefile targets with `wasmsdl`-based ones.
   - Delete the obsolete May-2026 artifacts in `examples/hello_world/`
     (`SDL3.js`, `SDL3.wasm`, `main.wasm`, `index.html`, `wasm_exec.js`).
   - Add a CI job that at least builds `GOOS=js GOARCH=wasm` (excluding
     `triggers/`).
   - ~~Commit the pending goxpyriment working-tree changes~~ Done 2026-07-13;
     the go-sdl3 `replace` now points at a pinned GitHub pseudo-version of
     the fork (portable — no local clone needed to build).
   - Decide: PR the fork upstream to Zyko0/go-sdl3, or keep maintaining it.
5. **Nice-to-haves / known gaps.**
   - Canvas-fills-viewport option in the js `NewScreen` (fixed 1024×768 now).
   - HTML-form replacement for `GetParticipantInfo` on js.
   - Un-stub remaining bindings as experiments hit them
     (`go run ./cmd/gen-wasm-exports/` prints the worklist);
     gamepad/joystick, audio recording, video playback are untouched.
   - Try more examples in the browser (Stroop, Number-Comparison,
     Contrast-Detection-QUEST) — each may surface one or two more stubs.

### How to verify (recipes)

```bash
# Bundle + serve an example
cd ~/00_git/go-sdl3-wasm
go run ./cmd/wasmsdl serve ~/00_git/goxpyriment/examples/parity_decision
# → http://localhost:8080/?s=1

# Headless render check
google-chrome --headless=new --use-gl=swiftshader --no-sandbox \
  --virtual-time-budget=6000 --screenshot=/tmp/shot.png \
  --enable-logging=stderr "http://localhost:8080/?s=1" 2>&1 | grep INFO:CONSOLE
```

A full driven run (presses SPACE, answers F/J, captures the CSV download) is
scripted in `drive_parity.py` from the 2026-07-13 session scratchpad — CDP
over websockets; see `docs/WASM.md` "Headless verification".

After changing the fork: `GOWORK=off go mod vendor` in goxpyriment, then
`go build ./... && go vet ./...`.

## Other

- improve the movie player for gv format: the gv file should be read
  progressively from the disk in a goroutine, if possible (but beware lz4
  compression).
- add support for some eyetrackers, e.g., eyelink 1000
- test triggers with Labjack T4, FT232H and GPIO
- add audio recording for naming
