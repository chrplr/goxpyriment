# Publishing an example as a standalone repository

This guide turns one directory from [`examples/`](GalleryOfExamples.md) into its
own self-contained GitHub repository, so a colleague can:

- download a **prebuilt binary** for Linux/macOS/Windows and run it, and
- optionally **play it in the browser** (a WebAssembly build on GitHub Pages).

It is the process used for the two reference repos — copy either as a template:

| Repo | Kind | Browser build? |
|---|---|---|
| [`chrplr/Language-Localizer-French-audio`](https://github.com/chrplr/Language-Localizer-French-audio) | audio experiment, ships `.wav` assets | no |
| [`chrplr/Rush-Hour`](https://github.com/chrplr/Rush-Hour) | mouse experiment, everything embedded | yes (Pages) |

> **This is different from `make share-NAME`.** `examples/share.sh` (see
> [`examples/README.md`](../examples/README.md)) exports a standalone *module*
> to zip and email. This guide takes that output and makes it a *published
> repository* with release CI and (optionally) an in-browser build. Start from
> `share.sh`, then add the pieces below.

## When to use it

Publish an example this way when you want an external audience to run it without
the goxpyriment workspace. Only publish a **browser** build for paradigms that
tolerate browser timing (choice RT, memory, attention, puzzles) — not ones that
need sub-millisecond onset control (rapid RSVP, subliminal priming). See
[WASM.md](WASM.md#timing-in-the-browser).

## Prerequisites

- The [`gh`](https://cli.github.com/) CLI, authenticated (`gh auth status`).
- For a **browser** build: the example must embed *all* its assets with
  `//go:embed` (no filesystem exists in the browser), and it should avoid APIs
  still stubbed on js — see [WASM.md](WASM.md#what-remains-for-a-complete-port).
- Use a goxpyriment release that contains the browser fixes (**≥ v0.12.5**):
  the js `Save()` no-op (one download at the end, not per block) and the mouse
  path fix (`RenderCoordinatesFromWindow`). Earlier releases crash or spam
  downloads in the browser.

## Step 1 — Generate the standalone module

From the goxpyriment repo root, export the example with the latest release
pinned (or pass a specific version):

```bash
bash examples/share.sh <ExampleName> <version> <output-parent-dir>
# e.g.
bash examples/share.sh Rush-Hour v0.12.5 ~/00_git
# → ~/00_git/Rush-Hour/  with a generated go.mod/go.sum requiring goxpyriment v0.12.5
```

`share.sh` copies the directory, drops the workspace `go.mod`/`go.work` coupling,
writes a fresh `go.mod` requiring the *published* goxpyriment, and runs
`go mod tidy`. The result builds on its own with `go build .`.

## Step 2 — Prune goxpyriment-internal files

The copy still contains files that only make sense inside the monorepo. Remove:

```bash
cd ~/00_git/<ExampleName>
rm -rf .claude meta.yaml description.md   # gallery/agent metadata, not needed standalone
```

Keep the `.go` sources, embedded assets, tests, and `README.md`.

## Step 3 — Add the repo scaffolding

Add these files (copy and adapt from the reference repos):

- **`LICENSE`** — pick a license. goxpyriment itself is GPLv3, but **you are its
  author**, so you may license your example however you like (Rush-Hour is MIT).
  If you change the license, update the per-file header comments to match.
- **`build.sh`** / **`build.bat`** — a one-command local build
  (`CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o <Name> .`). These let a
  colleague build a *locally-signed* binary that avoids OS gatekeeper warnings.
- **`.gitignore`** — ignore the built binary. Note `go build .` produces a binary
  named after the **module** (lowercase, e.g. `rush-hour`) *and* `build.sh`
  produces the `-o` name (e.g. `Rush-Hour`); ignore **both**, plus `dist/` and
  `_build/`.
- **`README.md`** — adapt the example's README: change `go run ./examples/<Name>`
  to `go run .`, and add "Binaries" (Releases) and, if applicable, "Play in your
  browser" sections. The reference READMEs include the standard notes about
  unsigned macOS/Windows binaries.
- For a **browser** build only: **`web/index.html`** (copy Rush-Hour's; it wires
  the canvas, `sdl.js`, `wasm_exec.js`, and `main.wasm`).

## Step 4 — Add the CI workflows

### `.github/workflows/release.yml` (native binaries)

Cross-compiles Linux/macOS/Windows on every push and, on a `v*` tag, publishes a
GitHub Release with archives + `SHA256SUMS`. `CGO_ENABLED=0`; go-sdl3 is pure Go
and embeds SDL for every platform, so all three cross-compile from Linux. This
path uses **upstream** go-sdl3 — no `replace`. Copy Rush-Hour's verbatim and
change the binary name.

### `.github/workflows/pages.yml` (browser build — optional)

Builds the WebAssembly bundle and deploys it to GitHub Pages. Two things make it
work (copy Rush-Hour's file and change nothing but the repo name in comments):

1. It checks out the **`chrplr/go-sdl3-wasm`** fork (branch **`wasm-render-fixes`**),
   which supplies the js/wasm SDL bindings and the `wasmsdl` bundler (ships
   `sdl.js` + `sdl.wasm`).
2. It runs `go mod edit -replace github.com/Zyko0/go-sdl3=../go-sdl3-wasm`
   **in CI only** (never committed), so the browser build uses the fork while
   native/release builds keep using upstream go-sdl3.

> **Gotcha — branch ref, not pinned commit.** `pages.yml` tracks the fork's
> *branch HEAD*, whereas goxpyriment's own `go.mod` pins a specific fork
> pseudo-version. A fix that lives only in goxpyriment's vendored copy (or an
> uncommitted fork edit) will **not** reach your repo's browser build until it is
> committed and pushed to `wasm-render-fixes`. This is the trap that broke
> Rush-Hour's first deploy.

## Step 5 — Create the GitHub repo and enable Pages

```bash
cd ~/00_git/<ExampleName>
git init -b main && git add -A && git commit -m "Initial commit"

gh repo create chrplr/<ExampleName> --public --source=. --remote=origin --push

# Browser build only: enable Pages with GitHub Actions as the source
gh api -X POST repos/chrplr/<ExampleName>/pages -f build_type=workflow
```

The first push runs both workflows. The Pages URL is
`https://chrplr.github.io/<ExampleName>/`.

## Step 6 — Cut a release

Binaries are published only on a version tag:

```bash
git tag v0.1.0 && git push origin v0.1.0   # → release.yml builds + publishes the Release
```

Native binaries are independent of the browser build — tag whenever the source
is ready.

## Updating the pinned goxpyriment later

When a newer goxpyriment fixes something you need, re-pin and redeploy:

```bash
go get github.com/chrplr/goxpyriment@vX.Y.Z && go mod tidy
git commit -am "Bump goxpyriment to vX.Y.Z" && git push   # redeploys Pages
```

> **Gotcha — fresh tags and the checksum DB.** Right after you push a goxpyriment
> tag, `sum.golang.org` may not have indexed it yet and `go get` fails with a
> `500`. Fetch straight from the tag once with
> `GOPROXY=direct GOSUMDB=off go get github.com/chrplr/goxpyriment@vX.Y.Z`; the
> correct hashes land in your `go.sum`, so CI (which reads `go.sum`) is fine.

## Verifying a browser build

Serve the bundle and open it in a real, focused, foreground tab:

```bash
# from the goxpyriment repo, before you split it out:
make wasm-<ExampleName>-serve      # http://localhost:8080/?s=1

# or the built standalone repo's Pages URL after deploy
```

Sanity checks: the experiment renders and responds; a normal end triggers **one**
`.csv` + `-info.txt` download (not one per block); on a crash you see the error
overlay and **no** download (see [WASM.md](WASM.md)). Check the browser console
for panics — each names the file:line of any still-stubbed binding.

## See also

- [`examples/README.md`](../examples/README.md) — `make share-NAME` (the zip-and-email path)
- [WASM.md](WASM.md) — how the browser build works, timing, and known gaps
- [How_to_package_your_experiment.md](How_to_package_your_experiment.md) — the GoReleaser alternative for native binaries
