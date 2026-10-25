// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Package config reads default values for command-line flags from a TOML
// file, so that the settings of a lab (which monitor, which TTL device, which
// keys the response box sends) are written once rather than on every command
// line. It is what -config does in control.NewExperimentFromFlags, and it does
// not depend on SDL, so a launcher can read the same file.
//
// Keys are flag names without the dash. Top-level keys apply to every program
// that defines the flag; a table named after a program applies to it alone and
// overrides the top-level keys:
//
//	d = 1
//	ttl = "parallel:/dev/parport1"
//	left-key = "b"
//
//	[MEG-Reading-Sentences]
//	resp-key = "1"
//
// A value given on the command line always wins over the file.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/pelletier/go-toml/v2"
	"github.com/peterbourgon/ff/v3"
)

// FlagName is the flag that ParseFlags reads the path of the file from.
const FlagName = "config"

// Setting is one flag value that ParseFlags took from the file.
type Setting struct {
	Name, Value string
}

// forbidden are the keys a file may not set: the file itself, the subject ID
// (a property of the session, never of the lab; and -s set from a file would
// count as given and skip the setup dialog), and the flags that act as soon as
// they are set.
var forbidden = map[string]bool{
	FlagName:  true,
	"s":       true,
	"version": true,
	"h":       true,
	"help":    true,
}

// Load reads the file at path and returns its top-level keys merged with
// those of the table named table, which take precedence. Values are returned
// as the strings a flag would be set to. table "" means the top-level keys
// only.
func Load(path, table string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	global, section, err := decode(f, table)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return merge(global, section), nil
}

// ParseFlags parses args into fs, then sets the flags not given in args from
// the file named by fs's -config flag, which the caller must define (empty
// means no file). table names the program, whose table in the file overrides
// the top-level keys. It returns the settings taken from the file, in name
// order.
//
// A top-level key that fs does not define is ignored: it is meant for another
// program. A key in the program's own table that fs does not define is an
// error, and so is a missing file.
func ParseFlags(fs *flag.FlagSet, args []string, table string) ([]Setting, error) {
	var applied []Setting
	parser := func(r io.Reader, set func(name, value string) error) error {
		global, section, err := decode(r, table)
		if err != nil {
			return err
		}
		for name := range section {
			if fs.Lookup(name) == nil {
				return fmt.Errorf("[%s] %s: this program has no flag -%s", table, name, name)
			}
		}
		// Flags given on the command line, which the file must not override.
		given := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { given[f.Name] = true })

		values := merge(global, section)
		for _, name := range sortedKeys(values) {
			if fs.Lookup(name) == nil || given[name] {
				continue
			}
			if err := set(name, values[name]); err != nil {
				return err
			}
			applied = append(applied, Setting{name, values[name]})
		}
		return nil
	}
	err := ff.Parse(fs, args,
		ff.WithConfigFileFlag(FlagName),
		ff.WithConfigFileParser(parser))
	return applied, err
}

// decode reads a TOML document and returns its top-level keys and the keys of
// its table named table, as flag values. Tables of other programs are not
// looked into.
func decode(r io.Reader, table string) (global, section map[string]string, err error) {
	var doc map[string]any
	if err := toml.NewDecoder(r).Decode(&doc); err != nil {
		var derr *toml.DecodeError
		if errors.As(err, &derr) {
			row, _ := derr.Position()
			return nil, nil, fmt.Errorf("line %d: %w", row, err)
		}
		return nil, nil, err
	}
	global = map[string]string{}
	section = map[string]string{}
	for key, v := range doc {
		sub, isTable := v.(map[string]any)
		switch {
		case !isTable:
			if global[key], err = value(key, v); err != nil {
				return nil, nil, err
			}
		case key == table && table != "":
			for k, sv := range sub {
				if section[k], err = value(k, sv); err != nil {
					return nil, nil, fmt.Errorf("[%s] %w", table, err)
				}
			}
		}
	}
	return global, section, nil
}

// value converts the TOML value of key to the string a flag is set to.
func value(key string, v any) (string, error) {
	if forbidden[key] {
		return "", fmt.Errorf("%s: cannot be set from a configuration file", key)
	}
	switch v := v.(type) {
	case string:
		return v, nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return "", fmt.Errorf("%s: a flag value must be a string, a number or a boolean, not %T", key, v)
	}
}

// merge returns the top-level values overridden by those of the table.
func merge(global, section map[string]string) map[string]string {
	m := make(map[string]string, len(global)+len(section))
	for k, v := range global {
		m[k] = v
	}
	for k, v := range section {
		m[k] = v
	}
	return m
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
