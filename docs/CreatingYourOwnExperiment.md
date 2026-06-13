---
title: Creating Your Own Experiment
author: Christophe Pallier <christophe@pallier.org>
date: 2026-06-11
---

# Creating Your Own Experiment (a beginner's guide)

This guide is for people who have **never written Go before**. It walks you
through making a brand-new experiment from scratch: creating a folder, writing a
`main.go` file, running it, building a standalone program, embedding your own
images/sounds/CSV files, and finally producing programs you can hand to
colleagues on Windows, macOS, or Linux.

You do **not** need to learn the whole Go language. For most experiments you only
copy an example, change a few lines, and run it.

---

## Before you start

Make sure goxpyriment is installed first — follow [Installation](Installation.md),
which walks you through installing Git and Go, cloning the repository, and building
the examples. (If you are new to setting up Go, that page links to a more
[detailed development-environment guide](Installing-a-development-environment.md).)

Once that is done, open a terminal and confirm the Go compiler is available:

```bash
go version
```

If you see something like `go version go1.25 ...`, you are ready.

> **What is a terminal?** A text window where you type commands.
> On Windows, use **Git Bash** (installed with Git). On macOS use **Terminal**,
> on Linux any terminal app.

---

## Step 1 — Create a folder for your experiment

Each experiment lives in its own folder. Pick a short name with no spaces:

```bash
mkdir my_experiment
cd my_experiment
```

Everything you do from now on happens inside this folder.

---

## Step 2 — Write `main.go`

Create a file called `main.go` in that folder with the following content. This is
the smallest complete experiment: it shows a message and waits for a key press.

```go
package main

import (
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	// Title, background color, foreground color, default font size.
	exp := control.NewExperimentFromFlags("My Experiment", control.Black, control.White, 32)
	defer exp.End()

	hello := stimuli.NewTextBox("Hello, World!\n\nPress any key to quit.", 600, control.FPoint{}, control.White)

	exp.Run(func() error {
		exp.Show(hello)
		exp.Keyboard.Wait()
		return control.EndLoop // leaving the loop ends the experiment
	})
}
```

You don't need to understand every line yet. The pattern is always the same:

- `control.NewExperimentFromFlags(...)` sets up the screen, audio, keyboard, and data file.
- `defer exp.End()` makes sure everything is closed cleanly when the program stops.
- `exp.Run(func() error { ... })` is your experiment body. Return `control.EndLoop` to finish.

> **Tip:** Start by copying an existing example from the
> [`examples/`](GalleryOfExamples.md) folder that resembles your paradigm, then
> modify it. That is far easier than writing from a blank page.

> 💡 **Vibe-coding:** Launch an AI coding agent (Claude, Gemini, etc.) inside the
> `goxpyriment` folder and ask it to add a new experiment to the `examples` folder —
> this leads the agent to read the existing examples for context. Describe the
> experiment (stimuli, design, etc.) in plain language and enjoy. Recommendation:
> save your prompt in a `description.md` file.

---

## Step 3 — Tell Go about its dependencies

Your code uses the `goxpyriment` library, so Go needs to know how to fetch it.
Run these two commands **once**, inside your folder:

```bash
go mod init my_experiment   # creates go.mod, the project's identity card
go mod tidy                 # downloads goxpyriment and everything it needs
```

`go mod tidy` reads your `import` lines, downloads the missing packages, and
records exact versions in `go.mod` / `go.sum`. Run it again any time you add a
new `import` and Go complains it can't find a package.

You should now have these files:

```text
my_experiment/
├── go.mod
├── go.sum
└── main.go
```

---

## Step 4 — Run it while you work

To compile *and* run in one step:

```bash
go run .
```

The `.` means "the program in this folder". A window opens, your experiment runs,
and the window closes when it ends.

This is your everyday loop: **edit `main.go` → `go run .` → look → edit again.**
You don't need to "build" anything during development.

Useful command-line flags that every experiment understands automatically:

```bash
go run . -w          # windowed mode (1024×768) instead of fullscreen — great for development
go run . -s 007      # set the subject/participant ID to 007
go run . -d 1        # use monitor #1 (for a second screen)
```

When the experiment ends, two files are written to `~/goxpy_data/`: a `.csv` with
your data and a `-info.txt` with session metadata.

---

## Step 5 — Build a standalone program

When you are happy with the experiment, turn it into a single executable file
that runs **without Go installed**:

```bash
go build .
```

This produces a file named after your folder (`my_experiment` on macOS/Linux,
`my_experiment.exe` on Windows). It is fully self-contained — SDL3 and all assets
are baked in — so you can copy that one file to another machine and run it:

```bash
./my_experiment          # macOS / Linux
my_experiment.exe        # Windows
```

You can rename the output or put it somewhere specific:

```bash
go build -o builds/my_experiment .
```

---

## Step 6 — Embed your stimuli (images, sounds, CSV)

A built program is just one file. If your experiment loads images, sounds, or a
list of trials from a CSV, those files would normally be missing on someone
else's computer. The solution is to **embed** them inside the program with Go's
`//go:embed` directive. Embedded files travel inside the executable — nothing to
copy alongside it.

Put your asset files in the folder (or a subfolder), then declare them at the top
of `main.go`.

