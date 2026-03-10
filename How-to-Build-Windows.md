## How to Build `goxpyriment` on Windows

This guide explains how to:

- Install Go on Windows
- Install FFmpeg (for video examples)
- Build and run the examples under `examples/`

These steps assume Windows 10/11 on x86_64.

---

### 1. Install Go

1. Download the Windows installer (`.msi`) from:
   - <https://go.dev/dl/>
2. Run the installer and accept the defaults.
3. Open a new `cmd` or PowerShell window and verify:

```powershell
go version
```

You should see Go 1.25 or newer.

---

### 2. Install FFmpeg (for video examples)

Most of `goxpyriment` (Stroop, lexical decision, card game, etc.) works without FFmpeg.  
FFmpeg is **only required** for:

- `stimuli.Video`
- `examples/play_videos`
- `examples/play_two_videos`

On Windows, the easiest route for trying the project is:

- Install a **pre-built FFmpeg** binary (for example from `https://www.gyan.dev/ffmpeg/builds/`), and
- Ensure that the `bin` directory containing `ffmpeg.exe` is on your `PATH`.

For development with `go-astiav`, you also need the **C headers and import libraries**. On Windows this typically means:

1. Download a full FFmpeg development build (with `.lib` and header files) or build FFmpeg with MSYS2/MinGW yourself.
2. Ensure that:
   - The FFmpeg headers (`libavcodec/avcodec.h`, etc.) are available under some `include` directory.
   - The import libraries (`avcodec.lib`, `avformat.lib`, etc.) are under a `lib` directory.
3. Set environment variables so CGO can find them, for example:

```powershell
$env:CGO_CFLAGS = "-IC:\ffmpeg\include"
$env:CGO_LDFLAGS = "-LC:\ffmpeg\lib"
```

Adjust `C:\ffmpeg` to wherever you installed the FFmpeg development files.

> Note: SDL3 is bundled by `github.com/Zyko0/go-sdl3` and its `bin` helpers, so you do **not** need to install SDL3 yourself for normal use.

If you only want to use **non-video** examples, you can skip the FFmpeg steps and still be able to build and run the majority of experiments.

---

### 3. Clone the repository

Open PowerShell:

```powershell
cd $HOME
git clone https://github.com/chrplr/goxpyriment.git
cd goxpyriment
```

If `git` is not installed, install it from <https://git-scm.com/download/win> first.

---

### 4. Run non-video examples

From PowerShell:

```powershell
cd $HOME\goxpyriment\examples\simple_example
go run . -d -s 1
```

Common flags:

- `-d` : **developer mode** — use a 1024×1024 window instead of exclusive fullscreen.
- `-s` : **subject ID** — integer written into the `.xpd` data filename and first column of the data file.

Other examples:

```powershell
cd ..\stroop_task
go run . -d -s 1

cd ..\card_game
go run . -d -s 1
```

To build a binary instead of running directly:

```powershell
cd $HOME\goxpyriment\examples\simple_example
go build -o simple_example.exe .
.\simple_example.exe -d -s 1
```

By default (without `-d`), experiments that call:

```go
control.NewExperiment("...", 0, 0, true)
```

will request **exclusive fullscreen** using the current desktop resolution. On Windows, this corresponds to a borderless fullscreen window.

---

### 5. Run video examples (with FFmpeg installed)

Once your FFmpeg development files are set up (see step 2), you can build and run the video examples:

```powershell
cd $HOME\goxpyriment\examples\play_videos
go run . -d -s 1
```

and:

```powershell
cd ..\play_two_videos
go run . -d -s 1
```

These examples expect video files under `examples\assets\` (the repository includes sample `.mov`/`.mp4` files).

If you encounter linker errors mentioning `avcodec`, `avformat`, etc., double-check:

- That `CGO_CFLAGS` and `CGO_LDFLAGS` point to your FFmpeg `include` and `lib` directories.
- That the `.lib` files are built for the same architecture as your Go toolchain (typically x86_64).

---

### 6. Building everything (optional)

To test a full repository build:

```powershell
cd $HOME\goxpyriment
go build ./...

cd .\examples
go build ./...
```

If the video examples fail due to FFmpeg linkage, you can still use the non-video experiments; the rest of the project will build fine without FFmpeg, as long as you avoid the video-related code.

