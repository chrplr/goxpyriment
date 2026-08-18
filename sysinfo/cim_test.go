// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package sysinfo

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
)

// A realistic capture of what the script emits: CRLF line endings, a quoted
// field containing a comma, a class that returned nothing, and a class that is
// not installed at all (no marker, because -ErrorAction silenced it -- the
// marker is still printed, with no CSV under it).
const sampleCIMOutput = "##CLASS Win32_ComputerSystem\r\n" +
	"\"Manufacturer\",\"Model\",\"PCSystemType\"\r\n" +
	"\"Dell Inc.\",\"Precision 5490\",\"2\"\r\n" +
	"##CLASS Win32_OperatingSystem\r\n" +
	"\"Caption\",\"Version\",\"TotalVisibleMemorySize\",\"FreePhysicalMemory\",\"LastBootUpTime\"\r\n" +
	"\"Microsoft Windows 11 Pro\",\"10.0.26100\",\"33234152\",\"18120044\",\"2026-08-17T09:12:44.1234567+02:00\"\r\n" +
	"##CLASS Win32_Processor\r\n" +
	"\"Name\",\"NumberOfCores\",\"NumberOfLogicalProcessors\",\"CurrentClockSpeed\",\"MaxClockSpeed\"\r\n" +
	"\"Intel(R) Core(TM) Ultra 7 165H\",\"16\",\"22\",\"2188\",\"4700\"\r\n" +
	"##CLASS Win32_PageFileUsage\r\n" +
	"##CLASS Win32_VideoController\r\n" +
	"\"Name\",\"DriverVersion\"\r\n" +
	"\"Intel(R) Arc(TM) Graphics\",\"32.0.101.6314\"\r\n" +
	"\"NVIDIA RTX 2000 Ada, Laptop GPU\",\"32.0.15.7602\"\r\n" +
	"##CLASS Win32_SoundDevice\r\n" +
	"\"Name\",\"Manufacturer\",\"DriverVersion\"\r\n" +
	"\"Realtek(R) Audio\",\"Realtek\",\"6.0.9622.1\"\r\n"

func TestParseCIMBatch(t *testing.T) {
	got := parseCIMBatch(sampleCIMOutput)

	if n := len(got["Win32_VideoController"]); n != 2 {
		t.Fatalf("expected 2 video controllers, got %d", n)
	}
	// The comma inside a quoted device name must not split the field. wmic's
	// old key=value format could not express this at all.
	if name := got["Win32_VideoController"][1]["Name"]; name != "NVIDIA RTX 2000 Ada, Laptop GPU" {
		t.Errorf("quoted comma mangled: %q", name)
	}
	if m := first(got["Win32_ComputerSystem"]); m["Model"] != "Precision 5490" || m["PCSystemType"] != "2" {
		t.Errorf("ComputerSystem = %v", m)
	}
	if v := first(got["Win32_OperatingSystem"])["Version"]; v != "10.0.26100" {
		t.Errorf("OS version = %q", v)
	}
	// A class that produced no rows must be absent, not present and empty:
	// callers use first(), and an empty map is indistinguishable from a real
	// instance whose fields all failed to read.
	if _, present := got["Win32_PageFileUsage"]; present {
		t.Errorf("a class with no rows should be absent from the map")
	}
	if m := first(got["Win32_PageFileUsage"]); len(m) != 0 {
		t.Errorf("first() of an absent class should be empty, got %v", m)
	}
}

