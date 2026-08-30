# Publishing individual apps to Cloudflare R2

Tagged releases publish the example and test programs twice: as the four
whole-collection archives attached to the [GitHub
release](https://github.com/chrplr/goxpyriment/releases), and — so a visitor who
wants one experiment need not download a ~500 MB bundle containing 113 others —
as **one zip per program per platform** in a Cloudflare R2 bucket.

* Bucket: `christophe-pallier-apps`
* Endpoint: `https://ce24dc0e8bb587a06d4cfdcf226ccfa9.r2.cloudflarestorage.com`
* Public URL: <https://downloads.pallier.org>
* Entry point: <https://downloads.pallier.org/builds/latest/>

## Layout in the bucket

```
builds/index.html                                     redirect to the newest build
builds/latest/index.html                              redirect to the newest build
builds/latest/wasm/{app}/index.html                   redirect to that experiment
builds/{commit_sha}/index.html                        generated download page
builds/{commit_sha}/wasm/_runtime/sdl.js              ┐ the SDL runtime, shared
builds/{commit_sha}/wasm/_runtime/sdl.wasm            │ by every browser build
builds/{commit_sha}/wasm/_runtime/wasm_exec.js        ┘ (5.3 MB, stored once)
builds/{commit_sha}/wasm/{app}/index.html             launcher page
builds/{commit_sha}/wasm/{app}/main.wasm              the experiment
builds/{commit_sha}/Windows_x86_64/{app}.zip          contains {app}.exe
builds/{commit_sha}/MacOS_arm64/{app}.zip             contains {app}.app/
builds/{commit_sha}/Linux_x86_64/{app}.zip            contains {app}
builds/{commit_sha}/Linux_arm64/{app}.zip             contains {app}
```

Each zip holds exactly one top-level entry, so unzipping yields the program
itself rather than a wrapper directory. Everything is zipped, the Linux binaries
included, so all four columns of the download page behave the same way.

The four whole-collection archives are **not** mirrored here. They contain the
same binaries as the per-app zips — measured, the two are the same size to
within a megabyte, because zip compresses each member independently — so hosting
both would double the bucket for no gain. The page's "Download everything"
section links `github.com/chrplr/goxpyriment/releases/latest/download/…`
instead, where they are kept indefinitely.

`latest` is **redirect pages, not a copy** of the newest build — the whole tree
is ~640 KB against the 3.0 GB it points at. Besides the download page there is
one stub per browser experiment, so
`downloads.pallier.org/builds/latest/wasm/Stroop_task/` is a link you can hand
someone directly. Without those stubs it would 404: the Run links on the
download page are relative and resolve fine once the browser has followed the
top-level redirect, but nobody expects a URL they can read to not exist.

The trade-off of redirecting rather than copying is that the final download
links are commit-pinned, so a link that has been followed goes stale when that
build is pruned. `builds/latest/…` itself never does.

## Browser versions

79 of the 91 examples are also published as WebAssembly and run straight from a
URL — `https://downloads.pallier.org/builds/latest/wasm/{app}/` — with no
download and no install. The remaining 12 cannot: they read their stimuli from
disk, and the browser has no filesystem. `examples/installers/wasm-skip.txt`
lists them with a reason each; converting one to `//go:embed` and deleting its
line is all it takes to publish it.

**The SDL runtime is shared.** `wasmsdl` emits five files per bundle, but
`sdl.js`, `sdl.wasm` and `wasm_exec.js` are byte-identical in every one (the
first two are `//go:embed` constants in the bundler, the third comes from
GOROOT). Publishing that 5.3 MB trio once instead of 79 times saves ~415 MB per
build. Each launcher page therefore loads them from `../_runtime/` and sets
Emscripten's `Module.locateFile` so the glue fetches `sdl.wasm` from there too —
without that one line the page loads and then cannot find its runtime.

Measured 2026-08-29: **607 MB for 79 bundles**, median ~5.5 MB. One outlier
dominates — `Retinotopy` is 129 MB, 21% of the whole tree, because its stimuli
are embedded in the binary. `MEG-localizer` (29 MB) and `Language-Localizer`
(17 MB) come next.

Cross-origin isolation matters here: without the COOP/COEP response headers
(see below) SDL timestamps tick at ~100 µs instead of ~5 µs. Each launcher page
checks `window.crossOriginIsolated` at load and says so on the page rather than
letting coarser reaction times go unnoticed.

## Retention — staying inside the 10 GB free tier

Measured on the 114 programs as of August 2026:

| | Windows | macOS | Linux x86-64 | Linux arm64 | total |
|---|---|---|---|---|---|
| per-app zips | 556 MB | 920 MB | 487 MB | 468 MB | **2.4 GB** |
| collection archives (on GitHub, not here) | 556 MB | 920 MB | 486 MB | 468 MB | 2.4 GB |
| browser (WASM) bundles | — | — | — | — | **607 MB** |

**One build in the bucket is therefore ~3.0 GB.**

`publish-to-r2.sh` keeps the `KEEP` most recent builds — **2 by default**, so the
bucket settles at ~6.0 GB of the 10 GB tier. The prune runs *after* the upload,
so while a new build lands the bucket briefly holds three, about 9.0 GB. That
fits, but it is the tightest part of the design. If the collection grows, the
first lever is `KEEP=1`; the second is dropping `Retinotopy` from the browser
build, which alone would return 129 MB per build.

Commit SHAs carry no ordering, so recency is taken from the last-modified time
of each folder's `index.html`; a folder without one is a failed upload and is
deleted first. The script refuses to prune the build it has just uploaded.

Because only two builds are retained, a commit-pinned URL survives one further
release and then stops working. Documentation should link `builds/latest/`.

## The pieces

| File | Role |
|---|---|
| `examples/installers/build-all-platforms.sh` | Cross-compiles everything. `KEEP_STAGE=1` keeps the per-platform staging directories; `ONLY=<name>` builds a single program, for quick local checks. |
| `examples/installers/package-per-app.sh` | Zips each staged program into `_build/r2/<OS_ARCH>/<app>.zip`. Fails loudly if a name is used by both an example and a test. |
| `cmd/gen-download-index` | Generates `index.html` by scanning `_build/r2/`, taking each program's description from its `meta.yaml` (the same files that feed `GalleryOfExamples.md`). `-redirect <sha>` emits the small forwarding page instead. |
| `examples/installers/build-wasm-apps.sh` | Bundles every eligible example for the browser into `_build/r2/wasm/`, sharing one copy of the SDL runtime. Reports every failure at the end rather than stopping at the first. `ONLY=<name>` builds one. |
| `cmd/gen-gallery` | Also injects the "Try it without building" block into each `examples/<app>/README.md` and the ▶ Run links into `docs/GalleryOfExamples.md`, from the same `wasm-skip.txt`. Run it with `make update-examples-gallery`. |
| `cmd/gen-wasm-launcher` | Writes each app's launcher page — generated from its `meta.yaml`, or adapted from the example's own `web/index.html` when it has one. |
| `examples/installers/publish-to-r2.sh` | Uploads, repoints the redirects, prunes old builds (`KEEP`, default 2). `DRY_RUN=1` changes nothing. |
| `.github/workflows/build-examples.yml` | Runs all of the above in the existing `build-all` job, on `v*` tags and on `workflow_dispatch`. |

The publishing steps are skipped unless the R2 credentials are present, so forks
and credential-less runs still build and release normally.

## Required GitHub secrets

Create an R2 API token (Cloudflare dashboard → R2 → *Manage API Tokens*) with
**Object Read & Write** permission — the prune step needs delete, so *Object
Read only* will not do — scoped to the `christophe-pallier-apps` bucket, then add
its two values as repository secrets:

| Secret | Value |
|---|---|
| `R2_ACCESS_KEY_ID` | the token's Access Key ID |
| `R2_SECRET_ACCESS_KEY` | the token's Secret Access Key (shown once) |

The bucket name and account endpoint are not secret and live in the scripts.

## Running it by hand

```bash
# Quick check of the whole pipeline with a single program (seconds, not minutes)
ONLY=Stroop_task KEEP_STAGE=1 bash examples/installers/build-all-platforms.sh
bash examples/installers/package-per-app.sh
go run ./cmd/gen-download-index -root _build/r2 -commit local -tag v0.0.0-test
xdg-open _build/r2/index.html

# Upload, changing nothing (needs the R2 credentials exported)
export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=...
COMMIT_SHA=$(git rev-parse HEAD) DRY_RUN=1 bash examples/installers/publish-to-r2.sh
```

## Custom-domain directory index

An R2 custom domain does **not** serve `index.html` for a bare directory URL, so
`downloads.pallier.org/builds/latest/` would 404 on its own. A **Cloudflare
redirect rule** on the `pallier.org` zone supplies the missing behaviour by
appending `index.html` to any path ending in `/`:

```
GET /builds/latest/  →  301  →  /builds/latest/index.html  →  200
```

Verified working for `builds/`, `builds/latest/` and `builds/{commit_sha}/`, in
one hop and with no loops. File URLs are left alone — `…/Linux_x86_64/Foo.zip`
still returns `200 application/zip` — because the rule matches only paths ending
in a slash.

A Transform Rule (*Rules → Transform Rules → Rewrite URL*) with the match
`(http.host eq "downloads.pallier.org" and ends_with(http.request.uri.path, "/"))`
rewriting *Path* to `concat(http.request.uri.path, "index.html")` achieves the
same thing internally, without the extra round trip and without changing the
address bar. Either is fine.

**If a bare directory URL ever starts returning 404 again, this rule is the
first thing to check.**

## Cross-origin isolation for the browser builds

A second Cloudflare rule gives the WASM pages the browser's full timer
resolution — **Rules → Transform Rules → Modify Response Header**, named
`WASM cross-origin isolation`:

* Match: `(http.host eq "downloads.pallier.org" and starts_with(http.request.uri.path, "/builds/"))`
* **Set static** — two headers:
  * `Cross-Origin-Opener-Policy` = `same-origin`
  * `Cross-Origin-Embedder-Policy` = `require-corp`

This raises SDL timestamps from ~100 µs to ~5 µs. Everything the pages load is
same-origin, so `require-corp` costs nothing, and it constrains only what a
*page* may embed — the zip downloads are unaffected. Without it the experiments
still run; each launcher page shows a banner saying the clock is coarser, which
is how to tell at a glance whether the rule is in force.
