// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

import (
	"strings"
	"testing"
)

func TestGvMarkTable(t *testing.T) {
	table, err := gvMarkTable([]GvMark{{Frame: 0, Code: 1}, {Frame: 186, Code: 0}}, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(table) != 2 || table[0] != 1 {
		t.Errorf("table = %v", table)
	}
	// Code 0 is a mark too (the square alone): it must be present, not absent.
	if code, ok := table[186]; !ok || code != 0 {
		t.Errorf("frame 186: code %d present %v, want a code-0 mark", code, ok)
	}
	if table, err := gvMarkTable(nil, 200); table != nil || err != nil {
		t.Errorf("no marks: %v, %v; want nil, nil", table, err)
	}
}

// A mark that cannot land must stop playback before it starts: silently
// playing an unmarked video wastes the recording.
func TestGvMarkTableRejects(t *testing.T) {
	for _, c := range []struct {
		name  string
		marks []GvMark
		want  string
	}{
		{"past the end", []GvMark{{Frame: 200}}, "frames 0-199"},
		{"negative", []GvMark{{Frame: -1}}, "frames 0-199"},
		{"twice", []GvMark{{Frame: 5, Code: 1}, {Frame: 5, Code: 2}}, "marked twice"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := gvMarkTable(c.marks, 200)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want one mentioning %q", err, c.want)
			}
		})
	}
}
