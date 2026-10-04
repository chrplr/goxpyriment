// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# results package

Experiment data file and buffered output file. Writes trial data to a plain `.csv` file (no comment lines) alongside a companion `-info.txt` file that holds all session metadata.

## DataFile

```go
df, err := results.NewDataFile(directory, subjectID, expName)
```

Creates two files in `<directory>`:

- `<expName>_sub-<NNN>_date-<YYYYMMDD>-<HHMMSS>.csv` — pure CSV data, directly importable by Excel and R.
- `<expName>_sub-<NNN>_date-<YYYYMMDD>-<HHMMSS>-info.txt` — `#`-prefixed metadata (start time, hostname, OS, framework version, host info, system info, display info, participant info).

The directory is created if absent.

In normal experiments, access via `exp.Data` — do not create a `DataFile` directly.

### CSV format

Numbers and booleans are unquoted; strings are always double-quoted with internal `"` doubled.

## OutputFile

Lower-level buffered text file, used as the base of `DataFile` (and its `InfoFile`).

`Save()` is defined in `output_file_desktop.go` (build tag: non-wasm). In the browser it is a no-op that keeps buffering, and `output_file_wasm.go` triggers a download at the end of the session instead.

`DataFile.Finalize()` is split the same way: `data_desktop.go` flushes the CSV and the info file to disk, while `data_wasm.go` packs both into a **single** `.zip` download. That is load-bearing — two downloads fired in a row lose the second one whenever the browser asks where to save each file, which silently cost every browser session its results file between July and August 2026. See "Why the results arrive as a .zip" in `docs/WASM.md` before touching this path.

When the page was served by JATOS (`JatosAvailable()`: the global `jatos` from jatos.js exists), the browser build sends data to the server instead (`jatos_wasm.go`): `DataFile.Save()` appends the CSV rows written since the previous Save to the JATOS result data, and `Finalize()` uploads the `.csv` and `-info.txt` as result files and waits for every request; if that fails, it falls back to the `.zip` download. The queued jatos.js calls must stay plain JS functions (no `js.FuncOf`) so they still go out after a crash has ended the Go program. See "Running on JATOS" in `docs/WASM.md`.

## Version

`results.Version` is a `string` var set from build info at init time — the git tag when the library is used as a versioned module dependency, `"(devel)"` when built from source via `go.work`. Written automatically to the info file.

## Key conventions

- Call `exp.Data.Save()` after each block for long experiments — the buffer is not flushed automatically until `exp.End()`.
- `DataFile.Add` prepends `subject_id` automatically; do not include it in `AddVariableNames`.
- Always call `AddVariableNames` before the first `Add` so column names appear at the top of the CSV.
