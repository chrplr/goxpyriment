# Packaging Your GoXpyriment Experiment

This guide explains how to package and distribute your `goxpyriment` experiments as standalone binary executables and installers (e.g., `.exe`, `.deb`, `.rpm`, `.zip`) for different platforms and architectures.

## Prerequisites

1.  **GoReleaser**: The primary tool for automating the build and release process. Install it from [goreleaser.com](https://goreleaser.com/).
2.  **No SDL3 installation needed**: `goxpyriment` uses `github.com/Zyko0/go-sdl3/bin/binsdl` which embeds pre-built SDL3 and SDL3_ttf libraries for Linux, macOS, and Windows (both amd64 and arm64) directly inside the Go binary. There is nothing extra to bundle or install.

## Project Structure Recommendation

For a distributable experiment, keep your assets organized:

```text
my_experiment/
├── assets/             # Large files (videos, high-res images)
├── assets_embed/       # Small files (fonts, icons, sound effects)
├── main.go             # Your experiment logic
├── go.mod
└── .goreleaser.yaml    # GoReleaser configuration
```

## Step 1: Handling Assets

### A. Embedding (Small Assets)
For small files (fonts, small sounds), use Go's `//go:embed` directive. This includes the assets directly inside the binary.

```go
//go:embed assets_embed/font.ttf
var MyFont []byte
// ...
exp.LoadFontFromMemory(MyFont, 32)
```

### B. Bundling (Very Large Assets)
For assets that are too large to embed in the binary (e.g., video files), bundle them alongside the executable and look for them relative to the executable path at runtime. For images, CSVs, audio clips, and similar small-to-medium files, prefer embedding (`//go:embed`) over bundling.

## Step 2: GoReleaser Configuration

Create a `.goreleaser.yaml` in your project root. Here is a template based on the `retinotopy` example:

```yaml
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - id: experiment
    env:
      - CGO_ENABLED=0  # Required for easy cross-compilation with purego
    goos:
      - linux
      - windows
      - darwin
    goarch:
      - amd64
      - arm64
    main: ./main.go
    binary: my_experiment

archives:
  - id: default
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    formats: [zip]
    files:
      - assets/**/*
      - README.md

nfpms:
  - id: linux-packages
    package_name: my-experiment
    maintainer: Your Name <you@example.com>
    description: My Awesome Behavioral Experiment
    formats:
      - deb
      - rpm
    contents:
      - src: assets/**/*
        dst: /usr/share/my-experiment/assets
```

## Step 3: Building

### Local Snapshot Build
To test the packaging locally without creating a GitHub release:

```bash
goreleaser build --snapshot --clean
```

The resulting binaries and packages will be in the `dist/` folder.

### Official Release
1.  Commit your changes.
2.  Tag your repository: `git tag -a v1.0.0 -m "First release"`
3.  Push the tag: `git push origin v1.0.0`
4.  Run GoReleaser (or use a GitHub Action like `retinotopy_build.yml`):

```bash
goreleaser release --clean
```

## Platform-Specific Notes

### Windows
SDL3 is embedded in the binary by `binsdl`, so users can simply unzip and run the experiment without installing anything extra.

### Linux
Generating `.deb` or `.rpm` files via `nfpms` (included in GoReleaser) allows users to install your experiment using `sudo apt install ./my_experiment.deb`.

### macOS
For macOS, GoReleaser creates a binary. To create a proper `.app` bundle or `.dmg`, you may need additional tools like `gon` for notarization or custom scripts to create the folder structure:
`MyExperiment.app/Contents/MacOS/my_experiment`.

## Using GitHub Actions

You can automate this entire process by using the `retinotopy_build.yml` workflow. Whenever you push a new version tag (e.g., `v1.2.3`), GitHub will automatically build all versions and attach them to a new Release page on your repository.
