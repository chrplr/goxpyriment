// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// gen-gallery regenerates the example tables in docs/GalleryOfExamples.md from
// per-example meta.yaml files.
//
// Run from the repo root via: make update-examples-gallery
// (runs: go run ./cmd/gen-gallery/)
//
// Each example directory that contains a main.go should also contain
// a meta.yaml file with three fields:
//
//	category:    experiment  # or "demo"
//	description: One-line task summary.
//	reference:   Author (year)  # empty string for demos
//
// Technical tests in the sibling tests/ module are documented the same way;
// each test directory should carry a meta.yaml (category "test").
//
// The script rewrites the content between these sentinel comments in GalleryOfExamples.md:
//
//	<!-- BEGIN:experiments -->  …  <!-- END:experiments -->
//	<!-- BEGIN:demos -->        …  <!-- END:demos -->
//	<!-- BEGIN:tests -->        …  <!-- END:tests -->
//
// All other content in GalleryOfExamples.md is left untouched.
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

// Where the published builds live. Always the "latest" alias, never a
// commit-pinned URL: only the two most recent builds are retained, so a pinned
// link written into the repo would rot within two releases.
const (
	downloadsURL = "https://downloads.pallier.org/builds/latest/"
	runURLBase   = "https://downloads.pallier.org/builds/latest/wasm/"
)

// skipFile lists the examples that cannot run in a browser, with a reason each.
// It is the same file build-wasm-apps.sh reads, so the gallery and the
// published site cannot disagree about which examples have a browser version.
const skipFile = "examples/installers/wasm-skip.txt"

