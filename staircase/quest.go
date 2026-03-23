// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package staircase

import "math"

// QuestConfig configures a QUEST adaptive staircase (Watson & Pelli 1983).
//
// QUEST maintains a Bayesian posterior over a discrete grid of possible
// threshold values and presents each trial at the current posterior estimate,
// converging on the true threshold efficiently.
//
// All intensity values (TGuess, IntensityMin, IntensityMax) must be in the
// same units as the stimulus parameter — typically log-units for contrast
// (e.g. log10(contrast)) but any linear or log scale works as long as the
// psychometric function parameters (Beta, etc.) are consistent with it.
type QuestConfig struct {
	// Prior: initial belief about the threshold location.
	TGuess   float64 // prior mode (centre of the initial Gaussian)
	TGuessSd float64 // prior SD (width of the initial belief)

	// Weibull psychometric function parameters.
	// p(correct | x, T) = Delta·Gamma + (1−Delta)·(1−(1−Gamma)·exp(−10^(Beta·(x−T+xL))))
	// where xL is computed internally so that p(T|T) = PThreshold.
	PThreshold float64 // target proportion correct at threshold, e.g. 0.82 for 2AFC
	Beta       float64 // slope steepness; ~3.5 for contrast detection
	Delta      float64 // lapse rate, e.g. 0.01 (fraction of random responses)
	Gamma      float64 // lower asymptote: 0.5 for 2AFC, 0 for yes/no detection

	// Discrete intensity grid for the posterior approximation.
	IntensityMin  float64 // smallest possible threshold (lower bound of prior support)
	IntensityMax  float64 // largest possible threshold (upper bound)
	IntensityStep float64 // grid resolution; smaller = more accurate, slightly slower

	// MaxTrials is the stopping criterion: Quest ends after this many trials.
	MaxTrials int

	// EstimateMethod selects how the threshold is derived from the posterior:
	//   "mean" (default) — posterior mean; less sensitive to flat posteriors
	//   "mode"           — posterior mode (MAP estimate); slightly faster
	EstimateMethod string
}

// Quest implements the QUEST adaptive staircase (Watson & Pelli 1983).
type Quest struct {
	cfg     QuestConfig
	grid    []float64 // discrete intensity axis
	logPDF  []float64 // log-posterior (unnormalised; shifted for stability)
	xL      float64   // Weibull offset so that p(T|T) = PThreshold
	current float64   // intensity returned by the most recent Intensity() call
	trials  []Trial
}

// NewQuest returns a Quest staircase with the given configuration.
//
// Panics if IntensityStep ≤ 0, IntensityMin ≥ IntensityMax, or TGuessSd ≤ 0.
func NewQuest(cfg QuestConfig) *Quest {
	if cfg.IntensityStep <= 0 {
		panic("staircase: QuestConfig.IntensityStep must be > 0")
	}
	if cfg.IntensityMin >= cfg.IntensityMax {
		panic("staircase: QuestConfig.IntensityMin must be < IntensityMax")
	}
	if cfg.TGuessSd <= 0 {
		panic("staircase: QuestConfig.TGuessSd must be > 0")
	}
	if cfg.EstimateMethod == "" {
		cfg.EstimateMethod = "mean"
	}

	n := int(math.Round((cfg.IntensityMax-cfg.IntensityMin)/cfg.IntensityStep)) + 1
	grid := make([]float64, n)
	for i := range grid {
		grid[i] = cfg.IntensityMin + float64(i)*cfg.IntensityStep
	}

	// Gaussian log-prior centred on TGuess.
	logPDF := make([]float64, n)
	for i, x := range grid {
		d := (x - cfg.TGuess) / cfg.TGuessSd
		logPDF[i] = -0.5 * d * d
	}

	xL := questXL(cfg.PThreshold, cfg.Beta, cfg.Delta, cfg.Gamma)

	q := &Quest{
		cfg:    cfg,
		grid:   grid,
		logPDF: logPDF,
		xL:     xL,
	}
	q.current = q.estimate()
	return q
}

// questXL computes the Weibull offset xL such that p(T|T) = pThreshold.
//
//	xL = log10( −log( (1 − (pThreshold − delta·gamma)/(1−delta)) / (1−gamma) ) ) / beta
func questXL(pThreshold, beta, delta, gamma float64) float64 {
	w := (pThreshold - delta*gamma) / (1 - delta)
	return math.Log10(-math.Log((1-w)/(1-gamma))) / beta
}

// pCorrect returns the probability of a correct response at intensity x when
// the true threshold is T, using the Weibull psychometric function.
func (q *Quest) pCorrect(x, T float64) float64 {
	d := x - T + q.xL
	w := 1 - (1-q.cfg.Gamma)*math.Exp(-math.Pow(10, q.cfg.Beta*d))
	return q.cfg.Delta*q.cfg.Gamma + (1-q.cfg.Delta)*w
}

// estimate computes the current threshold estimate from the posterior.
func (q *Quest) estimate() float64 {
	// Numerically stable: shift log-PDF by its maximum before exponentiating.
	maxLog := q.logPDF[0]
	for _, v := range q.logPDF {
		if v > maxLog {
			maxLog = v
		}
	}

	if q.cfg.EstimateMethod == "mode" {
		best := 0
		for i, v := range q.logPDF {
			if v > q.logPDF[best] {
				best = i
			}
		}
		return q.grid[best]
	}

	// Posterior mean (default).
	wSum, wmean := 0.0, 0.0
	for i, v := range q.logPDF {
		w := math.Exp(v - maxLog)
		wSum += w
		wmean += w * q.grid[i]
	}
	if wSum == 0 {
		return q.cfg.TGuess
	}
	return wmean / wSum
}

// Intensity returns the stimulus intensity for the next trial (the current
// posterior estimate). Repeated calls without an intervening Update return
// the same value.
func (q *Quest) Intensity() float64 {
	q.current = q.estimate()
	return q.current
}

// Update records the participant's response at the intensity returned by the
// most recent Intensity() call and updates the posterior accordingly.
//
// Call Intensity() before each trial; Update() uses the cached value.
func (q *Quest) Update(correct bool) {
	x := q.current
	for i, T := range q.grid {
		p := q.pCorrect(x, T)
		if correct {
			q.logPDF[i] += math.Log(p)
		} else {
			q.logPDF[i] += math.Log(1 - p)
		}
	}
	q.trials = append(q.trials, Trial{Intensity: x, Correct: correct})
}

// Done reports whether the MaxTrials stopping criterion has been met.
func (q *Quest) Done() bool {
	return q.cfg.MaxTrials > 0 && len(q.trials) >= q.cfg.MaxTrials
}

// Threshold returns the current posterior estimate of the perceptual threshold.
// Equivalent to calling Intensity() but does not cache the result.
func (q *Quest) Threshold() float64 { return q.estimate() }

// History returns all trials recorded so far.
func (q *Quest) History() []Trial { return q.trials }

// SD returns the standard deviation of the current posterior, providing a
// measure of uncertainty in the threshold estimate.
func (q *Quest) SD() float64 {
	mean := q.estimate()

	maxLog := q.logPDF[0]
	for _, v := range q.logPDF {
		if v > maxLog {
			maxLog = v
		}
	}

	wSum, wVar := 0.0, 0.0
	for i, v := range q.logPDF {
		w := math.Exp(v - maxLog)
		wSum += w
		d := q.grid[i] - mean
		wVar += w * d * d
	}
	if wSum == 0 {
		return 0
	}
	return math.Sqrt(wVar / wSum)
}
