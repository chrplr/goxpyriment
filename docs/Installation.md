# Goxpyriment installation instructions

## No installation is needed to run a goxypriment app

If you were given a ready-to-run goxpyriment app (a compiled file, such as the ones provided in the [compiled examples](pre-built-examples.md)), you do not need to install anything to use it. Just launch the app by double-clicking its icon in your file folder, or by typing its name in your command line (Terminal or Command Prompt).

⚠️  **WARNING** One potential issue can arise from the antivirus or protection system of your computer if these binaries are unsigned: macOS Gatekeeper and Windows Defender will show security warnings or worse, _misleasding messages_ such as 'this program is damaged'. Your antivirus may quarantine the files. Don't let them intimidate you:
*  macOS: Right-click the app → **Open**, or run `xattr -dr com.apple.quarantine <AppName>.app` in Terminal. See [macOS installation and security](https://chrplr.github.io/note-about-macos-unsigned-apps) for step-by-step instructions.
*  Windows: Just click on "More info" then "Run anyway".
*  These warnings will only pop out the first time you try to execute a given program.


## Minimal installation to execute a goxperiment source code

Let us consider the case where you have the  _source code_ of a goxperiment program (inside a file with the extension `.go`)

If the Go toolchain is not already installed on your computer, you need to download and install it following the instructions at <https://go.dev/doc/install>.

Then, from two things one. You either have :

1. a folder, containg at least the files `go.mod`, `go.sum` and the source code, say `experiment.go` (it can be any any file ).

  Then, you can directly execute the code by typing the command line:

        go run experiment.go

 Or build the app by typing:

        go build experiment.go


2. a single `.go` file, say `experiment.go`.

  Then, you need to copy this file into a folder, e.g. `experiment` and create `go.mod` and `go.sum` by typing

        go mod init experiment
        go mod tidy

  This will create the missing files and you are in the situation above: you can run or build the program with the commands `go run` or `go build`.


## Full installation

If you want to do serious development, in addition to Go, you need to install [Git](https://git-scm.com/), a code editor, and an AI-coding agent like [Claude Code](https://claude.com/product/claude-code). If you are new to this, consult the [detailed instructions](Installing-a-development-environment.md)

Then:


1. Clone [goxpyriment Github repository](http://github.com/chrplr/goxpyriment), by opening a Terminal (App `Git Bash` under Windows), and executing the command-line

            git clone https://github.com/chrplr/goxpyriment.git

   In the future, to update goxpyriment to the latest version, a simple `git pull` from within the `goxpyriment` folder will be sufficient.


2. On a command line in a Terminal, navigate to the `goxpyriment` folder and execute:

   ```
   ./build-all.sh
   ```

   (Note: On Linux/macOS you can instead use `make all`)

   This will compile the codes in [examples/*](https://github.com/chrplr/goxpyriment/tree/main/examples) and [test/*](https://github.com/chrplr/goxpyriment/tree/main/examples)
   (Note: The first time, this will take a while because Go needs to download several libraries. Once this is done, future compilations will be fast.)

   After this operation, a new `_build` folder should contain many binaries (executable apps), that you can either run from the command line, or launch by clicking on their icon in in the folder.

3. If you would like to access the documentations locally, install [pgksite](https://github.com/golang/pkgsite) and [zensical](https://zensical.org/docs/get-started/)
 , and launch them from goxpyriment root folder:

         cd goxpyriment
         pkgsite     # API documentation at http://127.0.0.1/8080
         zensical build --clean
         zensical serve  # Live-reload preview at http://127.0.0.1:8000


### Program your own experiments

Follow the step-by-step guide in [Creating Your Own
Experiment](CreatingYourOwnExperiment.md), which walks you through
writing, running, building, and sharing your own experiment.

For background and reference, see [Getting Started](GettingStarted.md), the examples'
[source codes](https://github.com/chrplr/goxpyriment/tree/main/examples), and the
[available functions](API.md).


### Use an AI coding agent

If you launch `claude` (Claude Code) from the `goxpyriment` folder, the many `CLAUDE.md` files will help the AI coding agent to create experiments fork you. You can descirbe an experiment in plain langauge and ask it to program it using the goxpyriment framework.
