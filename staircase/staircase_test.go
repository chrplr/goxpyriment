// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package staircase

import (
	"math"
	"math/rand"
	"testing"
)

func validQuestConfig() QuestConfig {
	return QuestConfig{
		TGuess:        -1,
		TGuessSd:      2,
		PThreshold:    0.82,
		Beta:          3.5,
		Delta:         0.01,
		Gamma:         0.5,
		IntensityMin:  -4,
		IntensityMax:  0,
		IntensityStep: 0.05,
		MaxTrials:     20,
	}
}

func TestNewQuestValidConfig(t *testing.T) {
	q, err := NewQuest(validQuestConfig())
	if err != nil {
		t.Fatalf("valid config returned error: %v", err)
	}
	if q == nil {
		t.Fatal("valid config returned nil Quest")
	}
}

func TestNewQuestInvalidConfig(t *testing.T) {
	cases := map[string]func(*QuestConfig){
		"non-positive IntensityStep": func(c *QuestConfig) { c.IntensityStep = 0 },
		"negative IntensityStep":     func(c *QuestConfig) { c.IntensityStep = -0.1 },
		"min equals max":             func(c *QuestConfig) { c.IntensityMin = c.IntensityMax },
		"min greater than max":       func(c *QuestConfig) { c.IntensityMin = c.IntensityMax + 1 },
		"non-positive TGuessSd":      func(c *QuestConfig) { c.TGuessSd = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := validQuestConfig()
			mutate(&cfg)
			q, err := NewQuest(cfg)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", name)
			}
			if q != nil {
				t.Fatalf("expected nil Quest on error, got %v", q)
			}
		})
	}
}

// TestQuestRunsToCompletion exercises the trial loop mechanically: intensities
// stay within the grid, and Done becomes true after exactly MaxTrials updates.
func TestQuestRunsToCompletion(t *testing.T) {
	cfg := validQuestConfig()
	q, err := NewQuest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for !q.Done() {
		x := q.Intensity()
		if x < cfg.IntensityMin || x > cfg.IntensityMax {
			t.Fatalf("intensity %v outside grid [%v, %v]", x, cfg.IntensityMin, cfg.IntensityMax)
		}
		q.Update(true)
		n++
		if n > cfg.MaxTrials+1 {
			t.Fatal("Done never became true")
		}
	}
	if got := len(q.History()); got != cfg.MaxTrials {
		t.Fatalf("expected %d trials, got %d", cfg.MaxTrials, got)
	}
	if th := q.Threshold(); math.IsNaN(th) {
		t.Fatal("Threshold returned NaN")
	}
}

// fakeStaircase is a minimal Staircase with a caller-controlled Done flag, used
// to drive Runner into its all-done state deterministically.
type fakeStaircase struct{ done bool }

func (f *fakeStaircase) Intensity() float64 { return 0 }
func (f *fakeStaircase) Update(bool)        {}
func (f *fakeStaircase) Done() bool         { return f.done }
func (f *fakeStaircase) Threshold() float64 { return 0 }
func (f *fakeStaircase) History() []Trial   { return nil }

func TestRunnerNextSkipsDone(t *testing.T) {
	active := &fakeStaircase{done: false}
	finished := &fakeStaircase{done: true}
	r := NewRunner(rand.New(rand.NewSource(1)), active, finished)

	if r.Done() {
		t.Fatal("Runner.Done true while one staircase is active")
	}
	sc, err := r.Next()
	if err != nil {
		t.Fatalf("Next returned error while a staircase is active: %v", err)
	}
	if sc != active {
		t.Fatal("Next returned a done staircase")
	}
}

func TestRunnerNextErrorWhenAllDone(t *testing.T) {
	a := &fakeStaircase{done: true}
	b := &fakeStaircase{done: true}
	r := NewRunner(rand.New(rand.NewSource(1)), a, b)

	if !r.Done() {
		t.Fatal("Runner.Done false while all staircases are done")
	}
	sc, err := r.Next()
	if err == nil {
		t.Fatal("expected error from Next when all staircases are done")
	}
	if sc != nil {
		t.Fatal("expected nil staircase on error")
	}
}

// TestUpDownReversesAndStops drives a 1-up/2-down staircase with a deterministic
// correct/correct/incorrect pattern and checks it reaches its reversal-based
// stopping criterion with a finite threshold.
func TestUpDownReversesAndStops(t *testing.T) {
	cfg := UpDownConfig{
		StartIntensity:         0.5,
		MinIntensity:           0,
		MaxIntensity:           1,
		StepUp:                 0.1,
		StepDown:               0.05,
		NCorrectDown:           2,
		MaxReversals:           4,
		NReversalsForThreshold: 4,
	}
	sc := NewUpDown(cfg)

	pattern := []bool{true, true, false}
	for i := 0; !sc.Done(); i++ {
		sc.Intensity()
		sc.Update(pattern[i%len(pattern)])
		if i > 1000 {
			t.Fatal("staircase never reached MaxReversals")
		}
	}
	if sc.NReversals() < cfg.MaxReversals {
		t.Fatalf("expected at least %d reversals, got %d", cfg.MaxReversals, sc.NReversals())
	}
	if th := sc.Threshold(); th < cfg.MinIntensity || th > cfg.MaxIntensity {
		t.Fatalf("threshold %v outside [%v, %v]", th, cfg.MinIntensity, cfg.MaxIntensity)
	}
}
