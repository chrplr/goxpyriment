// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import "testing"

func TestSubjectFromInfo(t *testing.T) {
	for _, c := range []struct {
		info map[string]string
		want Subject
	}{
		{nil, NoSubject},
		{map[string]string{"subject_id": ""}, NoSubject},
		{map[string]string{"subject_id": "  "}, NoSubject},
		{map[string]string{"subject_id": "007"}, "7"},
		{map[string]string{"subject_id": " AB12 "}, "AB12"},
	} {
		got, err := SubjectFromInfo(c.info)
		if err != nil || got != c.want {
			t.Errorf("SubjectFromInfo(%v) = %q, %v; want %q", c.info, got, err, c.want)
		}
	}
	for _, raw := range []string{"sub-1", "a b", "x/y"} {
		if got, err := SubjectFromInfo(map[string]string{"subject_id": raw}); err == nil {
			t.Errorf("SubjectFromInfo(%q) = %q, want an error", raw, got)
		}
	}
}

func TestExperimentSubjectNumber(t *testing.T) {
	e := &Experiment{Subject: "12"}
	if n := e.SubjectNumber(); n != 12 {
		t.Errorf("SubjectNumber() = %d, want 12", n)
	}
	if NewExperiment("x", 0, 0, false, Black, White, 12).Subject != NoSubject {
		t.Error("NewExperiment: Subject is not NoSubject")
	}
}
