// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package results

import (
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
)

// Subject is a participant's ID: one or more ASCII letters and digits, as
// returned by ParseSubject.
//
// A numeric ID (digits only) is stored without its leading zeros, so "001",
// "01" and "1" are the same subject: they get the same seed, the same file
// names and the same value in the data, as when the ID was an int. Any other
// ID is kept as written ("AB12" and "ab12" are different subjects; on macOS
// and Windows, whose file names ignore case, their files would collide).
//
// The zero value "" is not a valid ID; NoSubject ("0") is the subject of a
// run started without one.
type Subject string

// NoSubject is the ID of a run started without one (-headless, or a browser
// page without ?s=).
const NoSubject Subject = "0"

// SubjectRule describes the IDs that ParseSubject accepts, for error messages
// and help texts.
const SubjectRule = "letters and digits only (A-Z, a-z, 0-9)"

// ParseSubject checks s and returns it as a Subject: it must be one or more
// ASCII letters and digits. A numeric ID is read in base 10 (so "010" is 10,
// not 8 as with the C convention) and loses its leading zeros; it must fit in
// an int.
func ParseSubject(s string) (Subject, error) {
	if s == "" {
		return "", errors.New("subject ID is empty")
	}
	digits := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
			digits = false
		default:
			return "", fmt.Errorf("subject ID %q: %s", s, SubjectRule)
		}
	}
	if !digits {
		return Subject(s), nil
	}
	n, err := strconv.ParseInt(s, 10, strconv.IntSize)
	if err != nil {
		return "", fmt.Errorf("subject ID %q: too large a number", s)
	}
	return Subject(strconv.FormatInt(n, 10)), nil
}

// Number returns the ID as a number, and whether it is one (digits only).
// Counterbalancing by subject (odd/even, Latin squares) needs it: an ID such
// as "AB12" has no rank, and hashing it would assign the conditions at random
// rather than in balance.
func (s Subject) Number() (int, bool) {
	if s == "" {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(string(s))
	return n, err == nil
}

// Seed returns a number derived from the ID, to seed a random generator per
// subject: the number itself for a numeric ID (the value that uint64 of the
// former int SubjectID gave, so numeric subjects keep their random
// sequences), else the FNV-1a 64-bit hash of the ID.
func (s Subject) Seed() uint64 {
	if n, ok := s.Number(); ok {
		return uint64(n)
	}
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// Label returns the ID as it appears in file names, after "sub-": a numeric
// ID padded to 3 digits ("007"), any other ID as written.
func (s Subject) Label() string {
	if n, ok := s.Number(); ok {
		return fmt.Sprintf("%03d", n)
	}
	return string(s)
}

// String returns the ID as written in the data: without leading zeros for a
// numeric ID.
func (s Subject) String() string { return string(s) }
