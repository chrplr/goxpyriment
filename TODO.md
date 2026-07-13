# TODO

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
- Run more examples in the browser (Stroop, Number-Comparison,
  Contrast-Detection-QUEST, …) — each may surface a few more stubbed
  go-sdl3 js bindings (`go run ./cmd/gen-wasm-exports/` prints the
  worklist; the fork's COVERAGE.md tracks per-function status).
- Canvas-fills-viewport option in the js `NewScreen` (fixed 1024×768 now).
- HTML-form replacement for `GetParticipantInfo` on js (experiments calling
  it directly can't run in a browser; `NewExperimentFromFlags` users are
  unaffected).

## Other

- improve the movie player for gv format: the gv file should be read
  progressively from the disk in a goroutine, if possible (but beware lz4
  compression).
- add support for some eyetrackers, e.g., eyelink 1000
- test triggers with Labjack T4, FT232H and GPIO
- add audio recording for naming
