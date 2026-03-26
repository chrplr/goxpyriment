// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

// Package design provides experiment structure types (trials, blocks, factors)
// and randomization helpers for building experiment designs.
package design

import (
	"fmt"
	"math/rand"
	"reflect"

	"github.com/chrplr/goxpyriment/stimuli"
)

// Trial represents a single trial in an experiment.
// Factors hold arbitrary key-value metadata; Stimuli hold the visual/audio
// objects to present for this trial.
type Trial struct {
	ID      int
	Factors map[string]interface{}
	Stimuli []stimuli.VisualStimulus
}

// NewTrial allocates a new trial with empty factors and stimuli.
func NewTrial() *Trial {
	return &Trial{
		Factors: make(map[string]interface{}),
		Stimuli: make([]stimuli.VisualStimulus, 0),
	}
}

// AddStimulus adds a stimulus to the trial.
func (t *Trial) AddStimulus(s stimuli.VisualStimulus) {
	t.Stimuli = append(t.Stimuli, s)
}

// SetFactor sets a named factor (e.g. "condition", "target") for the trial.
func (t *Trial) SetFactor(name string, value interface{}) {
	t.Factors[name] = value
}

// GetFactor returns the value of the named factor, or nil if not set.
func (t *Trial) GetFactor(name string) interface{} {
	return t.Factors[name]
}

// Copy returns a deep copy of the trial (factors and stimulus slice copied).
func (t *Trial) Copy() *Trial {
	newT := &Trial{
		ID:      t.ID,
		Factors: make(map[string]interface{}),
		Stimuli: make([]stimuli.VisualStimulus, len(t.Stimuli)),
	}
	for k, v := range t.Factors {
		newT.Factors[k] = v
	}
	copy(newT.Stimuli, t.Stimuli)
	return newT
}

// Compare returns true if the two trials have the same factor keys and values.
func (t *Trial) Compare(other *Trial) bool {
	if len(t.Factors) != len(other.Factors) {
		return false
	}
	for k, v := range t.Factors {
		if !reflect.DeepEqual(other.Factors[k], v) {
			return false
		}
	}
	return true
}

// Block represents a block of trials, optionally with block-level factors.
type Block struct {
	ID             int
	Name           string
	Factors        map[string]interface{}
	Trials         []*Trial
	trialIDCounter int
}

// NewBlock allocates a new block with the given name and no trials.
func NewBlock(name string) *Block {
	return &Block{
		Name:    name,
		Factors: make(map[string]interface{}),
		Trials:  make([]*Trial, 0),
	}
}

// SetFactor sets a block-level factor (e.g. "block_type").
func (b *Block) SetFactor(name string, value interface{}) {
	b.Factors[name] = value
}

// GetFactor returns the value of the named block factor, or nil if not set.
func (b *Block) GetFactor(name string) interface{} {
	return b.Factors[name]
}

// Copy returns a deep copy of the block and all its trials.
func (b *Block) Copy() *Block {
	newB := &Block{
		ID:             b.ID,
		Name:           b.Name,
		Factors:        make(map[string]interface{}),
		Trials:         make([]*Trial, len(b.Trials)),
		trialIDCounter: b.trialIDCounter,
	}
	for k, v := range b.Factors {
		newB.Factors[k] = v
	}
	for i, t := range b.Trials {
		newB.Trials[i] = t.Copy()
	}
	return newB
}

// AddTrial appends copies of the trial to the block. If randomPosition is true,
// each new trial is inserted at a random position among existing trials.
func (b *Block) AddTrial(t *Trial, copies int, randomPosition bool) {
	for i := 0; i < copies; i++ {
		newT := t.Copy()
		newT.ID = b.trialIDCounter
		b.trialIDCounter++

		if randomPosition && len(b.Trials) > 0 {
			pos := rand.Intn(len(b.Trials) + 1)
			b.Trials = append(b.Trials, nil)
			copy(b.Trials[pos+1:], b.Trials[pos:])
			b.Trials[pos] = newT
		} else {
			b.Trials = append(b.Trials, newT)
		}
	}
}

// ShuffleTrials randomizes the order of trials in the block in place.
func (b *Block) ShuffleTrials() {
	ShuffleList(b.Trials)
}

