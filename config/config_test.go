// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package config

import (
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// flags is a flag set like a program's, with the -config flag set to path.
type flags struct {
	fs       *flag.FlagSet
	d        *int
	ttl      *string
	ttlMs    *float64
	photo    *bool
	key      *string
	duration *time.Duration
}

func newFlags(path string) flags {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	fs.String(FlagName, path, "configuration file")
	fs.Int("s", 0, "subject ID")
	return flags{
		fs:       fs,
		d:        fs.Int("d", -1, "display"),
		ttl:      fs.String("ttl", "", "TTL device"),
		ttlMs:    fs.Float64("ttl-ms", 0, "pulse length"),
		photo:    fs.Bool("photodiode", false, "photodiode square"),
		key:      fs.String("left-key", "left", "left key"),
		duration: fs.Duration("max-duration", 0, "time limit"),
	}
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "site.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseFlagsValueTypes(t *testing.T) {
	path := writeFile(t, `
d = 1
ttl = "parallel:/dev/parport1"
ttl-ms = 2.5
photodiode = true
max-duration = "10m"
`)
	f := newFlags(path)
	applied, err := ParseFlags(f.fs, nil, "prog")
	if err != nil {
		t.Fatal(err)
	}
	if *f.d != 1 || *f.ttl != "parallel:/dev/parport1" || *f.ttlMs != 2.5 || !*f.photo || *f.duration != 10*time.Minute {
		t.Errorf("got d=%d ttl=%q ttl-ms=%g photodiode=%v max-duration=%v",
			*f.d, *f.ttl, *f.ttlMs, *f.photo, *f.duration)
	}
	want := []Setting{
		{"d", "1"}, {"max-duration", "10m"}, {"photodiode", "true"},
		{"ttl", "parallel:/dev/parport1"}, {"ttl-ms", "2.5"},
	}
	if !reflect.DeepEqual(applied, want) {
		t.Errorf("applied = %v, want %v", applied, want)
	}
}

func TestParseFlagsCommandLineWins(t *testing.T) {
	path := writeFile(t, "d = 1\nttl = \"null\"\n")
	f := newFlags(path)
	applied, err := ParseFlags(f.fs, []string{"-d", "2"}, "prog")
	if err != nil {
		t.Fatal(err)
	}
	if *f.d != 2 || *f.ttl != "null" {
		t.Errorf("d=%d ttl=%q, want 2 and null", *f.d, *f.ttl)
	}
	if want := []Setting{{"ttl", "null"}}; !reflect.DeepEqual(applied, want) {
		t.Errorf("applied = %v, want %v (not d, given on the command line)", applied, want)
	}
}

func TestParseFlagsConfigOnCommandLine(t *testing.T) {
	path := writeFile(t, "d = 3\n")
	f := newFlags("")
	if _, err := ParseFlags(f.fs, []string{"-config", path}, "prog"); err != nil {
		t.Fatal(err)
	}
	if *f.d != 3 {
		t.Errorf("d = %d, want 3", *f.d)
	}
}

// A program's table overrides the top-level keys, whatever the order in
// which the decoded map is visited; another program's table is not used.
func TestParseFlagsTables(t *testing.T) {
	path := writeFile(t, `
d = 1
left-key = "a"

[prog]
left-key = "b"

[other]
d = 5
left-key = "c"
unknown-flag = 1
`)
	for i := 0; i < 50; i++ {
		f := newFlags(path)
		if _, err := ParseFlags(f.fs, nil, "prog"); err != nil {
			t.Fatal(err)
		}
		if *f.key != "b" || *f.d != 1 {
			t.Fatalf("run %d: left-key=%q d=%d, want b and 1", i, *f.key, *f.d)
		}
	}
}

func TestParseFlagsUnknownKeys(t *testing.T) {
	// A top-level key that this program does not define belongs to another.
	f := newFlags(writeFile(t, "right-key = \"y\"\nd = 1\n"))
	applied, err := ParseFlags(f.fs, nil, "prog")
	if err != nil {
		t.Fatalf("unknown top-level key: %v", err)
	}
	if want := []Setting{{"d", "1"}}; !reflect.DeepEqual(applied, want) {
		t.Errorf("applied = %v, want %v", applied, want)
	}

	// In the program's own table, it is a mistake.
	f = newFlags(writeFile(t, "[prog]\ntll = \"null\"\n"))
	if _, err := ParseFlags(f.fs, nil, "prog"); err == nil || !strings.Contains(err.Error(), "tll") {
		t.Errorf("unknown key in the program's table: err = %v, want an error naming tll", err)
	}
}

func TestParseFlagsRefusals(t *testing.T) {
	for name, content := range map[string]string{
		"subject":         "s = 3\n",
		"subject (table)": "[prog]\ns = 3\n",
		"config":          "config = \"other.toml\"\n",
		"version":         "version = true\n",
		"help":            "help = true\n",
		"array":           "d = [1, 2]\n",
		"date":            "d = 2026-10-25\n",
		"nested table":    "[prog.sub]\nd = 1\n",
		"bad value":       "d = \"one\"\n",
		"bad syntax":      "d = \n",
	} {
		f := newFlags(writeFile(t, content))
		if _, err := ParseFlags(f.fs, nil, "prog"); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestParseFlagsSyntaxErrorLine(t *testing.T) {
	f := newFlags(writeFile(t, "d = 1\nttl = \n"))
	if _, err := ParseFlags(f.fs, nil, "prog"); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("err = %v, want one naming line 2", err)
	}
}

func TestParseFlagsMissingFile(t *testing.T) {
	f := newFlags(filepath.Join(t.TempDir(), "absent.toml"))
	if _, err := ParseFlags(f.fs, nil, "prog"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want a not-exist error", err)
	}
}

func TestParseFlagsNoFile(t *testing.T) {
	f := newFlags("")
	applied, err := ParseFlags(f.fs, []string{"-d", "2"}, "prog")
	if err != nil || applied != nil || *f.d != 2 {
		t.Errorf("applied=%v err=%v d=%d, want nil, nil, 2", applied, err, *f.d)
	}
}

func TestLoad(t *testing.T) {
	path := writeFile(t, `
d = 1
ttl = "null"

[prog]
ttl = "parallel:/dev/parport1"
`)
	got, err := Load(path, "prog")
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"d": "1", "ttl": "parallel:/dev/parport1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Load(prog) = %v, want %v", got, want)
	}
	got, err = Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"d": "1", "ttl": "null"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Load(\"\") = %v, want %v", got, want)
	}
}