// readWasmSkip returns example name -> reason it has no browser build.
func readWasmSkip() map[string]string {
	skip := make(map[string]string)
	data, err := os.ReadFile(skipFile)
	if err != nil {
		log.Fatalf("reading %s: %v", skipFile, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		name, reason, _ := strings.Cut(line, "#")
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		skip[name] = strings.TrimSpace(reason)
	}
	return skip
}

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

// collect walks root/ for direct subdirectories that contain a main.go, and
// returns:
//   - metas: all entries with a valid meta.yaml, sorted case-insensitively
//   - undocumented: directory names without a meta.yaml, sorted
func collect(root string) ([]meta, []string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		log.Fatalf("reading %s dir: %v", root, err)
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
		// Skip the cmd/ utility directory.
		if name == "cmd" {
			continue
		}
		// Only include directories that directly contain a main.go.
		if _, err := os.Stat(filepath.Join(root, name, "main.go")); err != nil {
			continue
		}
		m, ok := readMeta(filepath.Join(root, name))
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

const repoExamplesURL = "https://github.com/chrplr/goxpyriment/tree/main/examples"
const repoTestsURL = "https://github.com/chrplr/goxpyriment/tree/main/tests"

// ncols is the number of examples laid out per row in the compact tables.
const ncols = 3

// compactCell builds one stacked cell: bold directory link, then the
// description, then an italic reference (each separated by a blank line so the
// three pieces read as a vertical unit). Empty pieces are omitted.
func compactCell(dir, urlBase, description, reference string, runnable bool) string {
	parts := []string{fmt.Sprintf("**[%s](%s/%s)**", dir, urlBase, dir)}
	if description != "" {
		parts = append(parts, description)
	}
	if reference != "" {
		parts = append(parts, fmt.Sprintf("*%s*", reference))
	}
	if runnable {
		parts = append(parts, fmt.Sprintf("[▶ Run in browser](%s%s/)", runURLBase, dir))
	}
	return strings.Join(parts, "<br><br>")
}

// compactTable lays cells out in an ncols-wide, center-aligned Markdown table.
// Trailing empty cells pad the final row.
func compactTable(cells []string) string {
	var sb strings.Builder
	sb.WriteString("| | | |\n")
	sb.WriteString("|:--:|:--:|:--:|\n")
	for i := 0; i < len(cells); i += ncols {
		row := make([]string, ncols)
		for j := range ncols {
			if i+j < len(cells) {
				row[j] = cells[i+j]
			}
		}
		fmt.Fprintf(&sb, "| %s |\n", strings.Join(row, " | "))
	}
	return sb.String()
}

// experimentTable returns the compact Markdown table body for experiments.
func experimentTable(metas []meta, skip map[string]string) string {
	var cells []string
	for _, m := range metas {
		if m.category != "experiment" {
			continue
		}
		_, blocked := skip[m.dir]
		cells = append(cells, compactCell(m.dir, repoExamplesURL, m.description, m.reference, !blocked))
	}
	return compactTable(cells)
}

// demoTable returns the compact Markdown table body for demonstrations.
func demoTable(metas []meta, skip map[string]string) string {
	var cells []string
	for _, m := range metas {
		if m.category != "demo" {
			continue
		}
		_, blocked := skip[m.dir]
		cells = append(cells, compactCell(m.dir, repoExamplesURL, m.description, "", !blocked))
	}
	return compactTable(cells)
}

// testsTable returns the compact Markdown table body for the technical tests.
func testsTable(metas []meta) string {
	var cells []string
	for _, m := range metas {
		cells = append(cells, compactCell(m.dir, repoTestsURL, m.description, "", false))
	}
	return compactTable(cells)
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

// Sentinels delimiting the generated block in each example's README.md.
const (
	linksBegin = "<!-- BEGIN:links -->"
	linksEnd   = "<!-- END:links -->"
)

// linksBlock renders the "Try it" section for one example.
func linksBlock(dir, blockedReason string) string {
	var sb strings.Builder
	sb.WriteString(linksBegin + "\n")
	sb.WriteString("## Try it without building\n\n")
	if blockedReason == "" {
		fmt.Fprintf(&sb, "- **[▶ Run it in your browser](%s%s/)** — no download, no install.\n", runURLBase, dir)
	} else {
		fmt.Fprintf(&sb, "- No browser version: %s.\n", blockedReason)
	}
	fmt.Fprintf(&sb, "- **[Download a prebuilt binary](%s)** — Windows, macOS, and Linux on x86-64 and arm64.\n\n", downloadsURL)
	sb.WriteString("<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>\n")
	sb.WriteString(linksEnd)
	return sb.String()
}

// writeExampleLinks adds (or refreshes) the generated block in every example's
// README.md, creating a minimal README for the examples that have none.
//
// The block is delimited by sentinels and appended at the end, so the
// hand-written body above it is never touched. Regenerating is idempotent.
func writeExampleLinks(metas []meta, skip map[string]string) (written, created int) {
	for _, m := range metas {
		path := filepath.Join("examples", m.dir, "README.md")
		block := linksBlock(m.dir, skip[m.dir])

		body, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			header := fmt.Sprintf("# %s\n\n%s\n", m.dir, m.description)
			if m.reference != "" {
				header += "\n*" + m.reference + "*\n"
			}
			body = []byte(header)
			created++
		} else if err != nil {
			log.Fatalf("reading %s: %v", path, err)
		}

		text := string(body)
		if i := strings.Index(text, linksBegin); i >= 0 {
			j := strings.Index(text, linksEnd)
			if j < 0 {
				log.Fatalf("%s: %s without a matching %s", path, linksBegin, linksEnd)
			}
			text = text[:i] + block + text[j+len(linksEnd):]
		} else {
			text = strings.TrimRight(text, "\n") + "\n\n---\n\n" + block + "\n"
		}

		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			log.Fatalf("writing %s: %v", path, err)
		}
		written++
	}
	return written, created
}

func main() {
	const galleryPath = "docs/GalleryOfExamples.md"

	skip := readWasmSkip()
	metas, undocumented := collect("examples")
	testMetas, testUndocumented := collect("tests")

	// Read GalleryOfExamples.md line by line.
	f, err := os.Open(galleryPath)
	if err != nil {
		log.Fatalf("open %s: %v", galleryPath, err)
	}
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	_ = f.Close()
	if err := sc.Err(); err != nil {
		log.Fatalf("scan %s: %v", galleryPath, err)
	}

	// Rewrite sentinel sections.
	lines = rewriteSentinel(lines,
		"<!-- BEGIN:experiments -->", "<!-- END:experiments -->",
		experimentTable(metas, skip))
	lines = rewriteSentinel(lines,
		"<!-- BEGIN:demos -->", "<!-- END:demos -->",
		demoTable(metas, skip))
	lines = rewriteSentinel(lines,
		"<!-- BEGIN:tests -->", "<!-- END:tests -->",
		testsTable(testMetas))

	// Write the result back.
	out, err := os.Create(galleryPath)
	if err != nil {
		log.Fatalf("create %s: %v", galleryPath, err)
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

	fmt.Printf("Wrote %d experiments, %d demos, %d tests.\n",
		countCategory(metas, "experiment"), countCategory(metas, "demo"), len(testMetas))

	n, created := writeExampleLinks(metas, skip)
	fmt.Printf("Refreshed the download/run block in %d example README(s); created %d.\n", n, created)
	fmt.Printf("%d example(s) have no browser build (see %s).\n", len(skip), skipFile)
	if len(undocumented) > 0 {
		fmt.Printf("WARNING: %d example(s) have no meta.yaml and were skipped:\n", len(undocumented))
		for _, name := range undocumented {
			fmt.Printf("  - %s\n", name)
		}
	}
	if len(testUndocumented) > 0 {
		fmt.Printf("WARNING: %d test(s) have no meta.yaml and were skipped:\n", len(testUndocumented))
		for _, name := range testUndocumented {
			fmt.Printf("  - %s\n", name)
		}
	}
}
