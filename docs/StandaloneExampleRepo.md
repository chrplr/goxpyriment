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

> **Just need to hand off a zip, not publish a repo?** Use the Makefile wrapper
> from the repo root:
>
> ```bash
> make share-<ExampleName>                    # → _build/share/<ExampleName>/
> make share-<ExampleName> VERSION=v0.12.5     # pin a specific goxpyriment release
> ```
>
> That exports a standalone *module* (buildable on its own with `go run .`, no
> workspace) to zip and email — see `examples/README.md` in the repository.
> This guide goes further: it turns that output into a *published repository*
> with release CI and an optional in-browser build. Start from `share.sh`
> (Step 1), then add the pieces below.

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
- **Pin a release that delivers the session as a single `.zip`.** Until
  2026-08-29 the browser build fired two downloads (the `.csv` and the
  `-info.txt`) milliseconds apart, and browsers set to *ask where to save each
  file* silently dropped the second one — sessions arrived with their metadata
  and no results. See ["Why the results arrive as a
  .zip"](WASM.md#why-the-results-arrive-as-a-zip). Check which release you are
  pinning before you publish anything that will collect real data, and make sure
  your `web/index.html` describes the download the pinned version actually
  produces.

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

> **`share.sh` starts with `rm -rf` on its destination.** Point the third
> argument at a parent directory where `<ExampleName>/` does **not** yet exist.
> Re-running it over a repository you have already started would delete the
> `.git` directory and everything you added in the steps below.

`make share-<ExampleName>` is the convenient wrapper, but it always writes to
`_build/share/<ExampleName>/`; call `share.sh` directly with the third argument
when you want the module created straight where the repo will live (e.g.
`~/00_git`).

## Step 2 — Prune goxpyriment-internal files

The copy still contains files that only make sense inside the monorepo. Remove:

```bash
cd ~/00_git/<ExampleName>
rm -rf .claude meta.yaml description.md   # gallery/agent metadata, not needed standalone
```

Keep the `.go` sources, embedded assets, tests, and `README.md`.

If the example ships its own launcher page (`web/index.html`, as
`examples/Reading-1` and `examples/Memory_span` do), `share.sh` has copied it
along with everything else — you do **not** need Rush-Hour's bare page. But it
was written for the monorepo, so grep it and `web/README.md` for references that
no longer resolve:

- `make wasm-<Name>-serve` — that target exists only in the goxpyriment repo.
- `docs/WASM.md` and other relative doc links — repoint them at
  <https://chrplr.github.io/goxpyriment/WASM/>.
- comparisons to sibling examples ("unlike `Memory_span/web`, …").

While you are in there, add a link back to the published repository: a demo page
with no way to reach the source or the desktop build is a dead end for anyone
who wants to actually run the experiment.

## Step 3 — Add the repo scaffolding

Add these files (copy and adapt from the reference repos):

- **`LICENSE`** — pick a license. goxpyriment itself is Apache-2.0, but **you are
  its author**, so you may license your example however you like (Rush-Hour is MIT).
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
- For a **browser** build only: **`web/index.html`** — if the example already
  had one, `share.sh` copied it and Step 2 tells you what to rewrite in it;
  otherwise copy Rush-Hour's, which wires the canvas, `sdl.js`, `wasm_exec.js`
  and `main.wasm` and nothing else. A page that asks for a participant ID before
  starting (`examples/Reading-1/web/index.html`) is the fuller template: it
  writes the ID back as `?s=<id>` with `history.replaceState`, which is how the
  Go side receives it.

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

## Building the browser bundle locally

CI is not the only way to get a bundle, and you should get one before you
publish — a Pages deploy is a slow way to discover that the page is broken. The
standalone repo needs the same two ingredients the workflow uses:

```bash
cd ~/00_git/<ExampleName>
git clone -b wasm-render-fixes https://github.com/chrplr/go-sdl3-wasm ../go-sdl3-wasm
go mod edit -replace github.com/Zyko0/go-sdl3=../go-sdl3-wasm   # NEVER commit this
(cd ../go-sdl3-wasm && go run ./cmd/wasmsdl serve \
    -html "$PWD/../<ExampleName>/web/index.html" "$PWD/../<ExampleName>")
# → http://localhost:8080/?s=1, with the COOP/COEP headers
go mod edit -dropreplace github.com/Zyko0/go-sdl3                # before committing
```

Use `wasmsdl build -out _build/wasm …` instead of `serve` to inspect the bundle.
It is about 10 MB (roughly half `sdl.wasm`, half `main.wasm`), which is why
neither reference repo commits it: the workflow rebuilds it on every push, and
committing 10 MB per rebuild would bloat the history fast. Make sure `.gitignore`
covers wherever you put it.

Verify the `replace` is gone before you commit — `git diff go.mod` — or the
published module will point at a path that exists only on your machine.

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

Sanity checks: the experiment renders and responds; a normal end triggers
exactly **one** download, a `.zip` holding the `.csv` and the `-info.txt` (not
one download per block, and not two files); on a crash you see the error overlay
and **no** download (see [WASM.md](WASM.md)). Check the browser console for
panics — each names the file:line of any still-stubbed binding.

Two things that will mislead you while checking by hand:

- **An unfocused window throttles `requestAnimationFrame` to ~1 Hz.** goxpyriment
  notices and logs `WARNING: measured refresh 1.00 Hz differs from nominal
  60.00 Hz`. A session driven from a background window is not a timing test, and
  a canvas that looks frozen is usually just being throttled.
- **A download that has not appeared is not necessarily lost.** If the browser
  asks where to save each file, the file lands only when the dialog is answered;
  Chrome's own record is in the `downloads` table of
  `~/.config/google-chrome/Default/History` (copy it first — it is locked while
  Chrome runs), where `state=2` with `interrupt_reason=40` and an empty
  `target_path` means the dialog was never answered.

## See also

- `examples/README.md` (in the repository) — `make share-NAME` (the zip-and-email path)
- [WASM.md](WASM.md) — how the browser build works, timing, and known gaps
- [How_to_package_your_experiment.md](How_to_package_your_experiment.md) — the GoReleaser alternative for native binaries