func TestParseCIMBatchSurvivesGarbage(t *testing.T) {
	// Whatever else happens, this must not panic or hang: the probe runs at the
	// start of every session and its output is not under our control.
	for name, in := range map[string]string{
		"empty":         "",
		"no markers":    "just some text\r\nmore text\r\n",
		"marker only":   cimClassMarker + "Win32_Foo\r\n",
		"header only":   cimClassMarker + "Win32_Foo\r\n\"A\",\"B\"\r\n",
		"unterminated":  cimClassMarker + "Win32_Foo\r\n\"A\",\"B\"\r\n\"unclosed",
		"empty row":     cimClassMarker + "Win32_Foo\r\n\"A\",\"B\"\r\n\"\",\"\"\r\n",
		"ragged row":    cimClassMarker + "Win32_Foo\r\n\"A\",\"B\",\"C\"\r\n\"1\",\"2\"\r\n",
		"banner before": "Welcome to PowerShell\r\n" + cimClassMarker + "Win32_Foo\r\n\"A\"\r\n\"1\"\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := parseCIMBatch(in)
			if name == "empty row" {
				if _, present := got["Win32_Foo"]; present {
					t.Errorf("an all-empty CSV row should not become an instance: %v", got)
				}
			}
			if name == "banner before" && first(got["Win32_Foo"])["A"] != "1" {
				t.Errorf("output before the first marker should be ignored, got %v", got)
			}
		})
	}
}

func TestBuildCIMScriptCoversEveryCollector(t *testing.T) {
	script := buildCIMScript(cimQueries)
	// Every class a Windows collector asks cimGet for must be in the batch;
	// one missing would silently blank that collector's fields.
	for _, class := range []string{
		"Win32_ComputerSystem", "Win32_OperatingSystem", "Win32_Processor",
		"Win32_PageFileUsage", "Win32_VideoController", "Win32_SoundDevice",
	} {
		if !strings.Contains(script, cimClassMarker+class) {
			t.Errorf("script does not emit a marker for %s", class)
		}
		if !strings.Contains(script, "Get-CimInstance -ClassName "+class) {
			t.Errorf("script does not query %s", class)
		}
	}
	if !strings.Contains(script, "ToString('o')") {
		t.Error("LastBootUpTime must be projected culture-invariantly")
	}
	if strings.Contains(script, "wmic") {
		t.Error("script still refers to the deprecated wmic")
	}
}

func TestEncodeUTF16LEBase64(t *testing.T) {
	// -EncodedCommand expects UTF-16LE. Decoding must give the script back.
	const script = "Get-CimInstance -ClassName Win32_Processor | Select-Object Name"
	raw, err := base64.StdEncoding.DecodeString(encodeUTF16LEBase64(script))
	if err != nil {
		t.Fatalf("not valid base64: %v", err)
	}
	if len(raw)%2 != 0 {
		t.Fatalf("UTF-16 payload has an odd byte count: %d", len(raw))
	}
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i < len(raw); i += 2 {
		units = append(units, uint16(raw[i])|uint16(raw[i+1])<<8)
	}
	if got := string(utf16.Decode(units)); got != script {
		t.Errorf("round trip gave %q", got)
	}
}

// TestWindowsCollectorFieldsAreQueried guards the one failure this package
// cannot catch by running: a Windows collector reading a property that the
// batch script never asked for. That yields "" -- an empty line in a data
// file, on a machine the authors of this framework do not have -- with nothing
// anywhere to indicate a mistake. The check is a source scan because the
// collectors are behind a windows build tag and cannot be linked here.
func TestWindowsCollectorFieldsAreQueried(t *testing.T) {
	queried := map[string]bool{}
	for _, q := range cimQueries {
		// Field names appear either bare in the Select-Object list or as the
		// name of a calculated property, @{n='Foo';e={...}}.
		for _, m := range regexp.MustCompile(`n='([A-Za-z]+)'`).FindAllStringSubmatch(q.selects, -1) {
			queried[m[1]] = true
		}
		for _, part := range strings.Split(regexp.MustCompile(`@\{[^}]*\}\}?`).ReplaceAllString(q.selects, ""), ",") {
			if p := strings.TrimSpace(part); p != "" {
				queried[p] = true
			}
		}
	}

	files, err := filepath.Glob("*_windows.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no *_windows.go files found: %v", err)
	}
	index := regexp.MustCompile(`\["([A-Za-z]+)"\]`)
	checked := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range index.FindAllStringSubmatch(string(src), -1) {
			field := m[1]
			checked++
			if !queried[field] {
				t.Errorf("%s reads field %q, which no cimQueries entry selects", f, field)
			}
		}
	}
	if checked == 0 {
		t.Error("scanned no field accesses at all; the regexp has stopped matching")
	}
	t.Logf("checked %d field accesses across %d files", checked, len(files))
}
