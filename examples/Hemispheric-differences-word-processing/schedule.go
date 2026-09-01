// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"math/rand"
	"sort"
)

// EventKind distinguishes the two kinds of presentation in the stream.
type EventKind int

const (
	// KindStudy is a lateralized presentation (LVF or RVF) to be memorized.
	// No response is collected.
	KindStudy EventKind = iota
	// KindTest is a central presentation calling for an old/new judgement.
	KindTest
)

func (k EventKind) String() string {
	if k == KindStudy {
		return "study"
	}
	return "test"
}

// Event is one presentation in the continuous recognition stream.
//
// The stream intermingles study and test presentations, as in Federmeier &
// Benjamin (2005): a studied word appears laterally once (KindStudy) and again
// centrally (KindTest) exactly Lag presentations later, while unstudied words
// appear centrally once and only once.
type Event struct {
	Kind   EventKind
	Word   string
	PairID int    // links a study event to its test event; -1 for unstudied words
	VF     string // "LVF" or "RVF" for a studied word's two events; "" for unstudied
	Lag    int    // presentations between study and test; 0 for study events and unstudied words
	IsOld  bool   // for test events: was this word studied earlier in the stream?
}

// StreamSpec describes the composition of a stream.
type StreamSpec struct {
	Lags   []int // lag conditions, in presentations since study
	PerLag int   // studied words per lag condition; must be even (split LVF/RVF)
	NNew   int   // unstudied words, tested once each
}

// PaperSpec is the design of Federmeier & Benjamin (2005), p. 995: nine lag
// conditions, 16 studied words each (8 per visual field), plus 112 unstudied
// words — 144 study + 144 old-test + 112 new-test = 400 presentations drawn
// from 256 distinct words.
var PaperSpec = StreamSpec{
	Lags:   []int{1, 2, 3, 5, 7, 10, 20, 30, 50},
	PerLag: 16,
	NNew:   112,
}

// ShortSpec is a reduced version for development: every lag condition is kept
// (so the scheduler is exercised end to end, lag 50 included) but with two
// studied words each — 18 study + 18 old-test + 30 new-test = 66 presentations,
// roughly three and a half minutes.
var ShortSpec = StreamSpec{
	Lags:   []int{1, 2, 3, 5, 7, 10, 20, 30, 50},
	PerLag: 2,
	NNew:   30,
}

// PracticeSpec is the short warm-up run the paper gave before the experimental
// list ("a short practice list (using proper names)"): 20 presentations.
var PracticeSpec = StreamSpec{
	Lags:   []int{1, 3, 7},
	PerLag: 2,
	NNew:   8,
}

// NStudied is the number of words presented laterally and then tested.
func (s StreamSpec) NStudied() int { return len(s.Lags) * s.PerLag }

// NWords is the number of distinct words the spec consumes.
func (s StreamSpec) NWords() int { return s.NStudied() + s.NNew }

// Len is the number of presentations in the stream.
func (s StreamSpec) Len() int { return 2*s.NStudied() + s.NNew }

// maxPlacementAttempts bounds the restart loop in BuildStream. Placement
// failures are rare (see BuildStream), so a run that exhausts this is a sign
// the spec itself is infeasible rather than bad luck.
const maxPlacementAttempts = 2000

// BuildStream lays out one continuous-recognition stream.
//
// Every studied word occupies two slots — a lateralized study presentation at
// some position p and a central test presentation at p+lag — and the unstudied
// words fill what is left. Lag is therefore counted in *presentations*, which
// is what the paper's "number of words since study" means: at lag 1 the test is
// the very next thing on the screen (the paper's "immediate repetition").
//
// Placement is randomized with restarts. Pairs are placed longest lag first,
// because a long lag has the fewest legal positions and must claim them while
// the stream is still empty; each pair then takes a position drawn uniformly
// from those still free. In the paper's design the pairs occupy 288 of 400
// slots, and the lag-1 pairs placed last need 16 disjoint adjacent free slots
// among the ~144 that remain, where dozens exist — so a restart is rarely
// needed at all.
//
// rng makes the whole layout reproducible: the same seed yields the same
// stream, which is what lets two participants share a list (see main.go).
func BuildStream(words []string, spec StreamSpec, rng *rand.Rand) ([]Event, error) {
	if spec.PerLag%2 != 0 {
		return nil, fmt.Errorf("BuildStream: PerLag must be even to split LVF/RVF, got %d", spec.PerLag)
	}
	if len(spec.Lags) == 0 || spec.PerLag <= 0 {
		return nil, fmt.Errorf("BuildStream: empty design (%d lags × %d per lag)", len(spec.Lags), spec.PerLag)
	}
	if need, have := spec.NWords(), len(words); have < need {
		return nil, fmt.Errorf("BuildStream: need %d distinct words, got %d", need, have)
	}
	n := spec.Len()
	for _, lag := range spec.Lags {
		if lag >= n {
			return nil, fmt.Errorf("BuildStream: lag %d does not fit in a %d-presentation stream", lag, n)
		}
	}

	// Draw the words for this stream, and split them into studied and unstudied.
	// A word is never both: the shuffled pool is cut once.
	pool := append([]string(nil), words...)
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	studiedWords := pool[:spec.NStudied()]
	newWords := pool[spec.NStudied():spec.NWords()]

	type pair struct {
		lag int
		vf  string
	}
	pairs := make([]pair, 0, spec.NStudied())
	for _, lag := range spec.Lags {
		for i := 0; i < spec.PerLag; i++ {
			vf := "LVF"
			if i >= spec.PerLag/2 {
				vf = "RVF"
			}
			pairs = append(pairs, pair{lag: lag, vf: vf})
		}
	}

	studyAt := make([]int, len(pairs))
	occupied := make([]bool, n)
	candidates := make([]int, 0, n)

	for attempt := 0; attempt < maxPlacementAttempts; attempt++ {
		// Random order within a lag, then longest lag first. Shuffling before
		// the stable sort is what varies which pair of a lag gets first pick.
		rng.Shuffle(len(pairs), func(i, j int) { pairs[i], pairs[j] = pairs[j], pairs[i] })
		sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].lag > pairs[j].lag })

		for i := range occupied {
			occupied[i] = false
		}

		placed := true
		for i, p := range pairs {
			candidates = candidates[:0]
			for q := 0; q+p.lag < n; q++ {
				if !occupied[q] && !occupied[q+p.lag] {
					candidates = append(candidates, q)
				}
			}
			if len(candidates) == 0 {
				placed = false
				break
			}
			q := candidates[rng.Intn(len(candidates))]
			occupied[q] = true
			occupied[q+p.lag] = true
			studyAt[i] = q
		}
		if !placed {
			continue
		}

		events := make([]Event, n)
		for i, p := range pairs {
			word := studiedWords[i]
			s, t := studyAt[i], studyAt[i]+p.lag
			events[s] = Event{Kind: KindStudy, Word: word, PairID: i, VF: p.vf}
			events[t] = Event{Kind: KindTest, Word: word, PairID: i, VF: p.vf, Lag: p.lag, IsOld: true}
		}
		next := 0
		for i := range events {
			if occupied[i] {
				continue
			}
			events[i] = Event{Kind: KindTest, Word: newWords[next], PairID: -1}
			next++
		}
		if next != len(newWords) {
			return nil, fmt.Errorf("BuildStream: filled %d free slots but have %d unstudied words", next, len(newWords))
		}
		return events, nil
	}

	return nil, fmt.Errorf("BuildStream: no valid layout for %d pairs in %d slots after %d attempts",
		len(pairs), n, maxPlacementAttempts)
}
