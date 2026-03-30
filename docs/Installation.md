## Installing Goxpyriment to create your own experiments

1. Install [Git](https://git-scm.com/install/), then [Go](https://go.dev/doc/install) on your compute (if you are new to this, consult the [detailed instructions](Installing-a-development-environment.md)).
2.  clone [goxpyriment Github repository](http://github.com/chrplr/goxpyriment), by opening a Terminal (App `Git Bash` under Windows), and executing the command-line 

        git clone https://github.com/chrplr/goxpyriment.git

    Later, a simple `git pull` will suffice to upgrade to the most recent version. 
        
   Alternatively you can just download the [ZIP](https://github.com/chrplr/goxpyriment/archive/refs/heads/main.zip) and unzip it. 

   
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


* Create a folder for your experiment and start coding a `main.go` file. You can test it by running `go run main.go`. 

> 💡 **TIP**                                                                                                 
> *Vibe-coding:* Launch an AI coding agent (Claude, Gemini, etc.) inside the `goxpyriment` folder and ask it to add a new experiment to the `examples` folder — this leads the agent to read the existing examples for context. Describe the experiment (stimuli, design, etc.) in plain language and enjoy.  Recommendation: save your prompt in a `description.md` file.


* Once satisfied with the code, compile your experiment into an executable with `go build .`. This executable will run on any machine with the same OS and architecture. 

* If you need to distribute your experiment to colleagues who use another operating system or architecture, you can [cross-compile](https://golangcookbook.com/chapters/running/cross-compiling/). 
