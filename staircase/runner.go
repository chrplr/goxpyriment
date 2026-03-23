// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package staircase

import (
	"math/rand"
	"time"
)

// Runner manages a set of interleaved staircases, selecting a random
// non-done staircase on each call to Next.
type Runner struct {
	staircases []Staircase
	rng        *rand.Rand
}

// NewRunner returns a Runner that interleaves the provided staircases.
// Pass nil for rng to use a time-seeded source.
func NewRunner(rng *rand.Rand, staircases ...Staircase) *Runner {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	scs := make([]Staircase, len(staircases))
	copy(scs, staircases)
	return &Runner{staircases: scs, rng: rng}
}

// Done reports whether all managed staircases are done.
func (r *Runner) Done() bool {
	for _, sc := range r.staircases {
		if !sc.Done() {
			return false
		}
	}
	return true
}

// Next returns a randomly selected non-done staircase.
// Panics if all staircases are done — check Done first.
func (r *Runner) Next() Staircase {
	var active []Staircase
	for _, sc := range r.staircases {
		if !sc.Done() {
			active = append(active, sc)
		}
	}
	if len(active) == 0 {
		panic("staircase: Runner.Next called when all staircases are done")
	}
	return active[r.rng.Intn(len(active))]
}

// All returns all staircases in the order they were passed to NewRunner.
func (r *Runner) All() []Staircase { return r.staircases }
