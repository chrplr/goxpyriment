# Memory Span in the browser

`index.html` is a launcher page for the WebAssembly build of this experiment:
a short task description, a participant-ID field, and a Start button, with the
SDL canvas hidden until Start is pressed.

## Running it

From the repository root:

```bash
make wasm-Memory_span-serve     # build + serve on http://localhost:8080
make wasm-Memory_span           # build only, into _build/wasm/Memory_span/
```

The `wasm-%` targets pick this page up automatically: they pass
`-html examples/Memory_span/web/index.html` to the `wasmsdl` bundler whenever
`examples/NAME/web/index.html` exists, so the bundle gets this launcher instead
of wasmsdl's bare-canvas default. The bundler writes `sdl.js`, `sdl.wasm`,
`wasm_exec.js` and `main.wasm` next to it — this directory holds only the page.

WebAssembly cannot be loaded from a `file://` URL, so the bundle needs a web
server. Any static server works, but prefer `make wasm-Memory_span-serve`:
it sends the `Cross-Origin-Opener-Policy: same-origin` and
`Cross-Origin-Embedder-Policy: require-corp` headers that make the page
cross-origin isolated, which is what gives SDL timestamps ~5 µs resolution
instead of ~100 µs. The page detects the difference and shows a warning when
those headers are missing.

## What the page does beyond loading the wasm

- **Start is a real user gesture.** Browsers create the AudioContext suspended;
  SDL's Emscripten backend resumes it on the first click. Pressing Start
  unlocks the buzzer that marks incorrect trials.
- **Participant ID → URL.** `control.NewExperimentFromFlags` synthesizes its
  flags from `location.search` on `GOOS=js`, so the field is written back as
  `?s=<id>` with `history.replaceState` before the Go program runs. Opening
  `?s=7` directly still works — the field is pre-filled from the URL.
- **Go starts on a fresh task**, not inside the click handler: the experiment
  blocks its main goroutine for the whole session, and the wasm scheduler can
  only park it — returning to the JS event loop so DOM events keep reaching
  SDL — when it is not nested in another callback's stack.
- **The canvas is scaled to fit** a short viewport once running, rather than
  letting the page scroll. The scale is a CSS transform applied *after* SDL
  has created its window, so the window size SDL derives from the canvas CSS
  box is unaffected, and Emscripten's `getBoundingClientRect`-based pointer
  mapping keeps clicks landing on the button under the cursor.

## Caveats

Responses in this task are mouse clicks on an on-screen grid, and the timing
that matters is the 1 s-per-item presentation — both fine in a browser. See
`docs/WASM.md` for what browser timing does and does not guarantee; anything
needing sub-millisecond stimulus onset should be run natively.

At the end of the 30 trials the browser downloads the two files `results`
normally writes to disk: `Memory Span_sub-<NNN>_date-<date>-<time>.csv` with one
row per trial, and the matching `-info.txt` with the session metadata.
