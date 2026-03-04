package design

import (
	"fmt"
	"goxpyriment/stimuli"
	"math/rand"
)

// Trial represents a single trial in an experiment.
type Trial struct {
	ID      int
	Factors map[string]interface{}
	Stimuli []stimuli.VisualStimulus
}

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

func (t *Trial) SetFactor(name string, value interface{}) {
	t.Factors[name] = value
}

func (t *Trial) GetFactor(name string) interface{} {
	return t.Factors[name]
}

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

func (t *Trial) Compare(other *Trial) bool {
	if len(t.Factors) != len(other.Factors) {
		return false
	}
	for k, v := range t.Factors {
		if other.Factors[k] != v {
			return false
		}
	}
	return true
}

// Block represents a block of trials.
type Block struct {
	ID             int
	Name           string
	Factors        map[string]interface{}
	Trials         []*Trial
	trialIDCounter int
}

func NewBlock(name string) *Block {
	return &Block{
		Name:    name,
		Factors: make(map[string]interface{}),
		Trials:  make([]*Trial, 0),
	}
}

func (b *Block) SetFactor(name string, value interface{}) {
	b.Factors[name] = value
}

func (b *Block) GetFactor(name string) interface{} {
	return b.Factors[name]
}

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

func (b *Block) ShuffleTrials() {
	ShuffleList(b.Trials)
}

func (b *Block) ClearTrials() {
	b.Trials = make([]*Trial, 0)
	b.trialIDCounter = 0
}

func (b *Block) RemoveTrial(index int) {
	if index < 0 || index >= len(b.Trials) {
		return
	}
	b.Trials = append(b.Trials[:index], b.Trials[index+1:]...)
}

func (b *Block) Summary() string {
	res := fmt.Sprintf("Block %d: %s\n", b.ID, b.Name)
	res += fmt.Sprintf("  block factors: %v\n", b.Factors)
	res += fmt.Sprintf("  n trials: %d\n", len(b.Trials))
	return res
}

// Experiment represents a basic experiment structure.
type Experiment struct {
	Name              string
	Blocks            []*Block
	blockIDCounter    int
	DataVariableNames []string
	ExperimentInfo    []string
	BWSFactors        map[string][]interface{}
	BWSFactorNames    []string
}

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

func (e *Experiment) AddBlock(b *Block, copies int) {
	for i := 0; i < copies; i++ {
		newB := b.Copy()
		newB.ID = e.blockIDCounter
		e.blockIDCounter++
		e.Blocks = append(e.Blocks, newB)
	}
}

func (e *Experiment) ShuffleBlocks() {
	ShuffleList(e.Blocks)
}

func (e *Experiment) ClearBlocks() {
	e.Blocks = make([]*Block, 0)
	e.blockIDCounter = 0
}

func (e *Experiment) AddDataVariableNames(names []string) {
	e.DataVariableNames = append(e.DataVariableNames, names...)
}

func (e *Experiment) AddExperimentInfo(text string) {
	e.ExperimentInfo = append(e.ExperimentInfo, text)
}

func (e *Experiment) AddBWSFactor(name string, conditions []interface{}) {
	e.BWSFactors[name] = conditions
	e.BWSFactorNames = append(e.BWSFactorNames, name)
}

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

func (e *Experiment) Summary() string {
	res := fmt.Sprintf("Experiment: %s\n", e.Name)
	res += fmt.Sprintf("  between subject factors: %v\n", e.BWSFactorNames)
	res += fmt.Sprintf("  n blocks: %d\n", len(e.Blocks))
	for _, b := range e.Blocks {
		res += b.Summary()
	}
	return res
}
