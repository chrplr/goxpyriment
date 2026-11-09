// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package results

import (
	"fmt"
	"strconv"
	"testing"
)

func TestParseSubject(t *testing.T) {
	for _, c := range []struct {
		in, want string
	}{
		{"1", "1"},
		{"001", "1"},
		{"010", "10"}, // base 10, not octal
		{"008", "8"},
		{"0", "0"},
		{"000", "0"},
		{"AB12", "AB12"},
		{"ab12", "ab12"},
		{"P007", "P007"},
		{"x", "x"},
	} {
		got, err := ParseSubject(c.in)
		if err != nil || string(got) != c.want {
			t.Errorf("ParseSubject(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	for _, in := range []string{"", " 1", "1 ", "sub-001", "a_b", "a.b", "../x", "é", "-1", "+1",
		"a,b", "99999999999999999999999"} {
		if got, err := ParseSubject(in); err == nil {
			t.Errorf("ParseSubject(%q) = %q, want an error", in, got)
		}
	}
}

// TestSubjectNumericCompatible pins what numeric IDs give, against what the
// former int SubjectID gave: the seed (uint64(id)), the file label (%03d) and
// the value in the data and in "s id:" (%d).
func TestSubjectNumericCompatible(t *testing.T) {
	for n := 0; n <= 100000; n++ {
		s, err := ParseSubject(strconv.Itoa(n))
		if err != nil {
			t.Fatalf("ParseSubject(%d): %v", n, err)
		}
		if got, ok := s.Number(); !ok || got != n {
			t.Fatalf("%d: Number() = %d, %v", n, got, ok)
		}
		if s.Seed() != uint64(n) {
			t.Fatalf("%d: Seed() = %d", n, s.Seed())
		}
		if want := fmt.Sprintf("%03d", n); s.Label() != want {
			t.Fatalf("%d: Label() = %q, want %q", n, s.Label(), want)
		}
		if want := fmt.Sprintf("%d", n); s.String() != want || fmt.Sprint(s) != want {
			t.Fatalf("%d: String() = %q, want %q", n, s.String(), want)
		}
	}
}

func TestSubjectAlphanumeric(t *testing.T) {
	s, err := ParseSubject("AB12")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Number(); ok {
		t.Error("AB12: Number() ok, want not a number")
	}
	if s.Label() != "AB12" || s.String() != "AB12" {
		t.Errorf("AB12: Label() = %q, String() = %q", s.Label(), s.String())
	}
	// FNV-1a 64 of "AB12", fixed so that a change of hash shows.
	if s.Seed() != 0x9296fa8ba4ac6709 {
		t.Errorf("AB12: Seed() = %#x", s.Seed())
	}
	other, _ := ParseSubject("ab12")
	if s.Seed() == other.Seed() {
		t.Error("AB12 and ab12 have the same seed")
	}
	if _, ok := Subject("").Number(); ok {
		t.Error(`"": Number() ok`)
	}
}
