# ConceptARC task files

The 176 `.json` files in this directory are the ConceptARC corpus of
Moskvichev, Odouard & Mitchell (2023), copied unchanged from
<https://github.com/victorvikram/ConceptARC> (commit `0e67da6`, 2026-02-10):

- `corpus/<Concept>/<Concept>N.json` → `<Concept>N.json` — 16 concept groups
  × 10 tasks (160 files);
- `MinimalTasks/<Concept>Minimal.json` → `<Concept>Minimal.json` — the 16
  "minimal" tasks used as attention checks in the human study.

Each file follows the format of Chollet's ARC: `train` holds the demonstration
pairs, `test` the three test inputs with their expected outputs; a grid is a
list of rows, each row a list of colour indices 0–9.

The corpus is distributed under the MIT licence (see `LICENSE`). If you use it,
cite the paper: A. Moskvichev, V. V. Odouard & M. Mitchell, "The ConceptARC
Benchmark: Evaluating Understanding and Generalization in the ARC Domain",
arXiv:2305.07141 (2023).
