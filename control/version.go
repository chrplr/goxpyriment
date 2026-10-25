// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"flag"
	"fmt"
	"os"

	"github.com/chrplr/goxpyriment/results"
)

// -version prints the version of the program and of goxpyriment, then exits.
// It is registered when the package is loaded, rather than in
// NewExperimentFromFlags, so that it also works in the programs that call
// flag.Parse themselves: it acts as soon as flag.Parse reaches it, before any
// window opens. The same two lines are written into every -info.txt.
func init() {
	flag.BoolFunc("version", "Print the version of this program and of goxpyriment, then exit", func(string) error {
		fmt.Printf("program: %s\ngoxpyriment: %s\n", results.ProgramVersion, results.Version)
		os.Exit(0)
		return nil
	})
}
