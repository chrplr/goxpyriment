// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"log"
	"strings"

	"github.com/chrplr/goxpyriment/results"
)

// Subject is re-exported from results so callers need not import that package.
// It is the type of exp.Subject: a participant ID of letters and digits.
//
//   - exp.Subject.Seed() seeds a random generator per subject (a numeric ID
//     gives its own number, as the former int SubjectID did).
//   - exp.Subject.Number() counterbalances by subject number; it fails for an
//     ID that is not a number, which the program should then refuse.
//   - exp.Subject.Label() is the ID in file names (sub-007, sub-AB12).
//   - exp.Subject itself prints as the ID in the data (7, AB12).
type Subject = results.Subject

// NoSubject is the ID of a run started without one: "0".
const NoSubject = results.NoSubject

// SubjectRule describes the IDs that ParseSubject accepts, for help texts.
const SubjectRule = results.SubjectRule

// ParseSubject checks a participant ID (letters and digits) and returns it as a
// Subject; a numeric ID loses its leading zeros. See results.ParseSubject.
func ParseSubject(s string) (Subject, error) { return results.ParseSubject(s) }

// SubjectFromInfo returns the subject of the "subject_id" field of info, as
// GetParticipantInfo returns it: NoSubject ("0") when the field is empty or
// absent (as under -headless, which returns the defaults), else the field as
// checked by ParseSubject. An invalid ID is an error, never subject 0.
func SubjectFromInfo(info map[string]string) (Subject, error) {
	raw := strings.TrimSpace(info["subject_id"])
	if raw == "" {
		return results.NoSubject, nil
	}
	return results.ParseSubject(raw)
}

// SubjectNumber returns the subject ID as a number, for a design that
// counterbalances by subject number (odd/even, groups of four, Latin
// squares). An ID that is not a number, such as "AB12", has no rank: the
// program stops with an error that says so, rather than assign the conditions
// out of balance.
func (e *Experiment) SubjectNumber() int {
	n, ok := e.Subject.Number()
	if !ok {
		msg := "subject ID %q is not a number, and %s assigns the conditions by subject number"
		if e.Screen != nil {
			e.Fatal(msg, e.Subject, e.Name)
		}
		log.Fatalf(msg, e.Subject, e.Name)
	}
	return n
}
