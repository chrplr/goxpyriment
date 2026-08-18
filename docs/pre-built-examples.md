## Pre-built binaries of examples and tests


If you prefer not to compile the [examples](GalleryOfExamples.md) with Go, you can download _pre-built executables_ ready to run on your computer.

⚠️  **WARNING** These binaries are unsigned. macOS Gatekeeper and Windows Defender will show security warnings or worse, _misleading messages_ such as 'this program is damaged'. Your antivirus may quarantine the files. Don't let them intimidate you. These warnings pop out because I am not willing to pay third parties to sign the executables. The isntructions below explain how to run these programs anyway.

* **Windows (x86-64):** Download [`goxpyriment-examples-windows-x86_64.zip`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-windows-x86_64.zip);  **before unzipping it**: right-click the downloaded `.zip` → **Properties** → at the bottom of the **General** tab tick **Unblock** → **OK**; then you can extract the `.exe` files and run them directly. If a security dialog window pops out, click on **More info**, then **Run anyway**. (You can do the same thing in PowerShell with `Unblock-File .\goxpyriment-examples-windows-x86_64.zip`. If you have already extracted the `.exe` files, unblock all of them with `Get-ChildItem -Recurse <folder> | Unblock-File`.)

* **macOS (Apple Silicon):** Download [`goxpyriment-examples-macos-arm64.zip`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-macos-arm64.zip), unzip it and move the folder to the location of your choice; Open a Terminal at that location and remove the protection with `xattr -rd com.apple.quarantine .` ( See [macOS installation and security](https://chrplr.github.io/note-about-macos-unsigned-apps).

* **Linux (x86-64):** Download [`goxpyriment-examples-linux-x86_64.tar.gz`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-linux-x86_64.tar.gz), extract the binaries with `tar xzf`, and run them directly (potentialy, you may need to do ``chmod +x *`` to set their execute permission bit).

* **Linux (arm64 / Raspberry Pi):** Download [`goxpyriment-examples-linux-arm64.tar.gz`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-linux-arm64.tar.gz), extract the binaries with `tar xzf`, and run the binaries directly. 

Note: if you do not find a version for your hardware (e.g. Mac/Intel or Windows/ARM), do not despair: it is very easy to compile these examples  
following the instructions in [Installation.md]

Good programs to start: `Memory_span`, `Change-Blindness`, `Simon_task`, `LoT_geometry`, `Retinotopy`...


When launched from the command line, most programs accept `-w` (windowed 1024×768 mode), `-d N` (open on monitor N, 0 = primary), and `-s <id>` (subject ID written to the `.csv` data file).

**Results are saved in a folder `goxpy_data` in your Home (User) folder**
