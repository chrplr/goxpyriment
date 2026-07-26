// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// gen-readme regenerates README.md (repo root) from docs/index.md.
//
// Run from the repo root via: make readme
// (runs: go run ./cmd/gen-readme/)
//
// docs/index.md is the single source of truth for the landing page. It is
// rendered by Zensical and served at the site root, so its relative links are
// bare filenames (e.g. "Installation.md", "icon_512.png"). README.md, by
// contrast, lives at the repo root and is rendered by GitHub, where those same
// targets must be reached through the docs/ subdirectory. This tool copies the
// content verbatim and rewrites every *relative* Markdown link/image target by
// prefixing "docs/". Absolute URLs (http/https), in-page anchors (#...),
// root-absolute (/...) and mailto: targets are left untouched.
//
// A CI guard re-runs this generator and fails if README.md is out of date, so
// editing docs/index.md without regenerating cannot silently drift.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	srcPath = "docs/index.md"
	dstPath = "README.md"
)

// header is prepended to the generated file so nobody edits README.md by hand.
const header = "<!-- DO NOT EDIT. Generated from docs/index.md by cmd/gen-readme. " +
	"Edit docs/index.md, then run `make readme`. -->\n\n"

// linkRe matches the target of a Markdown link or image: the "](target)" part.
var linkRe = regexp.MustCompile(`\]\(([^)]+)\)`)

// isAbsolute reports whether a link target must be left as-is (not relative to
// the docs/ directory).
func isAbsolute(target string) bool {
	switch {
	case strings.HasPrefix(target, "#"): // in-page anchor
		return true
	case strings.HasPrefix(target, "/"): // root-absolute path
		return true
	case strings.HasPrefix(target, "mailto:"):
		return true
	case strings.Contains(target, "://"): // http://, https://, ftp://, ...
		return true
	}
	return false
}

// rewriteLinks prefixes "docs/" onto every relative link/image target.
func rewriteLinks(content string) string {
	return linkRe.ReplaceAllStringFunc(content, func(m string) string {
		// m is the whole "](target)"; the target is between "](" and ")".
		target := m[2 : len(m)-1]
		// A target may carry a title: (url "Some title"). Only the URL token
		// (everything up to the first space) is a path to rewrite.
		url, rest := target, ""
		if i := strings.IndexByte(target, ' '); i >= 0 {
			url, rest = target[:i], target[i:]
		}
		if isAbsolute(url) {
			return m
		}
		return "](docs/" + url + rest + ")"
	})
}

func main() {
	src, err := os.ReadFile(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-readme: %v\n", err)
		os.Exit(1)
	}

	out := header + rewriteLinks(string(src))

	if err := os.WriteFile(dstPath, []byte(out), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gen-readme: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %s from %s\n", dstPath, srcPath)
}
