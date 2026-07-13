# Running goxpyriment experiments in a web browser (WASM)

**Status (2026-07-13): a real experiment runs end-to-end in the browser.**
`examples/parity_decision` — instructions screen, fixation cross, 10 RT trials
with keyboard responses, feedback logic, and CSV + info-file download — was
run to completion in headless Chrome, driven by DevTools-protocol key events.
The data files that Chrome downloaded are well-formed goxpyriment output with
correct browser metadata (`video_driver: emscripten`). See
[What remains](#what-remains-for-a-complete-port) for the open items.

> An earlier version of this document described a manual Emscripten workflow
> (hand-built `SDL3.js`, an `EXPORTED_FUNCTIONS` list, a custom `index.html`).
> That workflow is **obsolete**, replaced by the `wasmsdl` bundler described
> below. A condensed record of the old recipe is kept in the
> [appendix](#appendix-historical-manual-emscripten-recipe) as a reference for
> rebuilding the SDL wasm blob.

## Architecture

Two WASM runtimes cooperate in the same page:

1. **`sdl.wasm`** — SDL3 + SDL3_ttf + SDL3_image + SDL3_mixer (with a Web Audio
   backend) compiled by Emscripten into a single module, loaded by `sdl.js`.
2. **`main.wasm`** — the Go experiment binary compiled with
   `GOOS=js GOARCH=wasm`, loaded by Go's `wasm_exec.js`.

The Go side talks to the Emscripten side through `go-sdl3`'s js bindings
(`syscall/js` calls into `Module._SDL_*`), sharing one Emscripten heap.

### Where the pieces live

| Piece | Location |
|---|---|
| go-sdl3 fork with the js/wasm target | [`github.com/chrplr/go-sdl3-wasm`](https://github.com/chrplr/go-sdl3-wasm), branch **`wasm-render-fixes`** (local clone: `~/00_git/go-sdl3-wasm`). The module path is unchanged (`github.com/Zyko0/go-sdl3`), and goxpyriment's `go.mod` has a `replace` pointing at a pinned pseudo-version of the fork; `vendor/` is kept in sync with `GOWORK=off go mod vendor` |
| Prebuilt `sdl.js` / `sdl.wasm` + bundler | `cmd/wasmsdl` in the fork — embeds the blobs and an `index.html`, and builds/serves a complete browser bundle. Rebuild recipe: `.docker/emscripten-build/Dockerfile` in the fork |
| goxpyriment js platform code | `control/platform_js.go` (URL-parameter flags, no dialog, deferred audio), `apparatus/screen_newscreen_js.go` (canvas window), `results/output_file_wasm.go` (CSV → browser download) — all build tag `js` |
| Export-list generator | `cmd/gen-wasm-exports` — scans go-sdl3's js bindings + goxpyriment's own calls, emits `wasm/exported_functions.json` for `emcc -sEXPORTED_FUNCTIONS=@…`, and **lists the go-sdl3 calls whose js bindings are still panic-stubs** (the remaining-work list) |

### The go-sdl3 replace — what dependents need to know

goxpyriment's `go.mod` pins the fork with:

```
replace github.com/Zyko0/go-sdl3 => github.com/chrplr/go-sdl3-wasm v0.1.2-0.20260713083251-5eb745538d84
```

Go applies `replace` directives **only in the main module**, so a project that
imports goxpyriment as a dependency does *not* inherit this line. Such a
project resolves upstream `Zyko0/go-sdl3` — fine on desktop (behaviour is
unchanged there), but the browser build will hit `panic("not implemented on
js")` stubs. **To build your own experiment for the browser, copy the same
`replace` line into your project's `go.mod`.** (It works because the fork
keeps the upstream module path.)

When developing the fork itself, point the replace at the local clone instead
(`go mod edit -replace github.com/Zyko0/go-sdl3=/home/cp983411/00_git/go-sdl3-wasm`,
then `GOWORK=off go mod vendor`), and restore the pinned GitHub pseudo-version
before committing (`git log -1 --format=%cd-%h --date=format:%Y%m%d%H%M%S` in
the fork gives the timestamp/hash for a fresh pseudo-version; the base tag is
`v0.1.1`, so the form is `v0.1.2-0.<timestamp>-<12-char-hash>`).

## Building and serving an example

From the fork directory:

```bash
cd ~/00_git/go-sdl3-wasm

# Build a self-contained bundle (index.html + sdl.js + sdl.wasm + main.wasm + wasm_exec.js)
go run ./cmd/wasmsdl build -out /tmp/hello_bundle ~/00_git/goxpyriment/examples/hello_world

# Or build + serve on http://localhost:8080 in one step
go run ./cmd/wasmsdl serve ~/00_git/goxpyriment/examples/hello_world
```

WASM cannot be loaded from `file://` URLs; any static server works
(`python3 -m http.server` is enough for the `build` output).

### Session settings via URL parameters

There is no command line in a browser. On `GOOS=js`,
`control.NewExperimentFromFlags` synthesizes `os.Args` from the page URL's
query string, so the standard flags — and any experiment-specific flags the
program registered — work unchanged:

```
http://localhost:8080/?s=3&w        equivalent to:  -s 3 -w
```

Keys that don't match a registered flag are ignored with a console note. The
participant-info dialog never opens in the browser (it needs its own SDL
window); when `s` is absent the subject ID defaults to 0, exactly like
`-headless`.

### Headless verification (no display needed)

```bash
cd /tmp/hello_bundle && python3 -m http.server 8123 &
google-chrome --headless=new --use-gl=swiftshader --no-sandbox \
  --window-size=1100,850 --virtual-time-budget=6000 \
  --screenshot=/tmp/hello.png --enable-logging=stderr \
  "http://localhost:8123/?s=1" 2>&1 | grep "INFO:CONSOLE"
```

Go panics (with full stack traces) appear as `INFO:CONSOLE` lines; the
screenshot shows what rendered. Each remaining stub panics with its binding's
file:line, which tells you exactly what to un-stub next.

## What works today (verified 2026-07-13)

- `GOOS=js GOARCH=wasm go build` for the library (all packages except
  `triggers/`, which needs a serial port and stays desktop-only) and examples.
- `wasmsdl build` bundles a goxpyriment example; the page boots both WASM
  modules.
- Full `control.Experiment` initialization: SDL + TTF init, canvas window +
  renderer, embedded font via the in-memory IOStream path
  (`IOFromDynamicMem`/`Write`/`Seek`), keyboard/mouse wiring, system- and
  display-info gathering, data-file creation.
- Text rendering (`stimuli.TextBox`/`TextLine` with wrap alignment and string
  metrics), fixation cross, `SetLogicalSize` letterboxing.
- The full trial machinery inside `exp.Run`: `ShowInstructions`,
  `Blank`/`ShowTimed`/`Wait`, `ShowAndGetRT` with hardware-timestamp RTs,
  `exp.Data.Add`, ESC/quit handling via `pumpFrame`
  (`HasEvent`/`GetKeyboardState`).
- Keyboard events reach SDL from the canvas; RTs come back as plausible
  millisecond values (timestamp *granularity* not yet measured — see below).
- `results` output: at experiment end the browser downloads the `.csv` and
  `-info.txt` files (verified: files land in the download directory intact).
- Audio calls degrade to silent no-ops (`exp.Audio` has a zero Device on js)
  instead of crashing; real playback is Phase 4.

### The key design points

- **Blocking experiment code works in the browser — but only on the main
  goroutine.** Go's wasm scheduler parks a blocked main goroutine and returns
  to the JS event loop, so DOM events fire and SDL sees them. Code that blocks
  inside a `js.FuncOf` callback instead wedges the tab (the scheduler spins in
  `findRunnable`). Consequently `sdl.RunLoop` on js paces frames with
  `requestAnimationFrame` but runs the experiment logic on the goroutine that
  called `Run`; the RAF callback only signals a channel. (`SDL_Delay` is a
  no-op on js — it would busy-wait; use `time.Sleep`.)
- **Assets must be embedded.** There is no filesystem; `//go:embed` +
  `FontFromMemory` / `sdl.IOFromBytes` is the portable path (and was already
  the recommended pattern on desktop). Path-based loaders fail on js.

## What remains for a complete port

Phases 0–4 of the roadmap are **done**: branch consolidation and vendor/fork
unification; IOStream + info bindings, URL-parameter setup, hello_world
rendering; `parity_decision` running end-to-end (keyboard trials, RTs,
CSV download) under the redesigned main-goroutine `RunLoop`; timestamp
granularity measured and fixed; flips aligned to `requestAnimationFrame`
with measured 60.00 Hz pacing and zero dropped frames; and audio playback
(see "Audio in the browser" below). Remaining:

**Phase 5 — packaging, docs, CI.** Replace the stale `wasm-%` Makefile targets
with `wasmsdl`-based ones; delete the obsolete May-2026 `SDL3.js`/`SDL3.wasm`
artifacts and `index.html` from `examples/hello_world/`; add a CI job that
builds `GOOS=js`. Longer term: PR the fork upstream to `Zyko0/go-sdl3` or keep
maintaining it.

**Known gaps beyond the phases:**

- Fullscreen and multi-monitor are meaningless in a canvas; the js `NewScreen`
  always opens 1024×768 (or the requested size). A "resize canvas to
  viewport" option would be nicer for participants.
- Less-common APIs are still stubbed (gamepad/joystick enumeration, audio
  recording, `WaitEvent`/`WaitEventTimeout`, `SetRenderLogicalPresentation`,
  video playback). They panic with a clear console message when hit.
- `GetParticipantInfo` (the SDL dialog) cannot run on js; experiments that
  call it directly need an HTML-form alternative or URL parameters.

## Audio in the browser

Audio playback works (verified 2026-07-13): `platformInitAudio` on js opens
the default playback device exactly like on desktop, and the whole
goxpyriment audio API — `exp.Audio.PlayBuzzer/PlayCorrect/PlaySync/PlayAsync`,
`Sound.PreloadDevice/Play/Wait`, `Tone` — runs on SDL's Emscripten Web Audio
backend. Verified in `parity_decision`: the device opens as 48 kHz stereo
F32LE with a 2048-frame buffer, the AudioContext reaches `running`, and the
buzzer feedback plays on incorrect trials.

Browser specifics:

- **First sound needs a user gesture.** Browsers create the AudioContext
  suspended; SDL's Emscripten backend resumes it automatically on the first
  click or keypress. Any experiment that starts with a "press SPACE" screen
  satisfies this without extra code. Sounds triggered *before* any
  interaction stay silent until the first one.
- **Latency is higher than desktop.** The Emscripten device reports a
  2048-frame buffer at 48 kHz ≈ 43 ms; `SetAudioSampleFrames` (an SDL hint)
  is not honoured the same way by the Web Audio backend. Treat audio onset
  timing as approximate in the browser; `PlaySyncedWithFlip`'s desktop
  guarantees do not transfer.
- If the device cannot be opened, the experiment continues with a silent
  no-op `AudioManager` instead of crashing (browser audio support varies).

## Timing in the browser

### Clock / timestamp resolution (measured 2026-07-13, headless Chrome)

SDL timestamps in the browser (`sdl.TicksNS` and input-event timestamps, i.e.
what `ShowAndGetRT` / `GetKeyEventTS` use) tick at:

| Configuration | Resolution |
|---|---|
| plain page | ~100 µs |
| cross-origin isolated page (COOP/COEP headers) | ~5 µs |

`wasmsdl serve` sends the COOP/COEP headers, so served experiments get the
~5 µs clock; any other hosting needs `Cross-Origin-Opener-Policy: same-origin`
and `Cross-Origin-Embedder-Policy: require-corp` to match.

These numbers required a fix in the fork (commit `5eb7455`): out of the box,
**every SDL timestamp was quantized to 1 ms**, because SDL3's
`SDL_GetPerformanceCounter` probes `CLOCK_MONOTONIC_RAW`, Emscripten's WASI
shim rejects that clock id, and SDL silently falls back to `gettimeofday` =
`Date.now()`. The fork's `sdl.js` asset patches `emscripten_date_now` to
`performance.timeOrigin + performance.now()`, and the Dockerfile re-applies
the patch on rebuild.

Note: Go's `time.Now()` on js still has 1 ms resolution (the wall clock is
`Date.now()`). This is one more reason for the existing rule: RTs come from
the SDL event clock, never wall-clock deltas.

### Frame pacing (measured 2026-07-13, headless Chrome)

On js, `Screen.Update()` (and therefore `Flip`/`FlipTS`/`PacedFlip`/`Show`)
presents to the canvas and then parks until the browser's next
`requestAnimationFrame` tick — the browser's VSYNC equivalent (implemented in
`apparatus/screen_present_js.go` on top of `sdl.WaitAnimationFrame` in the
fork). This is required for correctness, not just pacing: canvas updates are
only composited when the page yields, and `PacedFlip`'s desktop busy-wait
would freeze the tab. A 250 ms fallback tick prevents deadlock in background
tabs (where browsers throttle RAF) — but run experiments in a **focused,
foreground tab**.

