// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build windows

package sysinfo

import (
	"sync"
	"time"
)

var (
	cimOnce  sync.Once
	cimCache map[string][]map[string]string
)

// cimGet returns every instance of a WMI class, from a single probe of all the
// classes in cimQueries that is run at most once per process.
//
// A class that is genuinely absent -- Win32_PageFileUsage on a machine with the
// page file disabled -- and a probe that could not run at all both yield nil.
// Every caller already treats a missing field as "unknown", which is the right
// answer in both cases: the data file loses a line rather than gaining a wrong
// one.
func cimGet(class string) []map[string]string {
	cimOnce.Do(func() {
		cimCache = parseCIMBatch(
			runPowerShell(buildCIMScript(cimQueries)))
	})
	return cimCache[class]
}

// runPowerShell executes a script and returns its stdout.
//
// Windows PowerShell 5.1 ships in the box on every supported version of
// Windows and is tried first; pwsh (PowerShell 7) is the fallback for a machine
// where the built-in one has been removed. -NoProfile matters for more than
// speed: a user profile that prints a banner would land in the middle of the
// output we are about to parse.
func runPowerShell(script string) string {
	enc := encodeUTF16LEBase64(script)
	for _, shell := range []string{"powershell", "pwsh"} {
		if out := run(shell, "-NoProfile", "-NonInteractive", "-EncodedCommand", enc); out != "" {
			return out
		}
	}
	return ""
}

// parseCIMTime parses the round-trip DateTime that cimQueries projects
// LastBootUpTime through.
func parseCIMTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
