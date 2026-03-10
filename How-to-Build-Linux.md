## How to Build `goxpyriment` on Linux

This guide explains how to:

- Install Go
- Install FFmpeg development libraries for video support
- Build and run the examples under `examples/`

The instructions assume a recent Debian/Ubuntu–like system; for other distros, package names may differ slightly.

---

### 1. Install Go

Check whether Go is already installed:

```bash
go version
```

If the command is not found or the version is older than 1.25, install Go:

```bash
# Example for Debian/Ubuntu; adjust version as needed.
sudo apt update
sudo apt install -y golang

# Or install from the official tarball (recommended if distro is old):
# 1. Download from https://go.dev/dl/ (e.g. go1.25.x.linux-amd64.tar.gz)
# 2. Then:
#   sudo rm -rf /usr/local/go
#   sudo tar -C /usr/local -xzf go1.25.x.linux-amd64.tar.gz
#   echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.bashrc
#   source ~/.bashrc
```

Verify:

```bash
go version
```

---

### 2. Install FFmpeg development libraries (for video examples)

The core `goxpyriment` library and most examples (e.g. Stroop, lexical decision, card game) work without FFmpeg.  
Only the video–related code (`stimuli.Video`, `examples/play_videos`, `examples/play_two_videos`) requires FFmpeg development headers.

On Debian/Ubuntu:

```bash
sudo apt update
sudo apt install -y \
  ffmpeg \
  libavcodec-dev \
  libavformat-dev \
  libavutil-dev \
  libswscale-dev \
  libswresample-dev \
  libavfilter-dev \
  libavdevice-dev
```

On Fedora:

```bash
sudo dnf install -y \
  ffmpeg ffmpeg-devel
```

If you build FFmpeg from source instead (to match a specific version), you will need to point `pkg-config` and the Go toolchain to your custom installation. A typical setup (adjust paths as needed) looks like:

```bash
export PKG_CONFIG_PATH="$HOME/ffmpeg/lib/pkgconfig"
export CGO_CFLAGS="-I$HOME/ffmpeg/include"
export CGO_LDFLAGS="-L$HOME/ffmpeg/lib"
```

Add those exports to your shell profile (`~/.bashrc` or `~/.zshrc`) if you want them to persist.

> Note: SDL3 is vendored via `github.com/Zyko0/go-sdl3` and its `bin` helpers, so you do **not** need system-wide SDL3 dev packages for normal use.

---

### 3. Clone the repository

```bash
cd ~/code   # or any directory you prefer
git clone https://github.com/chrplr/goxpyriment.git
cd goxpyriment
```

---

### 4. Build and run simple examples (no video)

Most examples live under `examples/` and are Go submodules. A typical build–run cycle:

```bash
cd examples/simple_example

# Initialize a module (only needed the first time if you copy the example elsewhere)
go mod tidy

# Run directly:
go run .

# Or build a binary:
go build -o simple_example .
./simple_example -d        # -d = developer mode (windowed 1024x1024)
```

You can do the same for other non-video examples, for instance:

```bash
cd examples/stroop_task
go run . -d -s 1

cd ../card_game
go run . -d -s 1
```

Flags used by most examples:

- `-d` : **developer mode** — windowed 1024×1024 instead of exclusive fullscreen.
- `-s` : **subject ID** — integer that is written into the `.xpd` result filename and the first column of the data file.

By default (without `-d`), `control.NewExperiment` with `width=0`, `height=0` uses **exclusive fullscreen** at the current desktop resolution.

---

### 5. Build and run video examples

Ensure FFmpeg dev packages are installed (see step 2), then:

```bash
cd examples/play_videos
go run . -d -s 1
```

The example expects video files in:

- `examples/assets/` (see the repository’s `assets` and `examples/assets` directories).

The dual–video example:

```bash
cd ../play_two_videos
go run . -d -s 1
```

If you compiled FFmpeg yourself and installed it under a non-standard prefix, be sure your `PKG_CONFIG_PATH`, `CGO_CFLAGS`, and `CGO_LDFLAGS` environment variables are set correctly **before** invoking `go run` or `go build`, so that `go-astiav` can find the FFmpeg headers and libraries.

---

### 6. Building everything (optional)

To check that the whole repo builds:

```bash
cd ~/code/goxpyriment

# Build the main module
go build ./...

# Then build all examples
cd examples
go build ./...
```

If some video examples fail to link, double-check the FFmpeg dev installation and CGO environment variables from step 2.

