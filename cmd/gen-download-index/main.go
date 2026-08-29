// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// gen-download-index generates the download page published alongside the
// per-app zips in the Cloudflare R2 bucket served at downloads.pallier.org.
//
// It has two modes.
//
// Index mode (the default) scans the packaged tree produced by
// examples/installers/package-per-app.sh:
//
//	_build/r2/<OS_ARCH>/<app>.zip
//
// and writes a self-contained index.html listing every app that actually
// built, with a download link per platform. Descriptions come from each
// program's meta.yaml — the same files that feed docs/GalleryOfExamples.md —
// so the page groups programs into experiments, demonstrations and technical
// tests exactly as the gallery does.
//
//	go run ./cmd/gen-download-index -root _build/r2 -commit "$GITHUB_SHA" -tag v1.2.3
//
// Redirect mode writes a ~2 KB page that forwards to a commit's index. It is
// what makes builds/index.html and builds/latest/index.html stable entry
// points without duplicating gigabytes of binaries:
//
//	go run ./cmd/gen-download-index -redirect "$GITHUB_SHA" -out _build/redirect.html
//
// Links in the index are relative by default, so the generated file is correct
// wherever it is uploaded. Pass -base to emit absolute URLs instead.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// platform pairs an R2 folder name with the two-line table header shown above
// its column. OS and Arch are separate so the template can break the line
// without either field carrying markup.
type platform struct {
	Dir  string
	OS   string
	Arch string
}

// platforms is the column order of the download tables.
var platforms = []platform{
	{"Windows_x86_64", "Windows", "x86-64"},
	{"MacOS_arm64", "macOS", "Apple silicon"},
	{"Linux_x86_64", "Linux", "x86-64"},
	{"Linux_arm64", "Linux", "arm64"},
}

// bundle is one whole-collection archive, for people who want everything.
//
// These are NOT mirrored into the bucket. They are byte-for-byte the same
// binaries as the per-app zips, so hosting both would double the storage for
// no gain; the GitHub release already serves them and keeps them indefinitely.
type bundle struct {
	File  string
	Label string
}

var bundles = []bundle{
	{"goxpyriment-examples-windows-x86_64.zip", "Windows x86-64"},
	{"goxpyriment-examples-macos-arm64.zip", "macOS Apple silicon"},
	{"goxpyriment-examples-linux-x86_64.tar.gz", "Linux x86-64"},
	{"goxpyriment-examples-linux-arm64.tar.gz", "Linux arm64"},
}

// releaseDownloadURL is where the collection archives are served from. The
// "latest" alias resolves to the most recent published release.
const releaseDownloadURL = "https://github.com/chrplr/goxpyriment/releases/latest/download"

// section groups apps by meta.yaml category, in the order they are rendered.
type section struct {
	Category string
	Title    string
	Blurb    string
}

var sections = []section{
	{"experiment", "Experiments", "Complete paradigms that record behavioural data to a CSV file."},
	{"demo", "Demonstrations", "Short programs showing one feature or illusion. Nothing is measured."},
	{"test", "Technical tests", "Timing, display and hardware-trigger checks, run and inspected by hand."},
	{"", "Other programs", "Programs without a meta.yaml entry."},
}

// meta holds the per-program metadata read from meta.yaml. The parsing mirrors
// cmd/gen-gallery, which reads the same files (both are package main, so the
// small overlap cannot be shared through an import).
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

// download is one platform cell: either a link with its size, or absent.
type download struct {
	URL     string
	Size    string
	Present bool
}

// app is one program as rendered in the tables.
type app struct {
	Name        string
	Description string
	Reference   string
	Downloads   []download // one per entry of platforms, in the same order
}

// renderSection is a section paired with the apps that belong to it.
type renderSection struct {
	Title string
	Blurb string
	Apps  []app
}

// bundleLink is one whole-collection archive as rendered.
type bundleLink struct {
	Label string
	URL   string
}

// pageData is the template input.
type pageData struct {
	Commit    string
	ShortSHA  string
	Tag       string
	Built     string
	CanonURL  string
	Sections  []renderSection
	Bundles   []bundleLink
	Platforms []platform
	NumApps   int
}

// humanSize formats a byte count for display, e.g. "5.2 MB".
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 3 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

// scan walks root/<OS_ARCH>/*.zip and returns, per app name, the file size
// found for each platform directory.
func scan(root string) (map[string]map[string]int64, error) {
	found := make(map[string]map[string]int64)
	for _, p := range platforms {
		entries, err := os.ReadDir(filepath.Join(root, p.Dir))
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("WARNING: no %s directory in %s — that platform will be blank", p.Dir, root)
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			name, ok := strings.CutSuffix(e.Name(), ".zip")
			if !ok || e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				return nil, err
			}
			if found[name] == nil {
				found[name] = make(map[string]int64)
			}
			found[name][p.Dir] = info.Size()
		}
	}
	return found, nil
}

