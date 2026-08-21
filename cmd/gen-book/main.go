// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// gen-book concatenates the documentation in docs/ into one Markdown file that
// pandoc can turn into a single PDF, each page becoming a chapter.
//
// Run from the repo root via: make book
// (runs: go run ./cmd/gen-book/ then docs/make_book.sh)
//
// # Where the chapter order comes from
//
// From zensical.toml's nav, which already orders every page for the website.
// Reading it here rather than keeping a second list means the book cannot fall
// out of step with the site: a page added to the nav appears in both, and one
// that is not in the nav is in neither.
//
// # What has to be rewritten, and why
//
// Concatenating the files is the easy half. The links are the half that decides
// whether the result is a book or a stack of chapters:
//
//   - Cross-page links ("](UserManual.md)", 95 of them) point at files that do
//     not exist inside a PDF. They become links to that chapter's heading.
//   - Every heading gets an explicit, file-scoped identifier. Pandoc would
//     otherwise deduplicate repeated headings by appending -1, -2 — and with
//     "## See also" in four pages and "## The short version" in three, a
//     later chapter's own "](#see-also)" would silently resolve to the *first*
//     chapter's section. A link that works and goes to the wrong place is worse
//     than one that visibly fails, so the identifiers are made unique up front
//     and every in-page link is rewritten to match.
//   - Relative links that leave docs/ ("](../tests/)") cannot resolve in a PDF
//     either; they become absolute URLs into the GitHub repository.
//
// Absolute URLs, mailto: and root-absolute targets are left alone.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	navPath  = "zensical.toml"
	docsDir  = "docs"
	outPath  = "_build/book.md"
	repoTree = "https://github.com/chrplr/goxpyriment/tree/main/"
)

// navEntryRe pulls every "Name.md" out of zensical.toml's nav, in order.
// The nav is a list of inline tables, so the file names appear in reading
// order and nothing more clever is needed.
var navEntryRe = regexp.MustCompile(`"([A-Za-z0-9_.-]+\.md)"`)

// headingRe matches an ATX heading and captures its level and text.
var headingRe = regexp.MustCompile(`^(#{1,6})[ \t]+(.*?)[ \t]*#*[ \t]*$`)

// linkRe matches the target of a Markdown link or image: the "](target)" part.
var linkRe = regexp.MustCompile(`\]\(([^)]+)\)`)

// fenceRe matches a fenced code block delimiter, so that "#" inside a shell
// snippet is never mistaken for a heading.
var fenceRe = regexp.MustCompile("^\\s*(```|~~~)")

