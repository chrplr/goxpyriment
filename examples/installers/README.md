# Installers for goxpyriment examples

This directory contains helper files to package the `examples/` into OS‑native installers for non‑technical users.

> These scripts/configs are **not run automatically** from this repository.  
> You should run them on the appropriate OS with the required tools installed.

## Windows (Inno Setup)

Requirements:

- Windows 10/11
- [Inno Setup](https://jrsoftware.org/isinfo.php)
- A working Go toolchain (to build the example binaries)

Steps:

1. Build all examples (from within `examples/`):

   ```powershell
   cd examples
   .\build.sh
   ```

2. Open `examples\installers\windows-goxpyriment-examples.iss` in **Inno Setup**.

3. Click **Build** ➜ **Compile**.

   - This generates `goxpyriment-examples-setup.exe` in `examples\installers\`.
   - Running that installer will copy the entire `examples\` tree (except `installers/`) into `C:\Program Files\Goxpyriment Examples`.

## macOS (DMG)

Requirements:

- macOS (Apple Silicon or Intel)
- A working Go toolchain

Steps:

1. From the `examples/installers` directory, run:

   ```bash
   cd examples/installers
   bash build-macos-dmg.sh
   ```

2. The script will:

   - Detect all subdirectories in `../` that contain a `main.go`.
   - Build each example as a `.app` bundle into `examples/installers/GoxpyrimentExamples-apps/`.
   - Create a compressed DMG `goxpyriment-examples.dmg` in `examples/installers/`.

3. Distribute the DMG to users. They can mount it and drag the example apps to `/Applications` or run them directly.

## Linux (AppImages)

Requirements:

- Linux (x86_64)
- A working Go toolchain
- FUSE available (to run `appimagetool` itself)

Steps:

1. From the `examples/installers` directory, run:

   ```bash
   cd examples/installers
   bash build-linux-appimages.sh
   ```

2. The script will:

   - Build all examples (via `examples/build.sh`).
   - Download `appimagetool-x86_64.AppImage` if not already present.
   - For each example directory under `examples/` that contains a `main.go` (excluding `assets/`, `installers/`, `goxpy_data/`), create an AppDir and corresponding `<ExampleName>.AppImage` under:

     ```text
     examples/installers/AppImages/
     ```

3. The resulting AppImages can be distributed directly; on most Linux distributions users will just mark them executable and run them.

