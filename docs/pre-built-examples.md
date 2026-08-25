## Pre-built binaries of examples and tests for goxpyriment

If want to run the [examples](GalleryOfExamples.md) directly, without compiling them, you download _pre-built apps_, bundled together and run them on your computer.

⚠️  **WARNING** These binaries are unsigned. macOS Gatekeeper and Windows Defender will show security warnings or worse, _misleading messages_ such as 'this program is damaged'. Your antivirus may quarantine the files. Don't let them intimidate you. These warnings pop out because I am not willing to pay third parties to sign the executables. The instructions below explain how to run these programs anyway.

### Windows (x86-64)

1. Download [`goxpyriment-examples-windows-x86_64.zip`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-windows-x86_64.zip);  
2. Before unzipping it: right-click the downloaded `.zip`, click on **Properties** → at the bottom of the **General** tab tick **Unblock** → **OK**. (You can achieve the same thing in PowerShell with `Unblock-File .\goxpyriment-examples-windows-x86_64.zip`.)
3. Unzip it. You should be able to run the `.exe` files directly, either by clicking on them, or typing their name from a command line.

If a security dialog window pops out, you have forgotten to unblock the zip file before unzipping. You can still run the executable by clicking on **More info**, then **Run anyway**. Note: You can unblock all the `.exe` files after unzipping with `Get-ChildItem -Recurse <folder> | Unblock-File`.

### macOS (Apple Silicon)


1. Download [`goxpyriment-examples-macos-arm64.zip`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-macos-arm64.zip), unzip it and move the folder to the location of your choice, e.g., `$HOME/goxpy`

2. Open a Terminal at that location, move the files out of quarantine and set the execute bit, with the following commandes::


        cd $HOME/goxpy   # <- change to the location where you move the content of the unzipped folder 
        xattr -rd com.apple.quarantine .
        chmod -R +x *

     (See [macOS installation and security](https://chrplr.github.io/note-about-macos-unsigned-apps).)


### Linux 

* **(x86-64):** Download [`goxpyriment-examples-linux-x86_64.tar.gz`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-linux-x86_64.tar.gz), extract the binaries with `tar xzf`, and run them directly (potentially, you may need to do ``chmod +x *`` to set their execute permission bit).

* **(arm64 / Raspberry Pi):** Download [`goxpyriment-examples-linux-arm64.tar.gz`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-linux-arm64.tar.gz), extract the binaries with `tar xzf`, and run the binaries directly. 

---
Note: if you do not find a version for your hardware (e.g. Mac/Intel or Windows/ARM), do not despair: it is very easy to compile these examples  
following the instructions in [Installation.md](Installation.md)
---
Good programs to start: `Memory_span`, `Change-Blindness`, `Simon_task`, `LoT_geometry`, `Retinotopy`...

When launched from the command line, most programs accept `-w` (windowed 1024×768 mode), `-d N` (open on monitor N, 0 = primary), and `-s <id>` (subject ID written to the `.csv` data file).

**Results are saved in a folder `goxpy_data` in your Home (User) folder**
