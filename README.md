# goxpyriment

`goxpyriment` is a high-level Go framework for building behavioral and psychological experiments. 

⟶ Jump to [Demos](./examples/README.md) (pre-built binaries for many experiments; not recommended: only for those in a hurry)
 
* [Getting Started](docs/GettingStarted.md) — Tutorial for psychologists
* [Setup](docs/Installation.md) — install tools to build your own experiments
* [User Manual](docs/UserManual.md) — Core concepts explained in depth
* [API Reference](docs/API.md) — Complete function and type reference
* [Migration Guide](docs/MigrationGuide.md) — Coming from Expyriment, PsychoPy, or Psychtoolbox?
* [Github repository](https://github.com/chrplr/goxpyriment)
* [Google group](https://groups.google.com/a/pallier.org/g/goxpyriment) — Forum (For bugs, report them at <https://github.com/chrplr/goxpyriment/issues>
* [Github.io Page](https://chrplr.github.io/goxpyriment)

If you are looking for a simpler, *no-code experiment generator*, check out [Gostim2](https://chrplr.github.io/gostim2/). 


💡 **TIP**  **Vibe-coding:** After installation, you can launch an AI coding agent (Claude, Gemini, ...) inside the `goxpyriment`folder and ask it to add a new experiment to the `examples` folder — this leads the agent to read the existing examples for context. Describe the experiment (stimuli, design, etc.) in plain language and enjoy. Recommendation: save your prompt in a `description.md` file alongside the code.


⚠️  **WARNING**:  This software is in beta-testing, that is, I am waiting for reports from the battleground before releasing a first stabl version. Although it is certainly possible to use it to implement real experiments in the lab, users should (as always) very carefully check their behavior, for example with a [bbtk](https://chrplr.github.io/bbtkv3/).


Goxpyriment relies on the [libsdl](http://libsdl.org) library through the [go-sdl3](https://github.com/Zyko0/go-sdl3) bindings. 

(While Python is easy, Go is simple: read [Go-vs-Python](gemini-about-go-vs-python.md)). The code was mostly written using Claude Sonnet 4.6, with some input from Gemini 2.5 flash.

As its name suggests, goxpyriment was inspired by [expyriment.org](https://github.com/expyriment/expyriment?tab=readme-ov-file), a nice, light-weight Python library for cognitive and neuroscientic experiments (See Krause, F., & Lindemann, O. (2014). *Behavior Research Methods*, 46(2), 416–428. <https://doi.org/10.3758/s13428-013-0390-6>). The API should feel very familiar to expyriment users.


[ChrPlr](https://github.com/chrplr), March 2026.


## License

This project is licensed under the GNU Public License v3 - see the [LICENSE](LICENSE.txt) file for details.

Please cite thie repository as:

* Christophe Pallier (2026) chrplr/goxpyriment: Goxpyriment vX.Y.Z. Zenodo. https://doi.org/10.5281/zenodo.19200598
*(updating the version!)*



---

![](assets/icon_512.png)

