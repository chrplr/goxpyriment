---
name: wasm
description: Build, serve or debug goxpyriment experiments in the browser (GOOS=js GOARCH=wasm), or change the go-sdl3-wasm fork and re-vendor. Use for anything touching browser builds, platform_js.go files, the fork, or make wasm-* targets.
allowed-tools: Bash, Read, Edit, Write
---

# Browser / WebAssembly (GOOS=js)

Experiments compile to WASM and run in a browser. Full status, build commands
and the remaining-work roadmap live in `docs/WASM.md` — read it first.

## Essentials

- **The fork.** go-sdl3 is replaced by `github.com/chrplr/go-sdl3-wasm`
  (branch `wasm-render-fixes`; local clone at `~/00_git/go-sdl3-wasm`), which
  implements the js bindings. `go.mod` pins a pseudo-version of the fork —
  after changing the fork, bump the pseudo-version (or temporarily point the
  `replace` at the local clone) and re-sync `vendor/` with
  `GOWORK=off go mod vendor`. Projects importing goxpyriment need the same
  `replace` line in their own `go.mod` for browser builds.
- **Bundle/serve** from the repo root: `make wasm-NAME` builds a
  self-contained bundle into `_build/wasm/NAME/`; `make wasm-NAME-serve`
  serves it at http://localhost:8080/?s=1 (runs the fork's `wasmsdl` bundler
  out of the module graph — no local clone or Emscripten needed).
- **Flags** come from URL query parameters (`?s=3&w`); the participant-info
  dialog never opens (see `control/platform_js.go`).
- **Assets** must be `//go:embed`-ed — there is no filesystem; path-based
  loaders fail on js.
- **Missing bindings.** `cmd/gen-wasm-exports` lists the go-sdl3 calls
  goxpyriment uses whose js bindings are still panic-stubs, and regenerates
  the emcc export list.
- **`triggers/` is desktop-only** (serial ports) and excluded from js builds.
- **Build tags.** A `!linux && !windows && !darwin` fallback silently catches
  `js/wasm`; `x/sys/unix` compiles there but defines no syscalls, so it fails
  only at link time. Any new `_other.go` fallback needs `&& !js && !wasip1`
  plus a js implementation.
- **Skip list.** Examples that cannot run in the browser are listed in
  `examples/installers/wasm-skip.txt`; after editing it run
  `make update-examples-gallery`.

## Check

CI runs `GOOS=js GOARCH=wasm go build $(go list ./... | grep -v /triggers)`
plus `-o /dev/null` builds of `./examples/demo_hello_world` and
`./examples/parity_decision`. `/verify` runs all of these.