Measured flip-loop intervals (299 frames, alternating full-screen colors):

| Loop | mean | SD | min–max | dropped frames |
|---|---|---|---|---|
| `Update` loop | 16.666 ms (60.00 Hz) | 0.12 ms | 16.1–17.3 ms | 0 |
| `PacedFlip` loop | 16.666 ms (60.00 Hz) | 0.11 ms | 16.3–17.1 ms | 0 |

Caveat: headless Chrome ticks a virtual 60 Hz compositor; on real hardware
the rate follows the display (e.g. 120 Hz laptop panels) and jitter depends
on system load. The hardware-locked VSYNC timing available on desktop (DRM
ioctl, CoreVideo) is not available in a browser, and there is no photodiode
validation of actual pixel onset yet. Experiments that require
sub-millisecond stimulus onset precision (rapid RSVP, subliminal priming)
should be run natively. Cognitive tasks (choice RT, memory, attention) work
fine.

## Notes for developers

- **Un-stubbing a go-sdl3 js binding** is usually just deleting the
  `panic("not implemented on js")` first line, *but audit the marshalling*:
  the fork's `CLAUDE.md` documents the rules (opaque handles vs input structs
  vs out-params, `float32` passed directly, `size_t` as i32 not BigInt, u64
  returns via `internal.GetInt64`, struct returns via `internal.NewObject`,
  `HEAPU8` fetched fresh).
