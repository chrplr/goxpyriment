// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

# design package

Trial/block structure, randomization utilities, Latin-square counterbalancing, and constrained shuffling.

## Hierarchy

```
design.Experiment
  └── []Block
        └── []Trial
              └── Factors map[string]interface{}
                  Stimuli []VisualStimulus
```

## Trial

```go
t := design.NewTrial()
t.SetFactor("condition", "congruent")
t.SetFactor("SOA", 50)
v := t.GetFactor("condition")  // returns interface{}
t.AddStimulus(someVisualStim)

copy := t.Copy()          // deep copy factors + stimulus slice
equal := t.Compare(other) // equality on factors only
```

`Factors` maps `string → interface{}`. Cast with a type assertion in experiment code.

## Block

```go
b := design.NewBlock("practice")
b.SetFactor("block_type", "practice")

// Add 10 copies of t at random positions among existing trials
b.AddTrial(t, 10, true)    // true = random insertion
b.AddTrial(t2, 1, false)   // false = append

b.ShuffleTrials()
b.RemoveTrial(index)
b.ClearTrials()

copy := b.Copy()  // deep copy all trials
```

## Experiment (design)

```go
exp := design.NewExperiment("Stroop")
exp.AddDataVariableNames([]string{"rt", "accuracy", "response"})
exp.AddExperimentInfo("Stroop color-word interference task")

exp.AddBlock(b, 2)    // add 2 copies of b
exp.ShuffleBlocks()
exp.ClearBlocks()

fmt.Println(exp.Summary())  // human-readable design description
```

### Between-subjects (BWS) factors

Latin-square counterbalancing for between-subjects factors:

```go
exp.AddBWSFactor("hand", []string{"left", "right"})

// In experiment loop, given subject ID:
hand := exp.GetPermutedBWSFactorCondition("hand", subjectID)
```

`GetPermutedBWSFactorCondition` indexes into a Latin-square row derived from `subjectID`, ensuring balanced assignment across subjects. The `subjectID` string is parsed as an integer; non-numeric IDs are handled gracefully.

## Randomization utilities

| Function | Signature | Description |
|---|---|---|
| `RandIntSequence` | `(first, last int) []int` | Shuffled slice of [first, last] |
| `RandInt` | `(a, b int) int` | Uniform random integer in [a, b] |
| `RandElement[T]` | `(list []T) T` | Random element from slice |
| `CoinFlip` | `(headBias float64) bool` | True with probability headBias |
| `RandNorm` | `(a, b float64) float64` | Truncated normal in [a, b] |
| `ShuffleList[T]` | `(list []T)` | In-place Fisher-Yates shuffle |
| `MakeMultipliedShuffledList[T]` | `(list []T, n int) []T` | n independent shuffles, concatenated |

`RandElement`, `ShuffleList`, `MakeMultipliedShuffledList` are generic (Go 1.18+).

## Latin-square permutations

```go
// Integer square
square := design.LatinSquareInts(n, design.PBalancedLatinSquare)

// Generic square with element values
conditions := []string{"A", "B", "C", "D"}
square := design.LatinSquare(conditions, design.PCycledLatinSquare)
```

### Permutation types

| Constant | Algorithm | Notes |
|---|---|---|
| `PBalancedLatinSquare` | Bradley (1958) | Balanced for carryover effects; odd n → 2n×2n square |
| `PCycledLatinSquare` | Simple cycled rows | Fast, not carryover-balanced |
| `PRandom` | Zigzag column reorder + random labels | Randomized assignment |

`IsPermutationType(typeStr) bool` — validates a string against the three constants.

## Constrained trial ordering

Prevents undesirable repetition patterns in shuffled trial lists.

```go
constraints := map[string]design.Constraint{
    "condition": design.Constraint(1),   // at most 1 consecutive same condition
    "target":    design.Constraint(-3),  // target items at least 3 trials apart
}
err := b.ShuffleTrialsConstrained(constraints, 1000)  // maxAttempts = 1000
```

### Constraint semantics

| Value | Meaning |
|---|---|
| `Constraint(p)` where p > 0 | At most p consecutive trials with the same value for this factor |
| `Constraint(-g)` where g > 0 | At least g index distance between any two trials sharing the same value |
| `Constraint(0)` | Unconstrained |

`ShuffleTableConstrained(table [][]string, constraints []Constraint, maxAttempts int)` is the lower-level function operating on a 2D string table.

The algorithm is greedy-constructive with random restarts. It returns an error if no valid ordering is found within `maxAttempts`. Increase `maxAttempts` for tight constraints; typical values are 100–10000.

## Key conventions

- `Factors` values are `interface{}`; use type assertions when reading back in experiment code.
- `AddTrial(t, copies, randomPosition)` inserts `copies` independent copies via `t.Copy()`, so modifying `t` after the call does not affect added trials.
- `GetPermutedBWSFactorCondition` requires `AddBWSFactor` to be called first with the same name; panics otherwise.
- For constrained shuffles, prefer meaningful factor names (`"condition"`, `"target"`) over numeric indices — constraints are keyed by factor name, not column index.