// ClearTrials removes all trials from the block and resets the trial ID counter.
func (b *Block) ClearTrials() {
	b.Trials = make([]*Trial, 0)
	b.trialIDCounter = 0
}

// RemoveTrial removes the trial at the given index; no-op if index is out of range.
func (b *Block) RemoveTrial(index int) {
	if index < 0 || index >= len(b.Trials) {
		return
	}
	b.Trials = append(b.Trials[:index], b.Trials[index+1:]...)
}

// Summary returns a short human-readable description of the block.
func (b *Block) Summary() string {
	res := fmt.Sprintf("Block %d: %s\n", b.ID, b.Name)
	res += fmt.Sprintf("  block factors: %v\n", b.Factors)
	res += fmt.Sprintf("  n trials: %d\n", len(b.Trials))
	return res
}

// Experiment represents a design-time experiment structure: name, blocks,
// data variable names, and optional between-subject factors for counterbalancing.
type Experiment struct {
	Name              string
	Blocks            []*Block
	blockIDCounter    int
	DataVariableNames []string
	ExperimentInfo    []string
	BWSFactors        map[string][]interface{}
	BWSFactorNames    []string
}

// NewExperiment allocates a new design experiment with the given name and no blocks.
func NewExperiment(name string) *Experiment {
	return &Experiment{
		Name:              name,
		Blocks:            make([]*Block, 0),
		DataVariableNames: make([]string, 0),
		ExperimentInfo:    make([]string, 0),
		BWSFactors:        make(map[string][]interface{}),
		BWSFactorNames:    make([]string, 0),
	}
}

// AddBlock appends the given number of copies of the block to the experiment.
func (e *Experiment) AddBlock(b *Block, copies int) {
	for i := 0; i < copies; i++ {
		newB := b.Copy()
		newB.ID = e.blockIDCounter
		e.blockIDCounter++
		e.Blocks = append(e.Blocks, newB)
	}
}

// ShuffleBlocks randomizes the order of blocks in place.
func (e *Experiment) ShuffleBlocks() {
	ShuffleList(e.Blocks)
}

// ClearBlocks removes all blocks and resets the block ID counter.
func (e *Experiment) ClearBlocks() {
	e.Blocks = make([]*Block, 0)
	e.blockIDCounter = 0
}

// AddDataVariableNames appends column names for the data file (e.g. trial, rt, key).
func (e *Experiment) AddDataVariableNames(names []string) {
	e.DataVariableNames = append(e.DataVariableNames, names...)
}

// AddExperimentInfo appends a line of experiment metadata (e.g. for headers).
func (e *Experiment) AddExperimentInfo(text string) {
	e.ExperimentInfo = append(e.ExperimentInfo, text)
}

// AddBWSFactor registers a between-subject factor with its conditions for counterbalancing.
func (e *Experiment) AddBWSFactor(name string, conditions []interface{}) {
	e.BWSFactors[name] = conditions
	e.BWSFactorNames = append(e.BWSFactorNames, name)
}

// GetPermutedBWSFactorCondition returns the condition for the given factor
// for the given subject ID, according to the experiment's permutation scheme.
func (e *Experiment) GetPermutedBWSFactorCondition(name string, subjectID int) interface{} {
	conditions, ok := e.BWSFactors[name]
	if !ok || len(conditions) == 0 {
		return nil
	}

	// Simple permutation logic
	nTotal := 1
	for _, f := range e.BWSFactorNames {
		nTotal *= len(e.BWSFactors[f])
	}

	nLower := nTotal
	for _, f := range e.BWSFactorNames {
		nLower /= len(e.BWSFactors[f])
		if f == name {
			idx := ((subjectID - 1) / nLower) % len(conditions)
			return conditions[idx]
		}
	}
	return nil
}

// Summary returns a short human-readable description of the experiment design.
func (e *Experiment) Summary() string {
	res := fmt.Sprintf("Experiment: %s\n", e.Name)
	res += fmt.Sprintf("  between subject factors: %v\n", e.BWSFactorNames)
	res += fmt.Sprintf("  n blocks: %d\n", len(e.Blocks))
	for _, b := range e.Blocks {
		res += b.Summary()
	}
	return res
}
