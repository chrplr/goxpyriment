// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# design package

Trial/block structure, randomization utilities, Latin-square counterbalancing, and constrained shuffling.

## Between-subjects (BWS) factors

`GetPermutedBWSFactorCondition(name string, subjectID int)` indexes into a Latin-square row derived from `subjectID`, ensuring balanced assignment across subjects. `subjectID` is an **int** (1-based: subject 1 → first condition). The default ID 0 and negative IDs are normalized into range via floored modulo, so the call never panics.

## Latin-square permutation types

| Constant | Algorithm | Notes |
|---|---|---|
| `PBalancedLatinSquare` | Bradley (1958) | Balanced for carryover effects; odd n → 2n×2n square |
| `PCycledLatinSquare` | Simple cycled rows | Fast, not carryover-balanced |
| `PRandom` | Zigzag column reorder + random labels | Randomized assignment |

## Constrained trial ordering

Prevents undesirable repetition patterns in shuffled trial lists.

### Constraint semantics

| Value | Meaning |
|---|---|
| `Constraint(p)` where p > 0 | At most p consecutive trials with the same value for this factor |
| `Constraint(-g)` where g > 0 | At least g index distance between any two trials sharing the same value |
| `Constraint(0)` | Unconstrained |

The algorithm is greedy-constructive with random restarts. It returns an error if no valid ordering is found within `maxAttempts`. Increase `maxAttempts` for tight constraints; typical values are 100–10000.

## Key conventions

- `Factors` values are `interface{}`; use type assertions when reading back in experiment code.
- `AddTrial(t, copies, randomPosition)` inserts `copies` independent copies via `t.Copy()`, so modifying `t` after the call does not affect added trials.
- `GetPermutedBWSFactorCondition` returns `nil` if `AddBWSFactor` was not called first with the same name (it does not panic).
- For constrained shuffles, prefer meaningful factor names (`"condition"`, `"target"`) over numeric indices — constraints are keyed by factor name, not column index.
