# Sternberg Memory Search

Implements the two experiments from Sternberg (1966), *High-speed scanning in human memory*, *Science*, 153, 652–654 ([article summary](https://www.sas.upenn.edu/~saul/hss.html)).

- **Experiment 1 (varied set):** On each trial, 1–6 digits are shown one at a time; after a delay, a test digit appears. Respond **F** if it was in the set, **J** if not. Set size varies from trial to trial. 24 practice + 144 test trials.

- **Experiment 2 (fixed set):** Same yes/no task, but the memorized set is fixed for a block (size 1, 2, or 4). Three blocks with 60 practice + 120 test trials each. 3.7 s between response and next trial.

Data columns: `experiment`, `block`, `set_size`, `trial`, `probe`, `positive`, `key`, `rt`, `correct`.

## Usage

```bash
# Run from repository root
go run ./examples/sternberg_memory_search [flags]

# Flags
-d       developer mode (1024×1024 window)
-s N     subject ID (default 0)
-exp N   experiment: 1 (varied set only), 2 (fixed set only), 0 (both, default)
```

Example: `go run ./examples/sternberg_memory_search -d -s 1 -exp 1`
