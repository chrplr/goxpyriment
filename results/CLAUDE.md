// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

# results package

Experiment data file and buffered output file. Writes trial data to a `.csv` file with `#`-prefixed metadata comments.

## DataFile

```go
df, err := results.NewDataFile(directory, subjectID, expName)
```

Creates `<directory>/<expName>_<subjectID>_<timestamp>.csv`. Directory is created if absent. A metadata header is written automatically with start time, hostname, OS, and framework version.

In normal experiments, access via `exp.Data` — do not create a `DataFile` directly.

| Method | Description |
|---|---|
| `AddVariableNames(names []string)` | Write CSV header row (`subject_id` is always prepended automatically — do not include it) |
| `Add(...interface{})` | Append a data row — numbers/bools bare, all other fields always quoted (RFC 4180) |
| `WriteDisplayInfo(apparatus.DisplayInfo)` | Write display metadata as comment block |
| `WriteParticipantInfo(map[string]string)` | Write participant metadata (keys sorted) |
| `WriteEndTime()` | Write session end time and duration |
| `Save()` | Flush buffer to disk |

### Output format

```
# --EXPERIMENT INFO
# e mainfile: My Experiment
# e start_time: 20260330-142011
# --SUBJECT INFO
# s id: 3
# --VARIABLES
subject_id,condition,response,rt_ms,correct
3,"congruent","F",412,true
3,"incongruent","J",538,false
```

Numbers and booleans are unquoted; strings are always double-quoted with internal `"` doubled.

### Constants

| Constant | Value |
|---|---|
| `OutputFileCommentChar` | `"#"` |
| `OutputFileEOL` | `"\n"` |
| `DataFileDirectory` | `"goxpy_data"` |
| `DataFileDelimiter` | `","` |

## OutputFile

Lower-level buffered text file, used as the base of `DataFile`.

```go
f, err := results.NewOutputFile(directory, filename)
f.Write(content)
f.WriteLine(content)    // content + EOL
f.WriteComment(text)    // "#" + text + EOL
f.Save()                // flush to disk
```

`Save()` is defined in `output_file_desktop.go` (build tag: non-wasm). A no-op stub exists in `output_file_wasm.go` for WebAssembly targets.

## Version

`results.Version` is a `string` var set from build info at init time — the git tag when the library is used as a versioned module dependency, `"(devel)"` when built from source via `go.work`. Written automatically to the `.csv` metadata header.

## Key conventions

- Call `exp.Data.Save()` after each block for long experiments — the buffer is not flushed automatically until `exp.End()`.
- `DataFile.Add` prepends `subject_id` automatically; do not include it in `AddVariableNames`.
- Always call `AddVariableNames` before the first `Add` so column names appear at the top of the file.
