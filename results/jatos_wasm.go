//go:build js

// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package results

import (
	"fmt"
	"regexp"
	"syscall/js"
)

// JATOS (https://www.jatos.org) hosts browser experiments and stores their
// results on its server. A page it serves loads jatos.js, which defines the
// global `jatos`; when that object exists, the data file goes to the server
// instead of being downloaded (see data_wasm.go). Nothing here talks HTTP
// directly: jatos.js already knows the study run's URLs and retries failed
// requests (jatos.httpRetry, 5 by default).
//
// Every jatos.js call returns a promise. They are queued on one chain so that
// appends reach the server in the order they were made — result data is
// concatenated server-side, so a reordering would scramble the CSV — and so
// that the trial loop never waits on the network. Finalize waits for the whole
// chain before the session is allowed to end.

// jatosChain is the tail of the queue of pending jatos.js calls. It is a
// native Promise; jatos.js returns jQuery promises, which a native .then
// adopts.
var jatosChain js.Value

// jatosErrors collects the rejection reasons of queued calls, in a JS array
// so that recording one needs no Go code (see jatosEnqueue).
var jatosErrors js.Value

// JatosAvailable reports whether the page was served by JATOS, i.e. whether
// jatos.js has defined its global object.
func JatosAvailable() bool {
	j := js.Global().Get("jatos")
	return !j.IsUndefined() && !j.IsNull()
}

// jatosEnqueue appends the call jatos[fn](args...) to the chain. It returns at
// once; the call runs when everything queued before it has completed. Once a
// call has failed, later calls still run (an upload is worth attempting even
// if an append was refused); the reasons are kept for jatosAwait.
//
// Every step of the chain is a plain JS function — bound methods and
// Function.prototype, the built-in function that returns undefined — never a
// js.FuncOf. A Go callback can only run while the Go program is alive, and a
// session that crashes exits with appends still queued: with Go callbacks in
// the chain, those were dropped ("Go program has already exited"), losing the
// block saved just before the crash. With plain JS they are sent regardless.
//
// Each step first maps the previous value to undefined: a bound method gets
// the value its promise resolves with as an extra trailing argument, which
// jatos.js would take for an onSuccess callback.
func jatosEnqueue(fn string, args ...any) {
	if jatosChain.IsUndefined() {
		jatosChain = js.Global().Get("Promise").Call("resolve")
		jatosErrors = js.Global().Get("Array").New()
	}
	jatos := js.Global().Get("jatos")
	call := jatos.Get(fn).Call("bind", append([]any{jatos}, args...)...)
	record := jatosErrors.Get("push").Call("bind", jatosErrors)
	toUndefined := js.Global().Get("Function").Get("prototype")
	jatosChain = jatosChain.Call("then", toUndefined).Call("then", call).Call("catch", record)
}

// jatosAwait blocks until every queued call has settled and returns the first
// error among them. The calling goroutine parks on a channel that a promise
// callback fills — the same pattern sdl.WaitAnimationFrame uses for every
// frame — so it must not itself be running inside a JS callback.
func jatosAwait() error {
	if jatosChain.IsUndefined() {
		return nil
	}
	done := make(chan struct{})
	settle := js.FuncOf(func(js.Value, []js.Value) any {
		close(done)
		return nil
	})
	jatosChain.Call("then", settle)
	<-done
	settle.Release()

	n := jatosErrors.Length()
	if n == 0 {
		return nil
	}
	err := fmt.Errorf("jatos.js: %s", jsErrorText(jatosErrors.Index(0)))
	if n > 1 {
		err = fmt.Errorf("%w (and %d more failed requests)", err, n-1)
	}
	jatosErrors.Set("length", 0)
	return err
}

// jatosIllegalInFilename is the set of characters JATOS refuses in the name of
// an uploaded result file (IOUtils.REGEX_ILLEGAL_IN_FILENAME in JATOS 3.11):
// whitespace and the characters below. The upload of a name containing one
// fails with "400 Bad filename" — which every experiment named with a space
// ("Simon Task") would hit.
var jatosIllegalInFilename = regexp.MustCompile("[\\s*?\"\\\\\\x00/,`<>|:~!§$%&^°]")

// JatosFilename makes name acceptable to JATOS as a result file name by
// replacing each refused character with '_', as JATOS itself does when it
// derives file names. Only uploads are renamed; the desktop and .zip names
// are left as they are.
func JatosFilename(name string) string {
	return jatosIllegalInFilename.ReplaceAllString(name, "_")
}

// jsErrorText renders a promise rejection reason. jatos.js rejects with a
// plain string, a jqXHR, or an Error, depending on where the failure arose.
func jsErrorText(v js.Value) string {
	switch v.Type() {
	case js.TypeString:
		return v.String()
	case js.TypeObject:
		if s := v.Get("status"); s.Type() == js.TypeNumber {
			return fmt.Sprintf("HTTP %d %s", s.Int(), v.Get("statusText").String())
		}
		if m := v.Get("message"); m.Type() == js.TypeString {
			return m.String()
		}
	}
	if v.IsUndefined() || v.IsNull() {
		return "unknown error"
	}
	return v.Call("toString").String()
}
