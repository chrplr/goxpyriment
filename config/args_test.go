// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package config

import (
	"flag"
	"strings"
	"testing"
)

func TestCheckArgs(t *testing.T) {
	tests := []struct {
		args    []string
		wantErr string // "" = no error
	}{
		{nil, ""},
		{[]string{"-photodiode=false", "-s", "3"}, ""},
		{[]string{"-s", "3", "list.csv"}, ""},
		{[]string{"list.csv", "other.csv"}, ""},
		{[]string{"-photodiode", "list.csv"}, ""},
		{[]string{"-photodiode", "--", "-weird-name.csv"}, ""},
		{[]string{"-s", "3", "--", "false"}, ""},
		// The value of -s is "-photodiode", not a flag: "false" is a file name.
		{[]string{"-s", "-photodiode", "false"}, ""},
		{[]string{"-photodiode", "false"}, "-photodiode false: a boolean flag takes its value after \"=\": write -photodiode=false"},
		{[]string{"-w", "--photodiode", "0", "-s", "3"}, "--photodiode 0: a boolean flag"},
		{[]string{"-photodiode", "true"}, "-photodiode true: a boolean flag"},
		{[]string{"list.csv", "-s", "3"}, "-s: flags must come before the other arguments"},
		{[]string{"-s", "3", "list.csv", "-w"}, "-w: flags must come before"},
	}
	for _, tt := range tests {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(new(strings.Builder))
		fs.Bool("photodiode", true, "")
		fs.Bool("w", false, "")
		fs.String("s", "", "")
		if err := fs.Parse(tt.args); err != nil {
			t.Fatalf("%q: Parse: %v", tt.args, err)
		}
		err := CheckArgs(fs, tt.args)
		switch {
		case tt.wantErr == "" && err != nil:
			t.Errorf("%q: unexpected error %v", tt.args, err)
		case tt.wantErr != "" && err == nil:
			t.Errorf("%q: no error, want %q", tt.args, tt.wantErr)
		case tt.wantErr != "" && !strings.HasPrefix(err.Error(), tt.wantErr):
			t.Errorf("%q: error %q, want it to begin with %q", tt.args, err, tt.wantErr)
		}
	}
}