func main() {
	order, err := navOrder(navPath)
	if err != nil {
		fatal(err)
	}

	// First pass: every heading in every page, so links can be resolved before
	// any of them are written out.
	pages := make([]*page, 0, len(order))
	byFile := make(map[string]*page, len(order))
	for _, name := range order {
		p, err := readPage(name)
		if err != nil {
			fatal(err)
		}
		pages = append(pages, p)
		byFile[name] = p
	}

	// Second pass: rewrite and concatenate.
	var b strings.Builder
	b.WriteString("<!-- DO NOT EDIT. Generated from docs/ by cmd/gen-book. -->\n\n")
	var unresolved []string
	for _, p := range pages {
		out, missing := p.render(byFile)
		b.WriteString(out)
		b.WriteString("\n\n")
		unresolved = append(unresolved, missing...)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(outPath, []byte(b.String()), 0o644); err != nil {
		fatal(err)
	}

	// Report rather than fail. A link into a page that is not in the nav is a
	// real defect, but it is the docs' defect and not this tool's, and refusing
	// to build the book would only hide the rest of it.
	sort.Strings(unresolved)
	for _, u := range unresolved {
		fmt.Fprintf(os.Stderr, "gen-book: %s\n", u)
	}
	fmt.Printf("gen-book: %d chapters, %d headings → %s", len(pages), countHeadings(pages), outPath)
	if n := len(unresolved); n > 0 {
		fmt.Printf(" (%d link(s) could not be resolved; see above)", n)
	}
	fmt.Println()
}

// page is one source file: its text, and the identifier every heading in it
// will carry in the book.
type page struct {
	file    string            // "UserManual.md"
	slug    string            // "usermanual", the per-file identifier prefix
	lines   []string          //
	title   string            // from the page's YAML front matter, if it had any
	chapter string            // identifier of this page's first heading
	ids     map[string]string // original gfm identifier → book identifier (first occurrence)
	seq     []string          // book identifier of each heading, in document order
	count   map[string]int    // occurrences of each identifier within this page
}

func readPage(name string) (*page, error) {
	raw, err := os.ReadFile(filepath.Join(docsDir, name))
	if err != nil {
		return nil, fmt.Errorf("gen-book: %w", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	lines, title := stripFrontMatter(lines)
	p := &page{
		file:  name,
		slug:  slugify(strings.TrimSuffix(name, ".md")),
		lines: lines,
		title: title,
		ids:   map[string]string{},
		count: map[string]int{},
	}
	inFence := false
	for _, line := range p.lines {
		if fenceRe.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		m := headingRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id := slugify(stripInline(m[2]))
		bookID := p.slug + "--" + id
		// A heading repeated inside one page (docs/API.md repeats several)
		// must still get a unique label, or LaTeX reports multiply-defined
		// labels and a link lands on whichever it resolved last. Number the
		// repeats, exactly as pandoc would have, but per page.
		if n := p.count[id]; n > 0 {
			bookID = fmt.Sprintf("%s-%d", bookID, n)
		}
		p.count[id]++
		p.seq = append(p.seq, bookID)
		// First heading of the page is the chapter anchor, and links resolve to
		// the first occurrence — what a same-page link hit before the merge.
		if p.chapter == "" {
			p.chapter = bookID
		}
		if _, seen := p.ids[id]; !seen {
			p.ids[id] = bookID
		}
	}
	if p.chapter == "" {
		// A page with no headings at all still needs an anchor to link to,
		// and a chapter opener; render() synthesises one from the title.
		p.chapter = p.slug
		p.ids[""] = p.slug
	}
	return p, nil
}

// render returns the page's Markdown with headings carrying book-unique
// identifiers and every link rewritten, plus a list of link targets that could
// not be resolved.
func (p *page) render(byFile map[string]*page) (string, []string) {
	var b strings.Builder
	var missing []string
	inFence := false
	first := true
	n := 0 // index into p.seq, so each heading keeps the identifier it was given
	if !p.hasHeading() {
		// No heading anywhere in the page: open the chapter ourselves, or the
		// text would be swallowed into whatever chapter came before it.
		name := p.title
		if name == "" {
			name = strings.TrimSuffix(p.file, ".md")
		}
		b.WriteString("# " + name + " {#" + p.chapter + "}\n\n")
		first = false
	}
	for _, line := range p.lines {
		switch {
		case fenceRe.MatchString(line):
			inFence = !inFence
		case inFence:
			// Code is copied verbatim: no headings, no links.
		case strings.TrimRight(line, " \t") == "---":
			// A horizontal rule written as "---" is ambiguous once the pages
			// are concatenated: pandoc reads a "---" ... "---" pair as a YAML
			// metadata block wherever it appears, and docs/pre-built-examples.md
			// has exactly that shape around a note. "***" is a rule and nothing
			// else, so the ambiguity is removed rather than worked around.
			line = "***"
		default:
			if m := headingRe.FindStringSubmatch(line); m != nil {
				id := p.chapter
				if n < len(p.seq) {
					id = p.seq[n]
				}
				n++
				if first {
					// The chapter opener, whatever level it was written at.
					line = "# " + m[2] + " {#" + id + "}"
					first = false
				} else {
					line = m[1] + " " + m[2] + " {#" + id + "}"
				}
			}
			var miss []string
			line, miss = p.rewriteLinks(line, byFile)
			missing = append(missing, miss...)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String(), missing
}

// rewriteLinks resolves one line's link targets against the whole book.
func (p *page) rewriteLinks(line string, byFile map[string]*page) (string, []string) {
	var missing []string
	out := linkRe.ReplaceAllStringFunc(line, func(m string) string {
		target := m[2 : len(m)-1]
		if isAbsolute(target) {
			return m
		}
		switch {
		case strings.HasPrefix(target, "#"):
			// Same-page anchor: point it at this page's copy of the heading.
			if id, ok := p.ids[strings.TrimPrefix(target, "#")]; ok {
				return "](#" + id + ")"
			}
			missing = append(missing, fmt.Sprintf("%s: no heading for %q", p.file, target))
			return m

		case strings.HasSuffix(target, ".md"), strings.Contains(target, ".md#"):
			file, anchor, _ := strings.Cut(target, "#")
			file = strings.TrimPrefix(file, "./")
			dest, ok := byFile[file]
			if !ok {
				// Not a chapter — a page outside the nav, or a file elsewhere
				// in the repo. Send the reader to GitHub rather than nowhere.
				missing = append(missing, fmt.Sprintf("%s: %q is not in the nav; linked to GitHub instead", p.file, target))
				return "](" + repoTree + path(file) + ")"
			}
			if anchor == "" {
				return "](#" + dest.chapter + ")"
			}
			if id, ok := dest.ids[anchor]; ok {
				return "](#" + id + ")"
			}
			missing = append(missing, fmt.Sprintf("%s: %s has no heading %q", p.file, file, anchor))
			return "](#" + dest.chapter + ")"

		default:
			// An asset or a path out of docs/: assets stay relative (pandoc
			// resolves them with --resource-path), anything else goes to GitHub.
			if strings.HasPrefix(target, "assets/") || !strings.Contains(target, "/") {
				return m
			}
			return "](" + repoTree + path(strings.TrimPrefix(target, "../")) + ")"
		}
	})
	return out, missing
}

// hasHeading reports whether the page contains at least one ATX heading.
func (p *page) hasHeading() bool {
	_, synthesised := p.ids[""]
	return len(p.ids) > 0 && !synthesised
}

// stripFrontMatter removes a leading YAML metadata block and returns the lines
// that follow, plus the block's title if it declared one.
//
// Each page carries its own title/author/date for the per-page PDFs that
// make_pdfs.sh builds. Concatenated, those blocks are neither wanted nor
// mergeable — pandoc would take the first as the book's metadata and choke on
// the rest — so they come out here.
func stripFrontMatter(lines []string) ([]string, string) {
	if len(lines) == 0 || strings.TrimRight(lines[0], " \t") != "---" {
		return lines, ""
	}
	title := ""
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimRight(lines[i], " \t")
		if trimmed == "---" || trimmed == "..." {
			return lines[i+1:], title
		}
		if rest, ok := strings.CutPrefix(lines[i], "title:"); ok {
			title = strings.Trim(strings.TrimSpace(rest), `"'`)
		}
	}
	// Unterminated: it was not front matter after all.
	return lines, ""
}

// path cleans a repo-relative target for use in a GitHub tree URL.
func path(p string) string {
	if strings.HasPrefix(p, "../") {
		return strings.TrimPrefix(p, "../")
	}
	if strings.HasPrefix(p, "docs/") || strings.Contains(p, "/") {
		return p
	}
	return docsDir + "/" + p
}

// navOrder reads the page order out of zensical.toml's nav.
func navOrder(tomlPath string) ([]string, error) {
	raw, err := os.ReadFile(tomlPath)
	if err != nil {
		return nil, fmt.Errorf("gen-book: reading %s: %w", tomlPath, err)
	}
	text := string(raw)
	start := strings.Index(text, "nav = [")
	if start < 0 {
		return nil, fmt.Errorf("gen-book: no nav in %s", tomlPath)
	}
	// The nav ends at the first line that closes the top-level array.
	end := strings.Index(text[start:], "\n]")
	if end < 0 {
		return nil, fmt.Errorf("gen-book: unterminated nav in %s", tomlPath)
	}
	var order []string
	seen := map[string]bool{}
	for _, m := range navEntryRe.FindAllStringSubmatch(text[start:start+end], -1) {
		if name := m[1]; !seen[name] {
			seen[name] = true
			order = append(order, name)
		}
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("gen-book: nav in %s lists no pages", tomlPath)
	}
	return order, nil
}

// slugify reproduces the GitHub/python-markdown identifier scheme that both
// Zensical and `pandoc --from=markdown+gfm_auto_identifiers` use, so an
// identifier written by hand in the docs resolves the same way here.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}

// stripInline removes the inline markup that does not survive into an
// identifier: code ticks, emphasis, and link syntax around the text.
func stripInline(s string) string {
	s = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`).ReplaceAllString(s, "$1")
	s = strings.NewReplacer("`", "", "*", "", "_", "").Replace(s)
	return s
}

func isAbsolute(target string) bool {
	switch {
	case strings.HasPrefix(target, "/"):
		return true
	case strings.HasPrefix(target, "mailto:"):
		return true
	case strings.Contains(target, "://"):
		return true
	}
	return false
}

func countHeadings(pages []*page) int {
	n := 0
	for _, p := range pages {
		n += len(p.ids)
	}
	return n
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
