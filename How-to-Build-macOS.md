## How to Build `goxpyriment` on macOS

This guide explains how to:

- Install Go
- Install FFmpeg (for video examples)
- Build and run the examples under `examples/`

The instructions assume a recent macOS version with [Homebrew](https://brew.sh) installed.

---

### 1. Install Go

Check whether Go is already installed:

```bash
go version
```

If it is missing or older than 1.25, install Go with Homebrew:

```bash
brew install go
```

Verify:

```bash
go version
```

---

### 2. Install FFmpeg (for video examples)

The majority of `goxpyriment` (Stroop, lexical decision, card game, etc.) does **not** require FFmpeg.  
FFmpeg is only needed for:

- `stimuli.Video`
- `examples/play_videos`
- `examples/play_two_videos`

Install FFmpeg with Homebrew:

```bash
brew install ffmpeg
```

This will install headers and libraries in `/opt/homebrew` (Apple Silicon) or `/usr/local` (Intel) depending on your platform.

If Go cannot find the FFmpeg libraries when building the video examples, you may need to provide hints via environment variables, for example:

```bash
export PKG_CONFIG_PATH="/opt/homebrew/lib/pkgconfig:$PKG_CONFIG_PATH"
export CGO_CFLAGS="-I/opt/homebrew/include $CGO_CFLAGS"
export CGO_LDFLAGS="-L/opt/homebrew/lib $CGO_LDFLAGS"
```

Adjust `/opt/homebrew` to `/usr/local` if that is where Homebrew is installed on your machine.

> Note: SDL3 is bundled through `github.com/Zyko0/go-sdl3` and its `bin` helpers; you do not normally need to install SDL3 system-wide.

---

### 3. Clone the repository

```bash
cd ~/code   # or any directory you prefer
git clone https://github.com/chrplr/goxpyriment.git
cd goxpyriment
```

---

### 4. Run non-video examples

Most examples live under `examples/`. You can run them directly with `go run`:

```bash
cd examples/simple_example
go run . -d -s 1
```

Common flags:

- `-d` : **developer mode** — run in a 1024×1024 window instead of fullscreen.
- `-s` : **subject ID** — integer stored in the `.xpd` data filename and in the first column of the data file.

Other examples:

```bash
cd ../stroop_task
go run . -d -s 1

cd ../card_game
go run . -d -s 1
```

If you prefer, you can build binaries:

```bash
cd ../simple_example
go build -o simple_example .
./simple_example -d -s 1
```

By default (without `-d`), experiments created via:

```go
control.NewExperiment("...", 0, 0, true)
```

will use **exclusive fullscreen** at the current desktop resolution.

---

### 5. Run video examples

After installing FFmpeg (step 2), you can try:

```bash
cd examples/play_videos
go run . -d -s 1
```

and:

```bash
cd ../play_two_videos
go run . -d -s 1
```

These examples expect video assets under `examples/assets/`. The repository includes sample `.mov`/`.mp4` files you can use.

If you built FFmpeg yourself in a non-standard location, make sure `PKG_CONFIG_PATH`, `CGO_CFLAGS`, and `CGO_LDFLAGS` point to the correct `include` and `lib` directories before running `go run` or `go build`.

---

### 6. Cross-compiling (optional)

Go makes cross-compilation straightforward. From macOS you can, for example, build a Linux binary for one of the examples:

```bash
cd ~/code/goxpyriment/examples/simple_example
GOOS=linux GOARCH=amd64 go build -o simple_example_linux .
```

Video support (FFmpeg) still requires that appropriate FFmpeg libraries are available on the **target** platform; the cross-compiled binary will look for them there at runtime.

