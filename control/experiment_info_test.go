// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"os"
	"strings"
	"testing"

	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/results"
)

// TestAddExperimentInfoReachesInfoFile checks that AddExperimentInfo lines end
// up in the -info.txt, both those added before the data file exists (written
// by Initialize) and those added mid-session (written at once, before the end
// time). It used to append to Design.ExperimentInfo only, which nothing wrote
// out, so the metadata was silently lost.
func TestAddExperimentInfoReachesInfoFile(t *testing.T) {
	exp := &Experiment{Design: design.NewExperiment("test")}
	exp.AddExperimentInfo("protocol: before")

	df, err := results.NewDataFile(t.TempDir(), 999, "test")
	if err != nil {
		t.Fatalf("NewDataFile: %v", err)
	}
	exp.Data = df
	exp.writePendingExperimentInfo() // as Initialize does once e.Data is set

	exp.AddExperimentInfo("limit: during")
	exp.AddExperimentInfo("two\nlines")
	df.WriteEndTime()
	if err := df.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(df.InfoFile.FullPath)
	if err != nil {
		t.Fatalf("reading info file: %v", err)
	}
	got := string(raw)
	want := []string{"# e protocol: before\n", "# e limit: during\n", "# e two\n", "# e lines\n", "# e end_time: "}
	last := -1
	for _, w := range want {
		i := strings.Index(got, w)
		if i < 0 {
			t.Fatalf("info file lacks %q:\n%s", w, got)
		}
		if i < last {
			t.Errorf("%q is out of order:\n%s", w, got)
		}
		last = i
	}
	if n := strings.Count(got, "protocol: before"); n != 1 {
		t.Errorf("pre-Initialize line written %d times, want 1", n)
	}
	if len(exp.Design.ExperimentInfo) != 3 {
		t.Errorf("Design.ExperimentInfo = %q, want the 3 entries", exp.Design.ExperimentInfo)
	}
}