- **`Module._SDL_Foo is not a function`** at runtime means the C symbol is
  missing from the `sdl.wasm` export list — regenerate the list with
  `go run ./cmd/gen-wasm-exports/` and relink the blob
  (`.docker/emscripten-build/Dockerfile` in the fork).
- **After changing the fork**, re-sync goxpyriment's vendor tree:
  `GOWORK=off go mod vendor` at the repo root.

---

## Appendix — historical manual Emscripten recipe

The early-2026 attempt built the SDL blob by hand. Kept here (condensed) as a
reference; the current blob is built by `.docker/emscripten-build/Dockerfile`
in the fork and additionally bundles SDL3_image and SDL3_mixer.

```bash
# Emscripten SDK
git clone https://github.com/emscripten-core/emsdk.git && cd emsdk
./emsdk install latest && ./emsdk activate latest && source ./emsdk_env.sh

# SDL3 (static)
git clone https://github.com/libsdl-org/SDL.git -b release-3.2.x SDL3-src
emcmake cmake -S SDL3-src -B SDL3-wasm -DCMAKE_BUILD_TYPE=Release \
  -DSDL_SHARED=OFF -DSDL_STATIC=ON -DSDL_TESTS=OFF -DSDL_EXAMPLES=OFF
emmake make -C SDL3-wasm -j$(nproc)
cmake --install SDL3-wasm --prefix "$PREFIX"

# FreeType, then SDL3_ttf against the same prefix
emcmake cmake -S freetype-src -B freetype-wasm -DCMAKE_BUILD_TYPE=Release
emmake make -C freetype-wasm -j$(nproc) && cmake --install freetype-wasm --prefix "$PREFIX"
emcmake cmake -S SDL3_ttf-src -B SDL3_ttf-wasm -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_PREFIX_PATH="$PREFIX" -DSDL3TTF_HARFBUZZ=OFF -DSDL3TTF_SAMPLES=OFF
emmake make -C SDL3_ttf-wasm -j$(nproc) && cmake --install SDL3_ttf-wasm --prefix "$PREFIX"

# Link one module; export list generated by cmd/gen-wasm-exports
emcc -o sdl.js "$PREFIX"/lib/libSDL3.a "$PREFIX"/lib/libSDL3_ttf.a "$PREFIX"/lib/libfreetype.a \
  -s MODULARIZE=0 -s ENVIRONMENT=web -s ALLOW_TABLE_GROWTH=1 \
  -s EXPORTED_RUNTIME_METHODS='["HEAPU8","stackAlloc","stackSave","stackRestore","getValue","setValue","stringToUTF8OnStack","UTF8ToString","addFunction"]' \
  -s EXPORTED_FUNCTIONS=@wasm/exported_functions.json
```

Pitfalls recorded at the time: use `-DCMAKE_PREFIX_PATH` (where
`SDL3Config.cmake` was installed), not `-DSDL3_DIR` at the raw build dir;
SDL3_ttf pins a minimum SDL3 version — if they mismatch, rebuild SDL3 from the
tag SDL3_ttf requires; all libraries must be linked into **one** module
because the Go bindings share a single Emscripten heap; force-link static
archives (`-Wl,--whole-archive`) or `EXPORT_ALL` drops unreferenced symbols.
