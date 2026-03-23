// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

// Package staircase implements adaptive psychophysical threshold estimation.
//
// All procedures implement the [Staircase] interface. The caller obtains the
// next stimulus intensity from [Staircase.Intensity], presents any stimulus
// parameterised by that value, collects a binary correct/incorrect response,
// and calls [Staircase.Update]. The staircase is completely decoupled from
// stimulus type and from SDL.
//
// Typical trial loop:
//
//	for !sc.Done() {
//	    intensity := sc.Intensity()        // get parameter for next trial
//	    correct := presentAndRespond(intensity)
//	    sc.Update(correct)
//	}
//	threshold := sc.Threshold()
//
// For multiple interleaved staircases use [Runner].
package staircase

// Trial records a single presented intensity and the participant's response.
type Trial struct {
	Intensity float64
	Correct   bool
	// Reversal is true when this trial caused a direction reversal.
	// Always false for procedures that have no notion of reversals (e.g. Quest).
	Reversal bool
}

// Staircase is the common interface for all adaptive threshold procedures.
type Staircase interface {
	// Intensity returns the stimulus intensity for the next trial.
	// Repeated calls without an intervening Update return the same value.
	Intensity() float64

	// Update records the participant's response for the current trial
	// and advances the internal state.
	Update(correct bool)

	// Done reports whether the stopping criterion has been met.
	Done() bool

	// Threshold returns the current best estimate of the perceptual threshold.
	// Valid at any time, including before Done returns true.
	Threshold() float64

	// History returns all trials recorded so far, in order.
	History() []Trial
}
