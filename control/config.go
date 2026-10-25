// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/chrplr/goxpyriment/config"
)

// defaultConfigFile is the default of -config, set by SetDefaultConfigFile.
var defaultConfigFile string

// SetDefaultConfigFile sets the default of the -config flag of
// NewExperimentFromFlags: the TOML file the default values of the flags are
// read from (see package config). "" (the default) means no file. A program
// that keeps its lab settings in a known place calls it before
// NewExperimentFromFlags; -config on the command line still overrides it. The
// file must exist: a program that only may have one should pass "" when it
// does not.
func SetDefaultConfigFile(path string) {
	defaultConfigFile = path
}

// parseFlags parses the command line into flag.CommandLine, then sets the
// flags it did not give from the -config file, the table named after the
// program's executable taking precedence over the top-level keys. It returns
// the lines that record the file and the values taken from it, for the
// -info.txt; none without a file.
// A bad file is fatal: running with settings other than the ones the lab wrote
// down would change what the session measures without saying so.
func parseFlags() []string {
	applied, err := config.ParseFlags(flag.CommandLine, os.Args[1:], programName(os.Args[0]))
	path := flag.Lookup(config.FlagName).Value.String()
	if err != nil {
		log.Fatalf("-config %s: %v", path, err)
	}
	if path == "" {
		return nil
	}
	if abs, absErr := filepath.Abs(path); absErr == nil {
		path = abs
	}
	lines := []string{"config: " + path}
	for _, s := range applied {
		lines = append(lines, fmt.Sprintf("config flag: %s=%s", s.Name, s.Value))
	}
	for _, line := range lines {
		fmt.Fprintln(os.Stderr, line)
	}
	return lines
}

// programName is the name of the executable at path, without ".exe": the
// table of the -config file that applies to this program. It is the name
// people type and list in session files, unlike the experiment name, which may
// contain spaces.
func programName(path string) string {
	base := filepath.Base(strings.ReplaceAll(path, `\`, "/"))
	if ext := filepath.Ext(base); strings.EqualFold(ext, ".exe") {
		base = strings.TrimSuffix(base, ext)
	}
	return base
}
