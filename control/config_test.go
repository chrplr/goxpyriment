// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import "testing"

func TestProgramName(t *testing.T) {
	for path, want := range map[string]string{
		"/home/me/DBP_MEG/bin/MEG-left-right":                  "MEG-left-right",
		`C:\Users\me\DBP_MEG\bin\MEG-left-right.exe`:           "MEG-left-right",
		"bin/MEG-left-right.EXE":                               "MEG-left-right",
		"bin/MEG-left-right.app/Contents/MacOS/MEG-left-right": "MEG-left-right",
		"/tmp/go-build123/b001/exe/demo_hello_world":           "demo_hello_world",
		"./Wisconsin-Card-Sorting-Task":                        "Wisconsin-Card-Sorting-Task",
	} {
		if got := programName(path); got != want {
			t.Errorf("programName(%q) = %q, want %q", path, got, want)
		}
	}
}
