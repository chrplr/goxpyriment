## Installing Goxpyriment to create your own experiments

1. Install [Git](https://git-scm.com/install/), then [Go](https://go.dev/doc/install) on your compute (See [detailed instructions](docs/Installing-a-development-environment.md) if you are new to thistle).
2.  clone [Goxpyriment Github repository](), by opening a Terminal (App `Git Bash` under Windows), and executing the command-line 

        git clone https://github.com/chrplr/goxpyriment.git
        
   (Alternatively you can just download the [ZIP](https://github.com/chrplr/goxpyriment/archive/refs/heads/main.zip) and unzip it. The interest of cloning is that it will make upgrading to more recent versions of goxpyriment trivial, by just excuting `git pull`.)
   
3. In the Terminal, execute:

```
cd goxpyriment
make examples
```

This compiles the codes in [examples/*](../examples/). If all goes well, the `_build` folder should now contain  executable (apps) for many experiments. If not report the issue on 

The first time, it will take a while because Go needs to download several libraries. Once done, compilation will be fast.
 


### Program your own experiment

After having a look at [Getting Started][GettingStarted.md), and the examples 's [source codes](../examples/).
 the [available functions](API.md)


1. Create a folder for your experiment and start coding a `main.go` file. You can test it by running `go run main.go`. 
> [!TIP]
> *Vibe-coding:* Launch an AI coding agent (Claude, Gemini, etc.) inside the `goxpyriment` folder and ask it to add a new experiment to the `examples` folder — this leads the agent to read the existing examples for context. Describe the experiment (stimuli, design, etc.) in plain language and enjoy.
> Recommendation: save your prompt in a `description.md` file alongside the code.
2. Once satisfied with the code, compile your experiment into an executable with `go build .`. This executable will run on any machine with the same OS and architecture. 
3. If you need to distribute your experiment to colleagues who use another operating system or architecture, you can [cross-compile](https://golangcookbook.com/chapters/running/cross-compiling/). 
