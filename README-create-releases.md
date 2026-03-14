# Creating Example Installers and Releases with GitHub Actions

This repository includes a GitHub Actions workflow that builds installers for all examples and (optionally) publishes them as release assets.

## Workflow overview

- **File**: `.github/workflows/build-examples.yml`
- **Triggers**:
  - On tag pushes matching `v*` (e.g. `v0.1.0`) → build installers and create a GitHub Release.
  - On manual run (`workflow_dispatch`) → build installers and upload them as artifacts (no release if not on a tag).

The workflow defines three jobs:

1. `build-windows-installers` (runs on `windows-latest`)
2. `build-macos-dmg` (runs on `macos-latest`)
3. `create-release` (runs on `ubuntu-latest`, depends on the first two)

---

## Windows: building the examples and installer

The Windows job:

1. Checks out the repository.
2. Sets up Go 1.25.x using `actions/setup-go`.
3. Builds all examples:

   ```powershell
   cd examples
   bash build.sh
   ```

4. Installs **Inno Setup** via Chocolatey:

   ```powershell
   choco install innosetup -y
   ```

5. Builds the Windows installer using the script in `examples/installers`:

   ```powershell
   cd examples/installers
   ISCC windows-goxpyriment-examples.iss
   ```

   This generates:

   ```text
   examples/installers/goxpyriment-examples-setup.exe
   ```

6. Uploads the installer as a workflow artifact named `windows-installer`.

You can also run these steps locally on Windows to test the installer outside of CI.

---

## macOS: building the DMG with all example apps

The macOS job:

1. Checks out the repository.
2. Sets up Go 1.25.x using `actions/setup-go`.
3. Runs the DMG builder script located at `examples/installers/build-macos-dmg.sh`:

   ```bash
   cd examples/installers
   bash build-macos-dmg.sh
   ```

   The script:

   - Finds each subdirectory of `examples/` that contains a `main.go`.
   - Builds each example as a `.app` bundle under:

     ```text
     examples/installers/GoxpyrimentExamples-apps/<ExampleName>.app
     ```

   - Copies any `assets/` directory into the app’s `Contents/Resources/`.
   - Creates a compressed DMG:

     ```text
     examples/installers/goxpyriment-examples.dmg
     ```

4. Uploads the DMG as a workflow artifact named `macos-dmg`.

Again, you can run `build-macos-dmg.sh` locally on macOS to inspect or test the DMG.

---

## Creating a GitHub Release with installers

The `create-release` job:

- Runs on `ubuntu-latest`.
- Depends on both `build-windows-installers` and `build-macos-dmg`.
- Only runs when the workflow was triggered by a **tag push** (`refs/tags/v*`).

Steps:

1. Downloads the previously uploaded artifacts (`windows-installer` and `macos-dmg`) into a local `installers/` directory.
2. Uses `actions/create-release` to create a GitHub Release:
   - `tag_name`: the pushed tag (e.g. `v0.1.0`).
   - `release_name`: `"Goxpyriment examples <tag>"`.
3. Uploads the installers as release assets:
   - Windows:  
     `installers/windows-installer/goxpyriment-examples-setup.exe`  
     → `goxpyriment-examples-setup-<tag>.exe`
   - macOS:  
     `installers/macos-dmg/goxpyriment-examples.dmg`  
     → `goxpyriment-examples-<tag>.dmg`

GitHub’s built‑in `GITHUB_TOKEN` is used for authentication (`secrets.GITHUB_TOKEN`).

---

## How to use it in practice

### 1. Manual test (no release)

From the GitHub Actions UI:

1. Go to **Actions** ➜ **Build example installers**.
2. Click **Run workflow** (workflow_dispatch).
3. Wait for the jobs to finish:
   - `build-windows-installers`
   - `build-macos-dmg`
4. Download the `windows-installer` and `macos-dmg` artifacts from the run page to inspect or test them.

### 2. Tagged release with installers

From your local clone:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This will:

1. Trigger the workflow on `v0.1.0`.
2. Build all examples and their installers.
3. Create a GitHub Release for `v0.1.0`.
4. Attach:
   - `goxpyriment-examples-setup-v0.1.0.exe`
   - `goxpyriment-examples-v0.1.0.dmg`

as release assets that non‑technical users can download directly.

