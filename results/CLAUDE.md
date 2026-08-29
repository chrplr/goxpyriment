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

| Method | Description |
|---|---|
| `AddVariableNames(names []string)` | Write CSV header row (`subject_id` is always prepended automatically — do not include it) |
| `Add(...interface{})` | Append a data row — numbers/bools bare, all other fields always quoted (RFC 4180) |
| `WriteComment(string)` | Write a `#`-prefixed line to the info file |
| `WriteSystemInfo(apparatus.SystemInfo)` | Write SDL/renderer/audio metadata to the info file |
| `WriteDisplayInfo(apparatus.DisplayInfo)` | Write display metadata to the info file |
| `WriteHostInfo(sysinfo.SysInfo)` | Write machine/OS metadata (kernel, desktop, CPU, GPUs, sound server) to the info file |
| `WriteParticipantInfo(map[string]string)` | Write participant metadata to the info file (keys sorted) |
| `WriteEndTime()` | Write session end time and duration to the info file |
| `Save()` | Flush both the CSV and the info file to disk |

### CSV format

```
subject_id,condition,response,rt_ms,correct
3,"congruent","F",412,true
3,"incongruent","J",538,false
```

Numbers and booleans are unquoted; strings are always double-quoted with internal `"` doubled.

### Info file format

```
# --EXPERIMENT INFO
# e mainfile: My Experiment
# e start_time: 20260330-142011
# --SUBJECT INFO
# s id: 3
# --SYSTEM INFO
# sys sdl_version: 3.2.10
# ...
# --DISPLAY INFO
# d refresh_rate_hz: 60.0000
# ...
# e end_time: 20260330-143012.000
# e duration: 00:10:01.000
```

### Constants

| Constant | Value |
|---|---|
| `OutputFileCommentChar` | `"#"` |
| `OutputFileEOL` | `"\n"` |
| `DataFileDirectory` | `"goxpy_data"` |
| `DataFileDelimiter` | `","` |

## OutputFile

Lower-level buffered text file, used as the base of `DataFile` (and its `InfoFile`).

```go
f, err := results.NewOutputFile(directory, filename)
f.Write(content)
f.WriteLine(content)    // content + EOL
f.WriteComment(text)    // "#" + text + EOL
f.Save()                // flush to disk
```

`Save()` is defined in `output_file_desktop.go` (build tag: non-wasm). In the browser it is a no-op that keeps buffering, and `output_file_wasm.go` triggers a download at the end of the session instead.

`DataFile.Finalize()` is split the same way: `data_desktop.go` flushes the CSV and the info file to disk, while `data_wasm.go` packs both into a **single** `.zip` download. That is load-bearing — two downloads fired in a row lose the second one whenever the browser asks where to save each file, which silently cost every browser session its results file between July and August 2026. See "Why the results arrive as a .zip" in `docs/WASM.md` before touching this path.

## Version

`results.Version` is a `string` var set from build info at init time — the git tag when the library is used as a versioned module dependency, `"(devel)"` when built from source via `go.work`. Written automatically to the info file.

## Key conventions

- Call `exp.Data.Save()` after each block for long experiments — the buffer is not flushed automatically until `exp.End()`.
- `DataFile.Add` prepends `subject_id` automatically; do not include it in `AddVariableNames`.
- Always call `AddVariableNames` before the first `Add` so column names appear at the top of the CSV.
