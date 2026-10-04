# Deploying an experiment on a JATOS server

[JATOS](https://www.jatos.org) (Just Another Tool for Online Studies) is a
server that runs experiments in participants' browsers and stores their results
on the server. A goxpyriment experiment can be packaged as a JATOS study and run
there without changing its code: instead of each participant downloading a
`.zip` of results and sending it to you, the data arrives on the server, block by
block, as the session runs.

This page explains how to build the study, put it on a server, recruit
participants and get the data back. For how browser builds work in general, see
[WASM build](WASM.md).

## What you need

- A JATOS server and an account on it. Your institution may run one; otherwise
  [MindProbe](https://www.mindprobe.eu) offers free hosting for researchers, and
  JATOS can be installed on any machine (see the JATOS documentation). Studies
  built here have been tested on JATOS 3.11.
- This repository and Go (the same setup as for building the examples; see
  [Installation](Installation.md)). No other tools: the build produces the study
  archive itself.
- An experiment that runs in a browser (next section).

## 1. Make sure the experiment runs in a browser

JATOS runs the browser (WebAssembly) build of the experiment, so anything that
works with `make wasm-NAME-serve` works on JATOS. Before deploying:

1. **Embed every stimulus file** (images, sounds, fonts, lists) with
   `//go:embed`. A browser has no file system: an experiment that reads files
   from disk at run time compiles, then fails when it starts. The examples
   listed in `examples/installers/wasm-skip.txt` are in that situation.
2. **Call `exp.Data.Save()` after every block.** Online, it is what sends the
   block's rows to the server. If a participant closes the tab halfway, you keep
   every block saved so far; without `Save()` calls, nothing reaches the server
   until the very end.
3. **Try it locally:**

   ```bash
   make wasm-Simon_task-serve      # then open http://localhost:8080/?s=1
   ```

   Run through it once and check the `.zip` it downloads at the end.

The experiment must live in its own directory under `examples/`, with a
`main.go` and a `meta.yaml` (copy one from an existing example). The
`description:` line of `meta.yaml` becomes the study description shown in
JATOS.

## 2. Build the study archive

```bash
make jatos-Simon_task
```

This writes `_build/jatos/Simon_task.jzip` (about 3 MB). A `.jzip` is JATOS's
own study-archive format: the files of the study plus a description of it. The
archive holds:

| File | Role |
|---|---|
| `index.html` | The page participants see: a short description and a **Start** button |
| `main.wasm` | Your experiment, compiled for the browser |
| `sdl.js`, `sdl.wasm` | The SDL3 graphics/audio runtime |
| `wasm_exec.js` | Go's browser support script |

On Windows, run the same command from Git Bash, or run the script it calls
directly: `bash examples/installers/build-jatos-study.sh Simon_task`.

## 3. Import it into JATOS

1. Log in to your JATOS server.
2. Use **Import study** and select the `.jzip`.
3. The study appears in your study list under the example's name, with one
   component (the experiment). Its files are stored on the server in a folder
   named `goxpy_<name>`.

**Updating the study.** Fix the experiment, rebuild with `make jatos-NAME`, and
import the new `.jzip`. Because every build of the same example carries the same
study identifiers, JATOS recognises it and asks whether to overwrite the
existing study and its files: answer yes to both. The study's links stay valid.

## 4. Create study links

Open the study and go to **Study links**. JATOS offers several kinds of link;
the usual choices are:

| Link type | Use it for |
|---|---|
| **Personal Single** | One link per participant, usable once. Best for lab-recruited participants. |
| **General Single** | One link for everybody, each browser can run it once. |
| **General Multiple** | One link, any number of runs. Use it for piloting and testing. |

A *Single* link really is single: once it has been used, opening it again shows
"Study can be done only once" — even after a failed or abandoned run. Test with
a *Multiple* link.

### Passing parameters

Anything appended to the link after `?` reaches the experiment exactly as on the
command line:

```
https://jatos.example.org/publix/AbCdEfGh?s=12            subject 12
https://jatos.example.org/publix/AbCdEfGh?s=12&w          …in a 1024×768 window
https://jatos.example.org/publix/AbCdEfGh?level=2         any flag the program defines
```

- **Subject ID.** Without `s`, the participant is numbered with the JATOS
  *study result ID*, which is unique for each run on the server. File names and
  the `subject_id` column then never collide, and the number matches the row in
  JATOS's results table.
- **Recruitment platforms** (Prolific, MTurk, SONA…) append their own
  parameters, for example `?PROLIFIC_PID=…`. The experiment ignores parameters
  it has no flag for, and JATOS records them with each result. For Prolific,
  JATOS can also redirect participants back with a completion code: set the
  study's *End redirect URL* in the study properties.
- **Participant information.** Fields of `exp.GetParticipantInfo` are filled
  from parameters of the same name (`?age=31&handedness=2`), as in any browser
  build. No dialog is shown online.

## 5. What participants see

1. The study link opens a page with the experiment's name, its description and a
   **Start** button. There is no participant-ID box.
2. Clicking **Start** runs the experiment in the browser window, as on a desktop.
   The click is needed: browsers allow sound and keyboard focus only after a user
   action.
3. At the end, JATOS's end page is shown (or the participant is redirected, if
   the study has an end redirect URL). Nothing is downloaded.

Ask participants to use a computer with a keyboard, Chrome, Edge or Firefox, and
to keep the tab in the foreground: browsers slow down background tabs.

## 6. Getting the results

Open the study and go to **Results**. Each run is a row, with its state
(`FINISHED`, `FAIL`, `DATA_RETRIEVED`…), the worker, the duration and the
parameters it was started with. Two kinds of data are stored for each run:

| In JATOS | Contents | When it is written |
|---|---|---|
| **Result data** | The CSV text, one block appended after another | At every `exp.Data.Save()` |
| **Result files** | `<name>.csv` and `<name>-info.txt`, the same two files a desktop run writes | At the end of the session |

- Use the **result files** for analysis: they are byte-for-byte what a desktop
  session produces, so existing analysis scripts work unchanged. One difference:
  JATOS refuses spaces and some punctuation in file names, so those characters
  are replaced by `_` — `Simon Task_sub-012_….csv` arrives as
  `Simon_Task_sub-012_….csv`.
- The **result data** is the safety net. It is there even for a participant who
  closed the tab halfway, and it is what you see when you open a result in the
  JATOS interface. Its content is the CSV: header line, then the rows of each
  saved block.

To download, select the runs and use **Export Results**. You can export the
result data (text), the result files (a zip of folders, one per run), or
everything at once. JATOS also has a REST API to fetch results from a script; see
the [JATOS API documentation](https://www.jatos.org/JATOS-API.html).

### When something goes wrong

| What happened | Participant sees | In JATOS |
|---|---|---|
| Session completed | JATOS end page | `FINISHED`, result data and both result files |
| Participant pressed **ESC** | JATOS end page | `FINISHED`, with the blocks saved before ESC |
| Tab closed or browser crashed | — | Run left unfinished; result data holds the saved blocks |
| Experiment error | An error message, which stays on screen | `FAIL` with the error message; result data holds the saved blocks |
| Results could not be sent at the end | A message asking them to send you the `.zip` their browser just downloaded | `FAIL`, with the reason |

The last case is a fallback: jatos.js retries each failed request several times
first, and only if the server still cannot be reached does the browser download
the usual `.zip`, so that the data is not lost.

## Limits and caveats

- **Timing.** In a browser, onsets are tied to the screen refresh as on desktop,
  but timestamps have ~0.1 ms resolution rather than the ~5 µs of a local or
  [cross-origin-isolated](WASM.md#timing-in-the-browser) page: JATOS does not
  send the headers that would enable the finer clock. This is far below
  behavioural response variability, but do not use an online study for
  sub-millisecond measurements.
- **Data size.** By default a JATOS server accepts up to 5 MB of result data per
  run (`jatos.resultData.maxSize`) and 30 MB per result file
  (`jatos.resultUploads.maxFileSize`). Ordinary behavioural CSVs are far
  smaller; an experiment logging every frame or mouse sample might not be.
- **Reloading the page** is refused: JATOS ends the run as failed rather than
  restarting the experiment and mixing two attempts in one result.
- **Custom launcher pages.** Examples that ship their own
  `web/index.html` (`Memory_span`, `Reading-1`) are deployed with the standard
  page; their extra instructions are not shown on JATOS.
- **Download size.** Each study carries its own copy of the SDL runtime, and
  JATOS serves the files uncompressed, so a participant downloads about 11 MB
  before Start is enabled (the `.jzip` itself is ~3 MB).
- **Several components.** The experiment can be one component among others (a
  consent form before it, a questionnaire after): when it finishes, JATOS moves
  on to the next component. Add the other components in the JATOS interface.

## Troubleshooting

- **The console shows "wasm streaming compile failed … Incorrect response MIME
  type", then "falling back to ArrayBuffer instantiation".** Harmless. JATOS
  serves `.wasm` files with a generic content type; the SDL runtime retries in a
  way that does not care, and the experiment's own module is loaded that way from
  the start.

- **The page stays on "Loading…".** Open the browser's developer console
  (F12). A message about a file that cannot be found usually means the study's
  files were not imported completely: import the `.jzip` again.
- **"Study can be done only once".** The link is a *Single* link that has
  already been used, possibly by you. Create a new one, or test with a
  *General Multiple* link.
- **The experiment starts but a stimulus is missing, or it stops at once.** The
  experiment probably reads a file from disk; see step 1. Check with
  `make wasm-NAME-serve`, which behaves the same way.
- **No result files, only result data.** The session did not reach
  `exp.End()`: the participant closed the tab, or the experiment stopped with
  an error (the run is then marked `FAIL` with the message).
