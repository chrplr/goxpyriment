# Reading-1 in the browser

`index.html` is a launcher page for the WebAssembly build of this experiment: a
short task description, the response-key legend, a participant-ID field and a
Start button, with the SDL canvas hidden until Start is pressed.

## Running it

From the repository root:

```bash
make wasm-Reading-1-serve     # build + serve on http://localhost:8080
make wasm-Reading-1           # build only, into _build/wasm/Reading-1/
```

The `wasm-%` targets pick this page up automatically: they pass
`-html examples/Reading-1/web/index.html` to the `wasmsdl` bundler whenever
`examples/NAME/web/index.html` exists, so the bundle gets this launcher instead
of wasmsdl's bare-canvas default. The bundler writes `sdl.js`, `sdl.wasm`,
`wasm_exec.js` and `main.wasm` next to it — this directory holds only the page.

WebAssembly cannot be loaded from a `file://` URL, so the bundle needs a web
server. Any static server works, but prefer `make wasm-Reading-1-serve`: it
sends the `Cross-Origin-Opener-Policy: same-origin` and
`Cross-Origin-Embedder-Policy: require-corp` headers that make the page
cross-origin isolated, which is what gives SDL timestamps ~5 µs resolution
instead of ~100 µs. The page detects the difference and shows a warning when
those headers are missing.

## What the page does beyond loading the wasm

- **Participant ID → URL.** `control.NewExperimentFromFlags` synthesizes its
  flags from `location.search` on `GOOS=js`, so the field is written back as
  `?s=<id>` with `history.replaceState` before the Go program runs. Opening
  `?s=7` directly still works — the field is pre-filled from the URL.
- **Go starts on a fresh task**, not inside the click handler: the experiment
  blocks its main goroutine for the whole session, and the wasm scheduler can
  only park it — returning to the JS event loop so DOM events keep reaching
  SDL — when it is not nested in another callback's stack.
- **Keyboard focus is tracked.** Every response here is a keypress, and keys
  reach SDL only while the canvas has focus. Losing focus otherwise looks like a
  hang, so the page detects it and offers a click target to focus the canvas
  again.
- **The canvas is scaled to fit** a short viewport once running, rather than
  letting the page scroll. The scale is a CSS transform applied *after* SDL has
  created its window, so the window size SDL derives from the canvas CSS box is
  unaffected.

Unlike `Memory_span/web`, there is no audio-unlock step: this task plays no
sound, so Start has no AudioContext to resume.

## Caveats

**This experiment is more timing-sensitive than the other browser examples.**
The stimulus is four 50 ms windows — three display refreshes each at 60 Hz — and
the browser paces frames through `requestAnimationFrame`. A dropped frame
lengthens a window instead of being reported as an error. Two consequences:

- The per-trial `stim_dur_ms` column exists precisely so this is checkable.
  Inspect its distribution before trusting a browser-collected session; on a
  60 Hz display it should sit at 200 ms with little spread.
- The `-info.txt` records the refresh rate and the refresh count actually used
  per window, so a session run on a 144 Hz display (7 refreshes = 48.6 ms) is
  not silently pooled with a 60 Hz one.

For results that depend on exact stimulus onset, run the native build. See
`docs/WASM.md` for what browser timing does and does not guarantee.

At the end of the session the browser downloads the two files `results` normally
writes to disk: `Reading-1_sub-<NNN>_date-<date>-<time>.csv`, one row per trial
including the 20 per-letter visibility values, and the matching `-info.txt` with
the session metadata.
