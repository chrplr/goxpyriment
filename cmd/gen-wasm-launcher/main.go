// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// gen-wasm-launcher writes the HTML page that runs one experiment in a browser,
// for publication under downloads.pallier.org/builds/{sha}/wasm/{app}/.
//
// It has two modes.
//
// Generated (the default): render the standard launcher — title, description
// and reference taken from the example's meta.yaml, a participant-ID field, a
// Start button, and the loading/diagnostic plumbing that a goxpyriment browser
// page needs. The template is adapted from examples/Memory_span/web/index.html,
// which is a working, debugged page; the comments there explaining *why* each
// piece is shaped the way it is are carried over deliberately.
//
//	go run ./cmd/gen-wasm-launcher -app Stroop_task -out .../wasm/Stroop_task/index.html
//
// Adapted: with -page, take an example's own hand-written launcher as the base
// and rewrite only what publication requires — the shared-runtime paths. This
// keeps the bespoke instructions in examples/Memory_span/web/index.html and
// examples/Reading-1/web/index.html rather than flattening them.
//
//	go run ./cmd/gen-wasm-launcher -app Memory_span -page examples/Memory_span/web/index.html -out …
//
// Either way the page loads sdl.js, sdl.wasm and wasm_exec.js from ../_runtime/
// rather than from its own directory: those three files are byte-identical in
// every bundle, so they are published once instead of ~79 times. sdl.wasm is
// fetched by the Emscripten glue rather than by a tag, which is why the page
// must also set Module.locateFile.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// runtimeDir is where the shared SDL runtime sits, relative to an app's page.
const runtimeDir = "../_runtime/"

// meta holds the per-example metadata read from meta.yaml. The parsing mirrors
// cmd/gen-gallery and cmd/gen-download-index, which read the same files (all
// three are package main, so the small overlap cannot be shared by importing).
type meta struct {
	category    string
	description string
	reference   string
}

// readMeta reads meta.yaml from dir. ok is false if the file is missing or
// carries neither a category nor a description.
func readMeta(dir string) (meta, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "meta.yaml"))
	if err != nil {
		return meta{}, false
	}
	var m meta
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

// unquote trims whitespace and strips surrounding double-quotes if present.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return s
}

// pageData is the template input.
type pageData struct {
	App         string
	Description string
	Reference   string
	RuntimeDir  string
}

// adapt rewrites an example's own launcher page to load the shared runtime.
//
// Only three things change, and each is required or the page breaks: the two
// <script src> tags, and the addition of Module.locateFile so the Emscripten
// glue fetches sdl.wasm from the shared directory too. Every rewrite is
// checked — a page that stops matching must fail loudly rather than be
// published subtly broken.
func adapt(html, app string) (string, error) {
	type rewrite struct{ what, from, to string }
	rewrites := []rewrite{
		{"sdl.js script tag", `<script src="sdl.js">`, `<script src="` + runtimeDir + `sdl.js">`},
		{"wasm_exec.js script tag", `<script src="wasm_exec.js">`, `<script src="` + runtimeDir + `wasm_exec.js">`},
		{
			"Module.locateFile",
			"var Module = {",
			"var Module = {\n        // sdl.wasm is published once for the whole collection, not per\n" +
				"        // app; the Emscripten glue fetches it by name, so redirect it.\n" +
				`        locateFile(path) { return ` + "'" + runtimeDir + "'" + ` + path; },`,
		},
	}
	for _, r := range rewrites {
		if strings.Count(html, r.from) != 1 {
			return "", fmt.Errorf("%s: expected exactly one %q in the page for %s, found %d",
				r.what, r.from, app, strings.Count(html, r.from))
		}
		html = strings.Replace(html, r.from, r.to, 1)
	}
	return html, nil
}

func main() {
	var (
		app      = flag.String("app", "", "example directory name (required)")
		page     = flag.String("page", "", "adapt this hand-written launcher instead of generating one")
		out      = flag.String("out", "", "output file (required)")
		examples = flag.String("examples", "examples", "directory holding the example sources")
	)
	flag.Parse()

	if *app == "" || *out == "" {
		log.Fatal("both -app and -out are required")
	}

	var html string
	if *page != "" {
		data, err := os.ReadFile(*page)
		if err != nil {
			log.Fatalf("reading %s: %v", *page, err)
		}
		html, err = adapt(string(data), *app)
		if err != nil {
			log.Fatalf("adapting %s: %v", *page, err)
		}
	} else {
		m, ok := readMeta(filepath.Join(*examples, *app))
		if !ok {
			log.Printf("WARNING: no usable meta.yaml for %q — the page will carry no description", *app)
		}
		var sb strings.Builder
		err := launcherTmpl.Execute(&sb, pageData{
			App:         *app,
			Description: m.description,
			Reference:   m.reference,
			RuntimeDir:  runtimeDir,
		})
		if err != nil {
			log.Fatalf("rendering the launcher for %s: %v", *app, err)
		}
		html = sb.String()
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatalf("creating output directory: %v", err)
	}
	if err := os.WriteFile(*out, []byte(html), 0o644); err != nil {
		log.Fatalf("writing %s: %v", *out, err)
	}
}
