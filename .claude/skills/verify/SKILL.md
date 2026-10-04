---
name: verify
description: Run the same checks as CI (desktop build, vet, unit tests, browser/WASM build, generated-file sync). Use before declaring a Go change done, before any push, or when the user asks to verify or check the build.
allowed-tools: Bash
---

# Verify

Mirrors `.github/workflows/go-build.yml`, `readme-sync.yml` and
`gallery-sync.yml`. If any of those workflows has changed, follow the workflow
rather than this list.

Run from the repo root (so `go.work` covers the library, `examples/` and
`tests/`), in order. Stop at the first failure and show its output verbatim —
do not claim success.

1. `CGO_ENABLED=0 go build ./...` then `go vet ./...`
2. `go test ./control/ ./clock/ ./design/ ./results/ ./geometry/ ./units/ ./staircase/ ./sysinfo/ ./eyetracker/`
3. Browser build (`triggers/` is desktop-only and excluded by design):
   ```bash
   GOOS=js GOARCH=wasm go build $(go list ./... | grep -v /triggers)
   GOOS=js GOARCH=wasm go build -o /dev/null ./examples/demo_hello_world
   GOOS=js GOARCH=wasm go build -o /dev/null ./examples/parity_decision
   ```
4. Generated files. Snapshot first, regenerate, then compare:
   ```bash
   paths="README.md docs/GalleryOfExamples.md examples/*/README.md"
   before=$(git status --porcelain -- $paths; git diff -- $paths | sha1sum)
   make readme && make update-examples-gallery
   after=$(git status --porcelain -- $paths; git diff -- $paths | sha1sum)
   ```
   If `before` ≠ `after`, those files were stale: list them and leave the
   regenerated versions in the working tree for the commit. This is a failure
   to report, not something to hide.

If every step passed, report success in one line.

## Notes
- Do not edit code in this skill — it only verifies (step 4 regenerates
  generated files, which is what CI expects to have been done).
- Docs-site (`zensical build`) and installer builds are not covered; they
  run only on push to main / tags.
