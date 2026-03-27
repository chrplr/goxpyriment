// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// gen-readme regenerates the example tables in examples/README.md from
// per-example meta.yaml files.
//
// Run from the repo root via: make update-readme
// (which cd's into examples/ and runs: go run ./cmd/gen-readme/)
//
// Each example directory that contains a main.go should also contain
// a meta.yaml file with three fields:
//
//	category:    experiment  # or "demo"
//	description: One-line task summary.
//	reference:   Author (year)  # empty string for demos
//
// The script rewrites the content between these sentinel comments in README.md:
//
//	<!-- BEGIN:experiments -->  …  <!-- END:experiments -->
//	<!-- BEGIN:demos -->        …  <!-- END:demos -->
//
// All other content in README.md is left untouched.
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// meta holds the per-example metadata read from meta.yaml.
type meta struct {
	dir         string // basename of the example directory
	category    string // "experiment" or "demo"
	description string
	reference   string // empty string for demos
}

// readMeta reads meta.yaml from dir and returns the parsed metadata.
// ok is false if the file does not exist or cannot be parsed.
func readMeta(dir string) (meta, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "meta.yaml"))
	if err != nil {
		return meta{}, false
	}
	m := meta{dir: filepath.Base(dir)}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "category":
			m.category = unquote(val)
		case "description":
			m.description = unquote(val)
		case "reference":
			m.reference = unquote(val)
		}
	}
	return m, m.category != "" && m.description != ""
}

// collectExamples walks the current directory (expected: examples/) for
// direct subdirectories that contain a main.go, and returns:
//   - metas: all entries with a valid meta.yaml, sorted case-insensitively
//   - undocumented: directory names without a meta.yaml, sorted
func collectExamples() ([]meta, []string) {
	entries, err := os.ReadDir(".")
	if err != nil {
		log.Fatalf("reading examples dir: %v", err)
	}

	var metas []meta
	var undocumented []string

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		// Skip the cmd/ utility directory (gen-readme lives there).
		if name == "cmd" {
			continue
		}
		// Only include directories that directly contain a main.go.
		if _, err := os.Stat(filepath.Join(name, "main.go")); err != nil {
			continue
		}
		m, ok := readMeta(name)
		if !ok {
			undocumented = append(undocumented, name)
			continue
		}
		metas = append(metas, m)
	}

	sort.Slice(metas, func(i, j int) bool {
		return strings.ToLower(metas[i].dir) < strings.ToLower(metas[j].dir)
	})
	sort.Strings(undocumented)
	return metas, undocumented
}

// experimentTable returns the Markdown table body for experiments.
func experimentTable(metas []meta) string {
	var sb strings.Builder
	sb.WriteString("| Directory | Task | Reference |\n")
	sb.WriteString("|-----------|------|-----------|\n")
	for _, m := range metas {
		if m.category != "experiment" {
			continue
		}
		fmt.Fprintf(&sb, "| [%s](%s/) | %s | %s |\n", m.dir, m.dir, m.description, m.reference)
	}
	return sb.String()
}

// demoTable returns the Markdown table body for demonstrations.
func demoTable(metas []meta) string {
	var sb strings.Builder
	sb.WriteString("| Directory | Description |\n")
	sb.WriteString("|-----------|-------------|\n")
	for _, m := range metas {
		if m.category != "demo" {
			continue
		}
		fmt.Fprintf(&sb, "| [%s](%s/) | %s |\n", m.dir, m.dir, m.description)
	}
	return sb.String()
}

// rewriteSentinel replaces the lines between begin and end sentinel comments
// (exclusive) with the given content, preserving all other lines.
func rewriteSentinel(lines []string, begin, end, content string) []string {
	var out []string
	inside := false
	injected := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == begin {
			out = append(out, line)
			// Inject replacement content immediately after opening sentinel.
			for _, cl := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
				out = append(out, cl)
			}
			inside = true
			injected = true
			continue
		}
		if trimmed == end {
			inside = false
			injected = false
		}
		if !inside {
			out = append(out, line)
		}
		_ = injected
	}
	return out
}

// unquote trims whitespace and strips surrounding double-quotes if present.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return s
}

func countCategory(metas []meta, cat string) int {
	n := 0
	for _, m := range metas {
		if m.category == cat {
			n++
		}
	}
	return n
}

func main() {
	const readmePath = "README.md"

	metas, undocumented := collectExamples()

	// Read README.md line by line.
	f, err := os.Open(readmePath)
	if err != nil {
		log.Fatalf("open %s: %v", readmePath, err)
	}
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	_ = f.Close()
	if err := sc.Err(); err != nil {
		log.Fatalf("scan %s: %v", readmePath, err)
	}

	// Rewrite sentinel sections.
	lines = rewriteSentinel(lines,
		"<!-- BEGIN:experiments -->", "<!-- END:experiments -->",
		experimentTable(metas))
	lines = rewriteSentinel(lines,
		"<!-- BEGIN:demos -->", "<!-- END:demos -->",
		demoTable(metas))

	// Write the result back.
	out, err := os.Create(readmePath)
	if err != nil {
		log.Fatalf("create %s: %v", readmePath, err)
	}
	w := bufio.NewWriter(out)
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
	if err := w.Flush(); err != nil {
		log.Fatalf("flush: %v", err)
	}
	if err := out.Close(); err != nil {
		log.Fatalf("close: %v", err)
	}

	fmt.Printf("Wrote %d experiments, %d demos.\n",
		countCategory(metas, "experiment"), countCategory(metas, "demo"))
	if len(undocumented) > 0 {
		fmt.Printf("WARNING: %d example(s) have no meta.yaml and were skipped:\n", len(undocumented))
		for _, name := range undocumented {
			fmt.Printf("  - %s\n", name)
		}
	}
}
