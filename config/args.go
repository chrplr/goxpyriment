// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package config

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

// CheckArgs reports the two command-line mistakes that Go's flag package
// accepts without a word, given the flag set fs after it parsed args. Go stops
// parsing at the first argument that is not a flag, and the program then runs
// with the flags after it at their defaults:
//
//   - a boolean flag given its value after a space, as in -photodiode false:
//     a boolean flag reads no separate value, so -photodiode is set to true
//     and "false" is the first non-flag argument;
//   - a flag after a non-flag argument, as in list.csv -s 3, where -s is
//     never read.
//
// Other non-flag arguments are allowed: some programs take a file name. After
// "--" every argument is taken as given.
func CheckArgs(fs *flag.FlagSet, args []string) error {
	rest := fs.Args()
	if len(rest) == 0 {
		return nil
	}
	if prev := lastFlag(fs, args); prev == "--" {
		return nil
	} else if prev != "" {
		name := strings.TrimLeft(prev, "-")
		if _, err := strconv.ParseBool(rest[0]); err == nil && isBool(fs.Lookup(name)) {
			return fmt.Errorf("%s %s: a boolean flag takes its value after \"=\": write %s=%s "+
				"(as written, %s is true and the arguments from %q on are not read)",
				prev, rest[0], prev, rest[0], prev, rest[0])
		}
	}
	for _, a := range rest {
		if len(a) > 1 && a[0] == '-' {
			return fmt.Errorf("%s: flags must come before the other arguments; "+
				"after %q, Go stops reading flags, and %s would be ignored", a, rest[0], a)
		}
	}
	return nil
}

// lastFlag returns the argument of args that flag.FlagSet.Parse read last
// before the first non-flag argument: a flag without "=", or "--"; "" when it
// read none, or when the last flag had its value after "=" or as the next
// argument. It walks args the way Parse does, so the value of a flag that
// begins with "-" (-s -photodiode) is not taken for a flag.
func lastFlag(fs *flag.FlagSet, args []string) string {
	last := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) < 2 || a[0] != '-' {
			return last
		}
		if a == "--" {
			return a
		}
		name := strings.TrimPrefix(strings.TrimPrefix(a, "-"), "-")
		if strings.Contains(name, "=") {
			last = ""
			continue
		}
		if isBool(fs.Lookup(name)) {
			last = a
			continue
		}
		i++ // the next argument is the value of this flag
		last = ""
	}
	return last
}

// isBool reports whether f is a boolean flag, which reads no separate value.
func isBool(f *flag.Flag) bool {
	if f == nil {
		return false
	}
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}
