## Installing Goxpyriment to create your own experiments

1. Install [Git](https://git-scm.com/install/), then [Go](https://go.dev/doc/install) on your compute (if you are new to this, consult the [detailed instructions](Installing-a-development-environment.md)).
2.  clone [goxpyriment Github repository](http://github.com/chrplr/goxpyriment), by opening a Terminal (App `Git Bash` under Windows), and executing the command-line 

        git clone https://github.com/chrplr/goxpyriment.git

    Later, a simple `git pull` will suffice to upgrade to the most recent version. 

    Alternatively you can just download the [ZIP](https://github.com/chrplr/goxpyriment/archive/refs/heads/main.zip) and unzip it. 

   
3. In the Terminal, execute:

   ```
   cd goxpyriment
   ./build-all.sh
   ```

   This compiles the codes in [examples/*](https://github.com/chrplr/goxpyriment/tree/main/examples). If all goes well, the `_build` folder should now contain  executable (apps) for many experiments. (On Linux/macOS you can also use `make all`.)

   The first time, it will take a while because Go needs to download several libraries. Once done, compilation will be fast.


### Program your own experiment

Once goxpyriment is installed, follow the step-by-step guide in
[Creating Your Own Experiment](CreatingYourOwnExperiment.md), which walks you
through writing, running, building, and sharing your own experiment. For
background and reference, see [Getting Started](GettingStarted.md), the examples'
[source codes](https://github.com/chrplr/goxpyriment/tree/main/examples), and the
[available functions](API.md).
