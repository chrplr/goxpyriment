// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package sysinfo

import (
	"encoding/base64"
	"encoding/csv"
	"strings"
	"unicode/utf16"
)

// This file holds the platform-independent half of the Windows system probe:
// building the PowerShell script and parsing what comes back. Only the exec
// lives in cim_windows.go, so the parsing can be tested on any machine --
// which matters here, because the developers of this framework do not run
// Windows and the previous implementation was never exercised by a test.
//
// # Why CIM rather than wmic
//
// The original probe shelled out to `wmic`. Microsoft deprecated wmic in 2021
// and it is absent by default on recent Windows 11 installations, where every
// field would have come back empty and a data file would have silently lost
// its entire host record. Get-CimInstance is the supported replacement and is
// present on every version of Windows this framework can run on.
//
// # Why one invocation
//
// PowerShell start-up dominates: six separate invocations would each pay it.
// The whole probe is issued as a single script whose output is split by class
// marker, so the cost is paid once. It is started early and off the critical
// path anyway (see PrimeHost), but "once" is still much better than "six times"
// on the machine where an experimenter is waiting.

// cimQuery is one WMI class and the PowerShell Select-Object projection that
// extracts the fields we want from it.
type cimQuery struct {
	class   string
	selects string
}

// cimQueries is every class the Windows collectors need.
//
// LastBootUpTime is projected through ToString('o') deliberately.
// Get-CimInstance returns it as a real DateTime, which ConvertTo-Csv would
// render in the machine's own culture -- so the same code would produce
// "8/18/2026 2:15:16 PM" on one machine and "18/08/2026 14:15:16" on another.
// Round-trip format is culture-invariant and parses as RFC 3339.
var cimQueries = []cimQuery{
	{"Win32_ComputerSystem", "Manufacturer,Model,PCSystemType"},
	{"Win32_OperatingSystem", "Caption,Version,TotalVisibleMemorySize,FreePhysicalMemory," +
		"@{n='LastBootUpTime';e={$_.LastBootUpTime.ToString('o')}}"},
	{"Win32_Processor", "Name,NumberOfCores,NumberOfLogicalProcessors,CurrentClockSpeed,MaxClockSpeed"},
	{"Win32_PageFileUsage", "AllocatedBaseSize,CurrentUsage"},
	{"Win32_VideoController", "Name,DriverVersion"},
	{"Win32_SoundDevice", "Name,Manufacturer,DriverVersion"},
}

// cimClassMarker prefixes each class's block in the script's output.
const cimClassMarker = "##CLASS "

// buildCIMScript renders the single PowerShell script that probes every class.
func buildCIMScript(queries []cimQuery) string {
	var b strings.Builder
	// A device name can contain a registered-trademark sign, and Windows
	// PowerShell writes redirected output in the console code page unless told
	// otherwise, which would mangle it.
	b.WriteString("[Console]::OutputEncoding=[System.Text.Encoding]::UTF8\n")
	// One unreadable class must not abort the rest.
	b.WriteString("$ErrorActionPreference='SilentlyContinue'\n")
	for _, q := range queries {
		b.WriteString("'" + cimClassMarker + q.class + "'\n")
		b.WriteString("Get-CimInstance -ClassName " + q.class +
			" | Select-Object " + q.selects + " | ConvertTo-Csv -NoTypeInformation\n")
	}
	return b.String()
}

// encodeUTF16LEBase64 encodes a script for PowerShell's -EncodedCommand.
//
// The script is passed this way rather than through -Command because it
// contains quotes, braces, dollars and pipes, and the rules for getting those
// through Windows' command-line parsing intact are notoriously unforgiving.
// Base64 has no such rules.
func encodeUTF16LEBase64(script string) string {
	units := utf16.Encode([]rune(script))
	b := make([]byte, 0, len(units)*2)
	for _, u := range units {
		b = append(b, byte(u), byte(u>>8))
	}
	return base64.StdEncoding.EncodeToString(b)
}

// parseCIMBatch splits the script's output into one slice of instances per
// class. Classes that produced no rows are absent from the map rather than
// present and empty, so a caller cannot mistake "no such device" for "the
// probe did not run".
func parseCIMBatch(out string) map[string][]map[string]string {
	result := map[string][]map[string]string{}
	var class string
	var block []string

	flush := func() {
		if class == "" {
			return
		}
		if rows := parseCIMCSV(strings.Join(block, "\n")); len(rows) > 0 {
			result[class] = rows
		}
		block = block[:0]
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if after, ok := strings.CutPrefix(line, cimClassMarker); ok {
			flush()
			class = strings.TrimSpace(after)
			continue
		}
		if class != "" {
			block = append(block, line)
		}
	}
	flush()
	return result
}

// parseCIMCSV turns one ConvertTo-Csv block into a map per row.
func parseCIMCSV(block string) []map[string]string {
	block = strings.TrimSpace(block)
	if block == "" {
		return nil
	}
	r := csv.NewReader(strings.NewReader(block))
	r.FieldsPerRecord = -1 // tolerate a ragged trailing line rather than lose the block
	records, err := r.ReadAll()
	if err != nil || len(records) < 2 {
		return nil
	}
	header := records[0]
	var rows []map[string]string
	for _, rec := range records[1:] {
		row := map[string]string{}
		empty := true
		for i, h := range header {
			if i >= len(rec) {
				break
			}
			v := strings.TrimSpace(rec[i])
			row[h] = v
			if v != "" {
				empty = false
			}
		}
		// ConvertTo-Csv emits a row of empty fields for an instance whose
		// properties are all null; it carries nothing and would otherwise be
		// returned by first().
		if !empty {
			rows = append(rows, row)
		}
	}
	return rows
}

// first returns the first instance of a probe result, or an empty map.
func first(rows []map[string]string) map[string]string {
	if len(rows) > 0 {
		return rows[0]
	}
	return map[string]string{}
}
