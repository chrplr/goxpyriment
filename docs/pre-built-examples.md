## Pre-built binaries of examples and tests

If you prefer not to install Go on your computer and recompile the examples, you can download pre-built executables ready to run on your computer.

⚠️  **WARNING** These binaries are unsigned. macOS Gatekeeper and Windows Defender will show security warnings or worse, _misleasding messages_ such as 'this program is damaged'. Your antivirus may quarantine the files. Don't let them intimidate you:
*  macOS: Right-click the app → **Open**, or run `xattr -dr com.apple.quarantine <AppName>.app` in Terminal. See [macOS installation and security](https://chrplr.github.io/note-about-macos-unsigned-apps) for step-by-step instructions.
*  Windows: Just click on "More info" then "Run anyway".
*  These warnings will only pop out the first time you try to execute a given program.  
*  They appear because I am not willing to pay third parties to sign the executables.


* **Linux (x86-64):** Download [`goxpyriment-examples-linux-x86_64.tar.gz`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-linux-x86_64.tar.gz), extract the binaries with `tar xzf`, and run them directly. 

* **Linux (arm64 / Raspberry Pi):** Download [`goxpyriment-examples-linux-arm64.tar.gz`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-linux-arm64.tar.gz), extract the binaries with `tar xzf`, and run the binaries directly. 

* **Windows (x86-64):** Download [`goxpyriment-examples-windows-x86_64.zip`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-windows-x86_64.zip), extract the `.exe` files and run them directly. 

* **macOS (Apple Silicon):** Download [`goxpyriment-examples-macos-arm64.zip`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-macos-arm64.zip), extract it, and move the `.app` bundles to a folder of your choice (e.g. `~/Applications/goxpyriment`). 

Good programs to start: `Memory_span`, `Change-Blindness`, `Simon_task`, `LoT_geometry`, `Retinotopy`...

When launched from the command line, most programs accept `-w` (windowed 1024×768 mode), `-d N` (open on monitor N, 0 = primary), and `-s <id>` (subject ID written to the `.csv` data file).

**Results are saved in a folder `goxpy_data` in your Home (User) folder**
