//go:build js

// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"fmt"
	"runtime/debug"
	"syscall/js"

	"github.com/chrplr/goxpyriment/results"
)

// platformHandleCrash reports an unrecovered panic in the browser: it logs the
// panic value and Go stack to the console (for the developer) and renders a
// full-page error overlay (for the participant), then returns true so
// Experiment.Run swallows the panic. It also sets e.crashed, which makes
// finalizeData skip the download of the partial, half-written data files — a
// crashed session should not hand the participant a broken .csv. Under JATOS,
// the rows of the blocks saved before the crash are already on the server.
//
// There is no filesystem in the browser, so a panic that would abort a desktop
// run instead used to unwind silently into End(), downloading empty files with
// no on-screen indication that anything went wrong. This makes the failure
// visible instead.
func (e *Experiment) platformHandleCrash(r any) bool {
	e.crashed = true
	// The launcher page ends a JATOS study run as failed on this, leaving the
	// overlay in place rather than moving on to the end page.
	platformReportOutcome(false, fmt.Sprintf("experiment crashed: %v", r))

	// Developer-facing: full detail in the browser console.
	stack := debug.Stack()
	if console := js.Global().Get("console"); !console.IsUndefined() {
		console.Call("error", fmt.Sprintf("goxpyriment: experiment crashed: %v\n%s", r, stack))
	}

	document := js.Global().Get("document")
	if document.IsUndefined() || document.Get("body").IsUndefined() {
		return true
	}

	overlay := document.Call("createElement", "div")
	setStyles(overlay, map[string]string{
		"position":       "fixed",
		"inset":          "0",
		"zIndex":         "2147483647",
		"display":        "flex",
		"flexDirection":  "column",
		"alignItems":     "center",
		"justifyContent": "center",
		"gap":            "0.8em",
		"padding":        "2em",
		"boxSizing":      "border-box",
		"background":     "rgba(24,0,0,0.94)",
		"color":          "#ffd7d7",
		"font":           "16px/1.5 system-ui, -apple-system, sans-serif",
		"textAlign":      "center",
	})

	title := document.Call("createElement", "div")
	title.Set("textContent", "The experiment stopped unexpectedly.")
	setStyles(title, map[string]string{"fontSize": "1.4em", "fontWeight": "bold"})

	hint := document.Call("createElement", "div")
	if results.JatosAvailable() {
		// The blocks the experiment saved before the crash were appended to
		// the JATOS result data as it went (results/data_wasm.go). Restarting
		// is the experimenter's call: JATOS normally refuses a second run of
		// the same study link.
		hint.Set("textContent", "Any blocks completed before this point were sent to the server. "+
			"Please tell the experimenter what happened.")
	} else {
		hint.Set("textContent", "No data file was saved. Reload the page to start again.")
	}

	detail := document.Call("createElement", "pre")
	detail.Set("textContent", fmt.Sprintf("%v", r))
	setStyles(detail, map[string]string{
		"maxWidth":   "90%",
		"maxHeight":  "40%",
		"overflow":   "auto",
		"opacity":    "0.7",
		"fontSize":   "0.8em",
		"whiteSpace": "pre-wrap",
	})

	overlay.Call("appendChild", title)
	overlay.Call("appendChild", hint)
	overlay.Call("appendChild", detail)
	document.Get("body").Call("appendChild", overlay)
	return true
}

// setStyles applies a set of CSS properties to a DOM element's inline style.
func setStyles(el js.Value, styles map[string]string) {
	style := el.Get("style")
	for k, v := range styles {
		style.Set(k, v)
	}
}