// lookupMeta finds a program's metadata by looking in examples/ then tests/.
// A program with no usable meta.yaml still gets listed — the download exists —
// but lands in the "Other programs" section.
func lookupMeta(name string) meta {
	for _, root := range []string{"examples", "tests"} {
		if m, ok := readMeta(filepath.Join(root, name)); ok {
			return m
		}
	}
	log.Printf("WARNING: no meta.yaml for %q — listed under \"Other programs\"", name)
	return meta{}
}

// linkTo builds the href for a path inside the build directory.
func linkTo(base, path string) string {
	if base == "" {
		return path
	}
	return strings.TrimSuffix(base, "/") + "/" + path
}

func main() {
	var (
		root     = flag.String("root", "_build/r2", "directory holding the packaged <OS_ARCH>/<app>.zip tree")
		out      = flag.String("out", "", "output file (default: <root>/index.html, or required with -redirect)")
		commit   = flag.String("commit", "", "commit SHA this build was made from")
		tag      = flag.String("tag", "", "git tag this build was made from, if any")
		base     = flag.String("base", "", "URL prefix for links (default: empty, i.e. relative links)")
		redirect = flag.String("redirect", "", "redirect mode: write a page forwarding to ../<SHA>/index.html")
	)
	flag.Parse()

	if *redirect != "" {
		if *out == "" {
			log.Fatal("-redirect requires -out")
		}
		writeRedirect(*out, *redirect)
		return
	}

	outPath := *out
	if outPath == "" {
		outPath = filepath.Join(*root, "index.html")
	}

	found, err := scan(*root)
	if err != nil {
		log.Fatalf("scanning %s: %v", *root, err)
	}
	if len(found) == 0 {
		log.Fatalf("no <OS_ARCH>/<app>.zip files under %s — run examples/installers/package-per-app.sh first", *root)
	}

	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})

	// Bucket the apps by category.
	byCategory := make(map[string][]app)
	for _, name := range names {
		m := lookupMeta(name)
		a := app{Name: name, Description: m.description, Reference: m.reference}
		for _, p := range platforms {
			size, ok := found[name][p.Dir]
			if !ok {
				a.Downloads = append(a.Downloads, download{})
				continue
			}
			a.Downloads = append(a.Downloads, download{
				URL:     linkTo(*base, p.Dir+"/"+name+".zip"),
				Size:    humanSize(size),
				Present: true,
			})
		}
		cat := m.category
		if cat != "experiment" && cat != "demo" && cat != "test" {
			cat = ""
		}
		byCategory[cat] = append(byCategory[cat], a)
	}

	var rendered []renderSection
	for _, s := range sections {
		if len(byCategory[s.Category]) == 0 {
			continue
		}
		rendered = append(rendered, renderSection{Title: s.Title, Blurb: s.Blurb, Apps: byCategory[s.Category]})
	}

	// The whole-collection archives, served from the GitHub release.
	var bl []bundleLink
	for _, b := range bundles {
		bl = append(bl, bundleLink{Label: b.Label, URL: releaseDownloadURL + "/" + b.File})
	}

	short := *commit
	if len(short) > 12 {
		short = short[:12]
	}
	data := pageData{
		Commit:    *commit,
		ShortSHA:  short,
		Tag:       *tag,
		Built:     time.Now().UTC().Format("2006-01-02 15:04 MST"),
		Sections:  rendered,
		Bundles:   bl,
		Platforms: platforms,
		NumApps:   len(names),
	}
	if *commit != "" {
		data.CanonURL = "https://downloads.pallier.org/builds/" + *commit + "/index.html"
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		log.Fatalf("creating output directory: %v", err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("create %s: %v", outPath, err)
	}
	if err := indexTmpl.Execute(f, data); err != nil {
		log.Fatalf("rendering %s: %v", outPath, err)
	}
	if err := f.Close(); err != nil {
		log.Fatalf("close %s: %v", outPath, err)
	}

	fmt.Printf("Wrote %s — %d programs across %d platforms.\n", outPath, len(names), len(platforms))
}

// writeRedirect emits the small forwarding page used for builds/index.html and
// builds/latest/index.html.
func writeRedirect(outPath, sha string) {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		log.Fatalf("creating output directory: %v", err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("create %s: %v", outPath, err)
	}
	short := sha
	if len(short) > 12 {
		short = short[:12]
	}
	err = redirectTmpl.Execute(f, struct{ SHA, ShortSHA string }{sha, short})
	if err != nil {
		log.Fatalf("rendering %s: %v", outPath, err)
	}
	if err := f.Close(); err != nil {
		log.Fatalf("close %s: %v", outPath, err)
	}
	fmt.Printf("Wrote %s — redirects to ../%s/index.html\n", outPath, sha)
}
