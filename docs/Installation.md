## Installing Go and Goxpyriment to create your own experiments

1. Install [Go](https://go.dev/doc/install) on your computer (if you are new to this, consult the [detailed instructions](Installing-a-development-environment.md))
2. Either:

    * Clone [goxpyriment Github repository](http://github.com/chrplr/goxpyriment), by opening a Terminal (App `Git Bash` under Windows), and executing the command-line

         git clone https://github.com/chrplr/goxpyriment.git

       Later, a simple `git pull` will suffice to upgrade goxpyriement to the latest version.

    * Or download and unzip [main.zip](https://github.com/chrplr/goxpyriment/archive/refs/heads/main.zip). 


3. On a command line in a Terminal, navigate to the `goxpyriment` folder and execute:

   ```
   ./build-all.sh
   ```

   (Note: On Linux/macOS you can instead use `make all`)

   This will compile the codes in [examples/*](https://github.com/chrplr/goxpyriment/tree/main/examples) and [test/*](https://github.com/chrplr/goxpyriment/tree/main/examples)
   (Note: The first time, this will take a while because Go needs to download several libraries. Once this is done, future compilations will be fast.)

   After this operation, a new `_build` folder should contain many binaries (executable apps), that you can either run from the command line, or launch by clicking on their icon in a the file browser.  


### Program your own experiments

Once goxpyriment is installed, follow the step-by-step guide in
[Creating Your Own Experiment](CreatingYourOwnExperiment.md), which walks you
through writing, running, building, and sharing your own experiment. For
background and reference, see [Getting Started](GettingStarted.md), the examples'
[source codes](https://github.com/chrplr/goxpyriment/tree/main/examples), and the
[available functions](API.md).
