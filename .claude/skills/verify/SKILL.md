---
name: verify
description: Verify the Go project compiles and vets cleanly. Use when the user asks to verify, check the build, or confirm a Go change is sound before declaring it done.
tools: Bash
---

# Verify

Run `go build ./...` then `go vet ./...` from the repo root, report any errors
verbatim, and only summarize success if **both** commands pass cleanly.

## Steps
1. Run `go build ./...` (repo root, so `go.work` resolves all modules).
2. Run `go vet ./...`.
3. If either fails, show the failing output and stop — do not claim success.
4. If both pass, report a one-line success summary.

## Notes
- Stay at the repo root so the Go workspace (`go.work`) covers the library,
  `examples/`, and `tests/` modules.
- Do not edit code in this skill — it only verifies.