### A single CSV file of trials

```go
import (
	"bytes"
	"encoding/csv"
	_ "embed" // needed for //go:embed
)

//go:embed trials.csv
var trialsCSV []byte

func loadTrials() [][]string {
	r := csv.NewReader(bytes.NewReader(trialsCSV))
	rows, _ := r.ReadAll()
	return rows
}
```

### A single image or sound

```go
//go:embed apple.png
var appleImg []byte

//go:embed beep.wav
var beepSnd []byte

// ... later, inside your experiment:
picture := stimuli.NewPictureFromMemory(appleImg, 0, 0) // x=0, y=0 → centered
sound := stimuli.NewSoundFromMemory(beepSnd)
```

### A whole folder of files

When you have many files, embed the folder as a virtual filesystem:

```go
import "embed"

//go:embed images
var imagesFS embed.FS

// ... read one file by name:
data, err := imagesFS.ReadFile("images/apple.png")
if err != nil {
	exp.Fatal("could not load image: %v", err)
}
pic := stimuli.NewPictureFromMemory(data, 0, 0)
```

Rules to remember:

- The `//go:embed` line must sit **directly above** the variable, with no blank line.
- Paths are **relative to the `.go` file** and use forward slashes `/` on every OS.
- Add `import "embed"` (for `embed.FS`) or the blank import `_ "embed"` (for a
  `[]byte` / `string`).
- After adding embeds, run `go run .` again to recompile.

> **When *not* to embed:** very large files such as long videos. Embedding a
> 500 MB video makes a 500 MB program and slows builds. For those, ship the file
> next to the program instead — see
> [Packaging Your Experiment](How_to_package_your_experiment.md).

---

## Step 7 — Build for other operating systems (to share with colleagues)

Here is where Go shines. Because `goxpyriment` is pure Go (no C compiler needed),
you can build a Windows program **from a Mac**, a Mac program **from Linux**, and
so on — just by setting two variables: `GOOS` (operating system) and `GOARCH`
(processor type).

```bash
# Windows (Intel/AMD 64-bit) → produces my_experiment.exe
GOOS=windows GOARCH=amd64 go build -o my_experiment-windows.exe .

# macOS (Apple Silicon: M1/M2/M3/M4)
GOOS=darwin  GOARCH=arm64 go build -o my_experiment-macos-arm .

# macOS (older Intel Macs)
GOOS=darwin  GOARCH=amd64 go build -o my_experiment-macos-intel .

# Linux (Intel/AMD 64-bit)
GOOS=linux   GOARCH=amd64 go build -o my_experiment-linux .
```

Common values:

| Target machine            | `GOOS`    | `GOARCH` |
|---------------------------|-----------|----------|
| Windows PC                | `windows` | `amd64`  |
| Mac with Apple Silicon    | `darwin`  | `arm64`  |
| Older Intel Mac           | `darwin`  | `amd64`  |
| Linux PC                  | `linux`   | `amd64`  |
| Raspberry Pi (64-bit)     | `linux`   | `arm64`  |

A small script that builds all of them at once (save as `cross-build.sh`):

```bash
#!/bin/bash
set -e
mkdir -p dist
GOOS=windows GOARCH=amd64 go build -o dist/my_experiment-windows.exe .
GOOS=darwin  GOARCH=arm64 go build -o dist/my_experiment-macos-arm .
GOOS=darwin  GOARCH=amd64 go build -o dist/my_experiment-macos-intel .
GOOS=linux   GOARCH=amd64 go build -o dist/my_experiment-linux .
echo "Done — see the dist/ folder."
```

Then:

```bash
bash cross-build.sh
```

Each file in `dist/` is a complete, standalone program. Email it, drop it on a
USB stick, or share it — your colleague just double-clicks (or runs it from a
terminal). No Go, no Python, no SDL install required.

> **On macOS**, a binary downloaded from the internet may be blocked by
> Gatekeeper. The recipient can allow it once via **System Settings → Privacy &
> Security**, or run `xattr -d com.apple.quarantine my_experiment-macos-arm` in a
> terminal.

For a more polished distribution — `.deb`/`.rpm` packages, Windows installers,
macOS `.app` bundles, and automated GitHub releases — see
[Packaging Your Experiment](How_to_package_your_experiment.md).

---

## Quick reference

| Task                                   | Command                                  |
|----------------------------------------|------------------------------------------|
| Create the project                     | `go mod init my_experiment && go mod tidy` |
| Run while developing                   | `go run .`                               |
| Run in a window                        | `go run . -w`                            |
| Add a new imported package             | `go mod tidy`                            |
| Build a standalone program             | `go build .`                             |
| Build for Windows from another OS      | `GOOS=windows GOARCH=amd64 go build -o app.exe .` |
| Embed a file                           | `//go:embed file.png` above a variable   |

---

## Where to go next

- [Getting Started](GettingStarted.md) — worked tutorials (trials, data logging, RSVP, reaction times).
- [Gallery of Examples](GalleryOfExamples.md) — dozens of complete experiments to copy from.
- [User Manual](UserManual.md) — the concepts behind the framework.
- [API Reference](API.md) — every function and type.
- [Packaging Your Experiment](How_to_package_your_experiment.md) — installers and automated releases.

Happy experimenting!
