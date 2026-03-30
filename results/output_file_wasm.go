//go:build js

// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

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

// Save triggers a browser download of the buffered content.
func (o *OutputFile) Save() error {
	if len(o.Buffer) == 0 {
		return nil
	}

	content := strings.Join(o.Buffer, "")
	o.Buffer = make([]string, 0)

	// Log to console for debugging
	log.Printf("Saving experiment results to %s...", o.Filename)

	// Use syscall/js to trigger a download in the browser
	document := js.Global().Get("document")
	if document.IsUndefined() {
		log.Println("Warning: js document is undefined; cannot trigger download.")
		return nil
	}

	// Create a Blob from the content
	blob := js.Global().Get("Blob").New([]any{content}, map[string]any{
		"type": "text/plain",
	})

	// Create a URL for the Blob
	url := js.Global().Get("URL").Call("createObjectURL", blob)

	// Create a hidden <a> element
	a := document.Call("createElement", "a")
	a.Set("href", url)
	a.Set("download", o.Filename)
	a.Get("style").Set("display", "none")

	// Append to body, click, and remove
	document.Get("body").Call("appendChild", a)
	a.Call("click")
	document.Get("body").Call("removeChild", a)

	// Revoke the URL to free memory
	js.Global().Get("URL").Call("revokeObjectURL", url)

	return nil
}
