// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// gen-jatos-jzip packs a browser bundle into a JATOS study archive (.jzip),
// ready for Import Study in the JATOS GUI (https://www.jatos.org).
//
// A .jzip is a zip holding the study assets directory and a .jas file — the
// study's JSON description. The layout and the .jas schema (version "3") are
// those of the archives JATOS itself exports; they were taken from
// test/resources/potato_compass.jzip in the JATOS repository (v3.11.3).
//
//	go run ./cmd/gen-jatos-jzip -app Stroop_task -bundle _build/jatos/Stroop_task -out _build/jatos/Stroop_task.jzip
//
// The study has one component, the bundle's index.html, which must be the
// launcher rendered by gen-wasm-launcher -jatos. Every UUID is derived from the
// example's name, so importing a rebuilt archive updates the study already on
// the server (JATOS asks before overwriting) instead of creating a second one.
//
// The study assets directory is named goxpy_<app>: a JATOS server keeps every
// study's assets side by side, and a bare example name is likely to collide
// with someone else's study on a shared server.
package main

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// jas mirrors the parts of the JATOS study description that an import reads.
type jas struct {
	Version string  `json:"version"`
	Data    jasData `json:"data"`
}

type jasData struct {
	UUID           string         `json:"uuid"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Active         bool           `json:"active"`
	GroupStudy     bool           `json:"groupStudy"`
	LinearStudy    bool           `json:"linearStudy"`
	DirName        string         `json:"dirName"`
	Comments       string         `json:"comments"`
	JSONData       *string        `json:"jsonData"`
	EndRedirectURL *string        `json:"endRedirectUrl"`
	ComponentList  []jasComponent `json:"componentList"`
	BatchList      []jasBatch     `json:"batchList"`
}

type jasComponent struct {
	UUID         string  `json:"uuid"`
	Title        string  `json:"title"`
	HTMLFilePath string  `json:"htmlFilePath"`
	Reloadable   bool    `json:"reloadable"`
	Active       bool    `json:"active"`
	Comments     string  `json:"comments"`
	JSONData     *string `json:"jsonData"`
}

type jasBatch struct {
	UUID               string   `json:"uuid"`
	Title              string   `json:"title"`
	Active             bool     `json:"active"`
	MaxActiveMembers   *int     `json:"maxActiveMembers"`
	MaxTotalMembers    *int     `json:"maxTotalMembers"`
	MaxTotalWorkers    *int     `json:"maxTotalWorkers"`
	AllowedWorkerTypes []string `json:"allowedWorkerTypes"`
	Comments           *string  `json:"comments"`
	JSONData           *string  `json:"jsonData"`
}

// nameUUID derives a stable UUID (version 5 layout, SHA-1 of a name) so the
// same example always yields the same study, component and batch.
func nameUUID(name string) string {
	h := sha1.Sum([]byte("goxpyriment/jatos/" + name))
	h[6] = (h[6] & 0x0f) | 0x50
	h[8] = (h[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

// readDescription returns the description line of the example's meta.yaml, or
// "" if there is none.
func readDescription(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "meta.yaml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(line), ": ")
		if ok && strings.TrimSpace(key) == "description" {
			val = strings.TrimSpace(val)
			if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
				val = val[1 : len(val)-1]
			}
			return val
		}
	}
	return ""
}

func main() {
	var (
		app      = flag.String("app", "", "example directory name (required)")
		bundle   = flag.String("bundle", "", "directory holding the browser bundle: index.html, main.wasm, sdl.js, sdl.wasm, wasm_exec.js (required)")
		out      = flag.String("out", "", "output .jzip file (required)")
		examples = flag.String("examples", "examples", "directory holding the example sources")
	)
	flag.Parse()
	if *app == "" || *bundle == "" || *out == "" {
		log.Fatal("-app, -bundle and -out are required")
	}
	for _, f := range []string{"index.html", "main.wasm", "sdl.js", "sdl.wasm", "wasm_exec.js"} {
		if _, err := os.Stat(filepath.Join(*bundle, f)); err != nil {
			log.Fatalf("bundle is incomplete: %v", err)
		}
	}

	dirName := "goxpy_" + *app
	study := jas{
		Version: "3",
		Data: jasData{
			UUID:        nameUUID(*app + "/study"),
			Title:       *app,
			Description: readDescription(filepath.Join(*examples, *app)),
			Active:      true,
			DirName:     dirName,
			Comments:    "Built with goxpyriment (https://github.com/chrplr/goxpyriment).",
			ComponentList: []jasComponent{{
				UUID:         nameUUID(*app + "/component"),
				Title:        *app,
				HTMLFilePath: "index.html",
				// A reload would restart the experiment from the top and
				// append a second copy of the first blocks to the result
				// data. Not reloadable, JATOS ends the run instead.
				Reloadable: false,
				Active:     true,
			}},
			BatchList: []jasBatch{{
				UUID:               nameUUID(*app + "/batch"),
				Title:              "Default",
				Active:             true,
				AllowedWorkerTypes: []string{"PersonalSingle", "Jatos", "PersonalMultiple"},
			}},
		},
	}
	jasBytes, err := json.MarshalIndent(study, "", "  ")
	if err != nil {
		log.Fatalf("encoding the .jas: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatalf("creating output directory: %v", err)
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatalf("creating %s: %v", *out, err)
	}
	zw := zip.NewWriter(f)
	now := time.Now()

	add := func(name string, r io.Reader) {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: now})
		if err != nil {
			log.Fatalf("adding %s: %v", name, err)
		}
		if _, err := io.Copy(w, r); err != nil {
			log.Fatalf("writing %s: %v", name, err)
		}
	}

	err = filepath.WalkDir(*bundle, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(*bundle, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(filepath.Base(rel), ".") {
			return nil // build logs and the like
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		add(dirName+"/"+filepath.ToSlash(rel), src)
		return nil
	})
	if err != nil {
		log.Fatalf("packing %s: %v", *bundle, err)
	}
	add(*app+".jas", strings.NewReader(string(jasBytes)))

	if err := zw.Close(); err != nil {
		log.Fatalf("closing %s: %v", *out, err)
	}
	if err := f.Close(); err != nil {
		log.Fatalf("closing %s: %v", *out, err)
	}
}
