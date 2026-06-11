# goxpyriment

`goxpyriment` is a high-level Go framework for building behavioral and psychological experiments.

* [GitHub repository](https://github.com/chrplr/goxpyriment)

## Features

1. **Zero-dependency deployment.** Your experiments compile to standalone executables.
2. **Timing precision.** The stimulus loop runs VSYNC-locked with GC pauses disabled, giving sub-millisecond frame jitter on typical hardware (VSYNC can be disabled to support Variable Refresh Rate monitors).
3. **AI-friendly API.** The API is well suited to "vibe-coding": after cloning the repository, describe your paradigm in plain language to Claude, Gemini, or ChatGPT and try the generated code. Given the examples and the constraints afforded by the framework, the experiment is likely to work as expected (but must be checked, of course).

---

## Documentation

| Document | | PDF |
|---|---|---|
| [Presentation](https://github.com/chrplr/goxpyriment/blob/main/paper/goxpyriment_paper.pdf) | short paper | | 
| [Getting Started](GettingStarted.md) | Tutorials | [↓](GettingStarted.pdf) |
| [Creating Your Own Experiment](CreatingYourOwnExperiment.md) | Beginner guide: new project, embedding assets, sharing binaries | |
| [Gallery of Examples](GalleryOfExamples.md) | Ready-to-run experiments and demos | |
| [Installation](Installation.md) | Install Go and build the examples | [↓](Installation.pdf) |
| [User Manual](UserManual.md) | Core concepts explained in depth | [↓](UserManual.pdf) |
| [Migration Guide](MigrationGuide.md) | Coming from Expyriment, PsychoPy, or Psychtoolbox? | [↓](MigrationGuide.pdf) |
| [API Reference](API.md) | Complete function and type reference | [↓](API.pdf) |
| [Timing-Tests](TimingTests.md) | Check the timing of your computer | [↓](TimingTests.pdf) |


**Support:** 

* [Google group](https://groups.google.com/a/pallier.org/g/goxpyriment) — Forum
* Report bugs at <https://github.com/chrplr/goxpyriment/issues>


Note: If you are looking for a simpler, *no-code experiment generator*, check out [Gostim2](https://chrplr.github.io/gostim2/).


--- 

## Quick Start

* Download and install Go from <https://go.dev> (See [Installing-a-development-environment.md](Installing-a-development-environment.md) if you are new to this).

* Clone goxpriment and check that you can compile the examples, by executing the following commands in a Terminal (`Git Bash` for Windows):


        git clone https://github.com/chrplr/goxpyriment.git
        cd goxpyriment
        ./build-all.sh


* Create a `hello` folder somewhere on your computer, and create  `main.go` inside it, containing the following code:

    ```go
    package main

    import (
      "github.com/chrplr/goxpyriment/control"
      "github.com/chrplr/goxpyriment/stimuli"
    )

    func main() {
      exp := control.NewExperimentFromFlags("Hello World", control.Black, control.White, 32)
      defer exp.End()

      hello := stimuli.NewTextBox("Hello, World!", 600, control.FPoint{}, control.White)

      exp.Run(func() error {
        exp.Show(hello)
        exp.Keyboard.Wait()
        return control.EndLoop
      })
    }
    ```

* Initialize dependencies in the `hello` folder:

    ```bash
    go mod init hello
    go mod tidy
    ```

* Run your program


    ```bash
    go run .
    ```

---

## Ready-to-run experiments

[Pre-built binaries](pre-built-examples.md) (ready-to run apps) of many experiments are available for Windows, macOS, and Linux.

---

## Background

Goxpyriment relies on [libsdl](http://libsdl.org) via the [go-sdl3](https://github.com/Zyko0/go-sdl3) bindings.

It was inspired by [expyriment.org](https://github.com/expyriment/expyriment), a lightweight Python library for cognitive and neuroscientific experiments (Krause & Lindemann, 2014. *Behavior Research Methods*, 46(2), 416–428. <https://doi.org/10.3758/s13428-013-0390-6>). The API should feel familiar to expyriment users.

---

## License & citation

GNU GPL v3 — see [LICENSE](https://github.com/chrplr/goxpyriment/blob/main/LICENSE.txt).

Please cite as:
> Christophe Pallier (2026) chrplr/goxpyriment. Zenodo. https://doi.org/10.5281/zenodo.19200598


[Christophe Pallier](http://github.com/chrplr), 2026.


---

![](icon_512.png)
