//go:build js

// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package results

import (
	"log"
	"strings"
	"syscall/js"
)

// NewOutputFile creates a new OutputFile in memory.
func NewOutputFile(directory, filename string) (*OutputFile, error) {
	return &OutputFile{
		Filename:    filename,
		Directory:   directory,
		FullPath:    filename,
		CommentChar: OutputFileCommentChar,
		Buffer:      make([]string, 0),
	}, nil
}

// Save is a no-op in the browser: there is no filesystem to flush to, and a
// download per call would rain partial files on the participant (the metadata
// header alone is written before the first trial, and experiments typically
// flush after every block). The buffer is kept intact and written out once, by
// Finalize, at the end of the session.
func (o *OutputFile) Save() error {
	return nil
}

// Finalize triggers a browser download of everything buffered so far. It is
// called by Experiment.End; on desktop the equivalent simply flushes to disk.
//
// Experiment data does not come through here: DataFile.Finalize packs the CSV
// and the info file into one archive instead, because two downloads in a row
// lose the second one (see data_wasm.go). This path remains for a standalone
// OutputFile — a log, say — of which an experiment produces at most one.
func (o *OutputFile) Finalize() error {
	if len(o.Buffer) == 0 {
		return nil
	}

	content := strings.Join(o.Buffer, "")
	o.Buffer = make([]string, 0)

	log.Printf("Saving %s...", o.Filename)
	return downloadBytes(o.Filename, []byte(content), "text/plain")
}

// downloadBytes hands the participant a file, by wrapping the bytes in a Blob
// and clicking a hidden <a download>. It is the only way a wasm program can
// write anything the participant keeps.
//
// Call it once per session. The browser may be configured to ask where to save
// each file, in which case every call queues a modal dialog and only the first
// is shown; a second file requested before the first dialog is answered is
// cancelled outright when the page goes away.
func downloadBytes(filename string, data []byte, mime string) error {
	document := js.Global().Get("document")
	if document.IsUndefined() {
		log.Println("Warning: js document is undefined; cannot trigger download.")
		return nil
	}

	// Copy into a JS-owned buffer: a Go []byte is not addressable from JS, and
	// Blob accepts a typed array directly.
	buf := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buf, data)
	blob := js.Global().Get("Blob").New([]any{buf}, map[string]any{"type": mime})

	url := js.Global().Get("URL").Call("createObjectURL", blob)

	a := document.Call("createElement", "a")
	a.Set("href", url)
	a.Set("download", filename)
	a.Get("style").Set("display", "none")

	document.Get("body").Call("appendChild", a)
	a.Call("click")
	document.Get("body").Call("removeChild", a)

	js.Global().Get("URL").Call("revokeObjectURL", url)

	return nil
}
