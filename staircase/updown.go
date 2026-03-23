// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package staircase

import "math"

// UpDownConfig configures a classical transformed up-down staircase
// (Levitt 1971).
type UpDownConfig struct {
	// StartIntensity is the initial stimulus intensity.
	StartIntensity float64
	// MinIntensity is the lower bound; intensity will never go below this.
	MinIntensity float64
	// MaxIntensity is the upper bound; intensity will never exceed this.
	MaxIntensity float64

	// StepUp is the step size applied on an incorrect response (toward louder / stronger).
	StepUp float64
	// StepDown is the step size applied after NCorrectDown consecutive correct
	// responses (toward quieter / weaker).
	StepDown float64

	// NCorrectDown is the number of consecutive correct responses required
	// before stepping down. Common values and their threshold targets:
	//   1 → 1-up/1-down  → ~50 %
	//   2 → 1-up/2-down  → ~70.7 %
	//   3 → 1-up/3-down  → ~79.4 %
	// Defaults to 1 when zero.
	NCorrectDown int

	// Phase2StepUp and Phase2StepDown, if non-zero, replace StepUp / StepDown
	// after Phase2StartReversal reversals have been recorded. Leave both at zero
	// for a single-phase staircase.
	Phase2StepUp        float64
	Phase2StepDown      float64
	Phase2StartReversal int

	// MaxReversals is the primary stopping criterion: the staircase ends when
	// this many direction reversals have been recorded. Zero disables it.
	MaxReversals int
	// MaxTrials is an alternative stopping criterion. Zero disables it.
	MaxTrials int

	// NReversalsForThreshold is the number of trailing reversals used to
	// compute the threshold estimate (mean of last N reversal intensities).
	// Zero means use all reversals.
	NReversalsForThreshold int
}

// UpDown implements a classical transformed up-down staircase.
type UpDown struct {
	cfg           UpDownConfig
	current       float64
	direction     int // 0 = undefined, +1 = going up, -1 = going down
	consecCorrect int
	reversals     []float64 // intensity at each reversal
	trials        []Trial
}

// NewUpDown returns an UpDown staircase with the given configuration.
func NewUpDown(cfg UpDownConfig) *UpDown {
	if cfg.NCorrectDown <= 0 {
		cfg.NCorrectDown = 1
	}
	return &UpDown{
		cfg:     cfg,
		current: cfg.StartIntensity,
	}
}

// Intensity returns the stimulus intensity for the next trial.
func (sc *UpDown) Intensity() float64 { return sc.current }

// Update records the participant's response and steps the intensity.
func (sc *UpDown) Update(correct bool) {
	presented := sc.current
	reversal := false

	if correct {
		sc.consecCorrect++
		if sc.consecCorrect < sc.cfg.NCorrectDown {
			// Need more consecutive hits before stepping down.
			sc.trials = append(sc.trials, Trial{Intensity: presented, Correct: correct})
			return
		}
		sc.consecCorrect = 0
		if sc.direction == +1 {
			sc.reversals = append(sc.reversals, presented)
			reversal = true
		}
		sc.direction = -1
		sc.current = math.Max(presented-sc.stepDown(), sc.cfg.MinIntensity)
	} else {
		sc.consecCorrect = 0
		if sc.direction == -1 {
			sc.reversals = append(sc.reversals, presented)
			reversal = true
		}
		sc.direction = +1
		sc.current = math.Min(presented+sc.stepUp(), sc.cfg.MaxIntensity)
	}

	sc.trials = append(sc.trials, Trial{Intensity: presented, Correct: correct, Reversal: reversal})
}

func (sc *UpDown) stepUp() float64 {
	if sc.cfg.Phase2StepUp > 0 && len(sc.reversals) >= sc.cfg.Phase2StartReversal {
		return sc.cfg.Phase2StepUp
	}
	return sc.cfg.StepUp
}

func (sc *UpDown) stepDown() float64 {
	if sc.cfg.Phase2StepDown > 0 && len(sc.reversals) >= sc.cfg.Phase2StartReversal {
		return sc.cfg.Phase2StepDown
	}
	return sc.cfg.StepDown
}

// Done reports whether the stopping criterion has been met.
func (sc *UpDown) Done() bool {
	if sc.cfg.MaxReversals > 0 && len(sc.reversals) >= sc.cfg.MaxReversals {
		return true
	}
	if sc.cfg.MaxTrials > 0 && len(sc.trials) >= sc.cfg.MaxTrials {
		return true
	}
	return false
}

// Threshold returns the mean of the last NReversalsForThreshold reversal
// intensities (or all reversals when NReversalsForThreshold is zero).
// Returns the current intensity if no reversals have been recorded yet.
func (sc *UpDown) Threshold() float64 {
	n := len(sc.reversals)
	if n == 0 {
		return sc.current
	}
	k := sc.cfg.NReversalsForThreshold
	if k <= 0 || k > n {
		k = n
	}
	sum := 0.0
	for _, v := range sc.reversals[n-k:] {
		sum += v
	}
	return sum / float64(k)
}

// History returns all trials recorded so far.
func (sc *UpDown) History() []Trial { return sc.trials }

// Reversals returns the intensity at each recorded direction reversal.
func (sc *UpDown) Reversals() []float64 { return sc.reversals }

// NReversals returns the number of direction reversals recorded so far.
func (sc *UpDown) NReversals() int { return len(sc.reversals) }
