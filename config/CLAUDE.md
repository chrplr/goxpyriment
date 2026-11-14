# config package

Default values for command-line flags from a TOML file: what `-config` does in
`control.NewExperimentFromFlags`. No SDL dependency, so a launcher (e.g. a menu
that starts experiments) can read the same file with `Load` without linking
`control`, whose `init` registers flags on `flag.CommandLine`.

## Rules

- Keys are flag names without the dash. Values: string, integer, float or
  boolean, converted with `strconv`. Arrays, dates and nested tables are
  errors.
- The table named `table` (control passes the executable name) overrides the
  top-level keys; other tables are not looked into. The two are merged before
  anything is set, and applied in name order, so Go's map order cannot decide
  the result.
- A top-level key the flag set does not define is skipped (it is meant for
  another program). A key in the program's own table that it does not define
  is an error: that is where a typo can be caught.
- `config`, `s`, `version`, `h`, `help` are refused. `s` set from a file
  would count as given (`flag.Visit`) and skip the setup dialog; `version` and
  `h` act as soon as they are set.
- Precedence: command line > program table > top-level key > built-in default.
  `ParseFlags` runs `ff.Parse` (peterbourgon/ff v3), which parses the arguments
  first; the parser then skips every flag `fs.Visit` reports as given. ff's own
  `fftoml` is not used: it is on go-toml v1 and knows nothing of per-program
  tables.
- A missing file is an error. A caller that only *may* have a file passes ""
  as the default of `-config`.
- `ParseFlags` returns exactly the settings it applied, for the caller to
  record (control writes them into the -info.txt).
- `CheckArgs` (called by control after `ParseFlags`) rejects the two
  command-line mistakes Go's `flag` accepts silently: a boolean flag given its
  value after a space (`-photodiode false` sets it to true and stops parsing),
  and a flag after a non-flag argument (`list.csv -s 3`). Other positional
  arguments stay allowed: some examples take a file name. It walks `args` the
  way `flag.Parse` does, so `-s -photodiode false` is not misread.
