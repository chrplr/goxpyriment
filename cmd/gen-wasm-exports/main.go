// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Command gen-wasm-exports generates the exhaustive list of C symbols that must
// be passed to Emscripten's -sEXPORTED_FUNCTIONS when linking SDL3.js/.wasm for
// a goxpyriment WebAssembly (GOOS=js) build.
//
// Without this, an experiment that calls an SDL/TTF/IMG function whose symbol
// was not exported fails at run time with "Module._SDL_Foo is not a function".
// This tool eliminates that whack-a-mole by scanning:
//
//  1. the vendored go-sdl3 js binding (*_js.go), to map each i<Name> binding
//     variable to the C symbol it calls via js.Global().Get("Module").Call(...);
//  2. the vendored go-sdl3 public API (non-js .go), to map each exported Go
//     function/method name to the i<Name> variables it uses (this resolves
//     renamed methods, e.g. Go Window.Size -> C SDL_GetWindowSize);
//  3. the goxpyriment source tree, to collect the set of call names actually
//     used.
//
// It emits wasm/exported_functions.json (a JSON array usable directly as
// emcc -sEXPORTED_FUNCTIONS=@wasm/exported_functions.json) plus a summary.
//
// Symbols reached only through a binding function that panics
// ("not implemented on js") are excluded, since those C symbols are typically
// absent from an Emscripten SDL3 build.
//
// Run from the repository root: go run ./cmd/gen-wasm-exports/
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	vendorSDL3 = "vendor/github.com/Zyko0/go-sdl3"
	outputPath = "wasm/exported_functions.json"
)

// Symbols the marshalling glue needs on every call, independent of which API
// functions an experiment happens to use.
var mandatory = []string{"_malloc", "_free", "_SDL_free", "_SDL_GetError"}

func main() {
	if _, err := os.Stat(vendorSDL3); err != nil {
		fatalf("cannot find %s (run from the repository root): %v", vendorSDL3, err)
	}

	// (1) i<Name> -> {C symbols}, and (2) Go name -> {i<Name>}, from go-sdl3.
	symOf := map[string][]string{}   // iVar -> C symbols (non-stubbed only)
	iVarsOf := map[string][]string{} // exported Go func/method name -> iVars
	for _, pkg := range []string{"sdl", "ttf", "img"} {
		dir := filepath.Join(vendorSDL3, pkg)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // package may be absent
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			if strings.HasSuffix(name, "_js.go") {
				collectBindingSymbols(path, symOf)
			} else {
				collectPublicAPI(path, iVarsOf)
			}
		}
	}

	// Resolve each exported Go name to its C symbols.
	symbolsForName := map[string][]string{}
	for goName, iVars := range iVarsOf {
		set := map[string]bool{}
		for _, iv := range iVars {
			for _, s := range symOf[iv] {
				set[s] = true
			}
		}
		if len(set) > 0 {
			symbolsForName[goName] = keys(set)
		}
	}

	// (3) Collect call names used across the goxpyriment tree.
	used := map[string]bool{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", ".git", "site", "_build":
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			collectCallNames(path, used)
		}
		return nil
	})
	if err != nil {
		fatalf("walking source tree: %v", err)
	}

	// Intersect: symbols reachable from goxpyriment's used call names.
	// A go-sdl3 name that goxpyriment uses but whose js binding is a stub (or is
	// commented out) has an entry in iVarsOf but no resolved symbol — it will
	// panic in the browser. Collect those as portability gaps.
	final := map[string]bool{}
	for _, s := range mandatory {
		final[s] = true
	}
	matched := 0
	gaps := map[string]bool{}
	for name := range used {
		if syms, ok := symbolsForName[name]; ok {
			matched++
			for _, s := range syms {
				final[s] = true
			}
		} else if _, isAPI := iVarsOf[name]; isAPI {
			gaps[name] = true
		}
	}
	gapList := keys(gaps)
	sort.Strings(gapList)

	symbols := keys(final)
	sort.Strings(symbols)

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		fatalf("creating output dir: %v", err)
	}
	blob, _ := json.MarshalIndent(symbols, "", "  ")
	if err := os.WriteFile(outputPath, append(blob, '\n'), 0o644); err != nil {
		fatalf("writing %s: %v", outputPath, err)
	}

	fmt.Fprintf(os.Stderr,
		"gen-wasm-exports: %d go-sdl3 API names mapped, %d matched in goxpyriment, %d C symbols → %s\n",
		len(symbolsForName), matched, len(symbols), outputPath)
	if len(gapList) > 0 {
		fmt.Fprintf(os.Stderr,
			"gen-wasm-exports: %d go-sdl3 calls used by goxpyriment have stubbed js bindings (will panic in the browser):\n",
			len(gapList))
		for _, g := range gapList {
			fmt.Fprintf(os.Stderr, "  %s\n", g)
		}
	}
}

// collectBindingSymbols parses a *_js.go file and records, for each
// `iName = func(...) {...}` binding, the C symbols it calls via Module.Call,
// unless the binding is a "not implemented on js" stub.
func collectBindingSymbols(path string, symOf map[string][]string) {
	f := parseFile(path)
	if f == nil {
		return
	}
	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		id, ok := as.Lhs[0].(*ast.Ident)
		if !ok || !isBindingVar(id.Name) {
			return true
		}
		lit, ok := as.Rhs[0].(*ast.FuncLit)
		if !ok {
			return true
		}
		if isStub(lit.Body) {
			return true
		}
		for _, s := range moduleCallSymbols(lit.Body) {
			symOf[id.Name] = append(symOf[id.Name], s)
		}
		return true
	})
}

// collectPublicAPI parses a non-js .go file and records, for each exported
// function/method declaration, the i<Name> binding variables it references.
func collectPublicAPI(path string, iVarsOf map[string][]string) {
	f := parseFile(path)
	if f == nil {
		return
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !fn.Name.IsExported() {
			continue
		}
		seen := map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && isBindingVar(id.Name) {
				seen[id.Name] = true
			}
			return true
		})
		for iv := range seen {
			iVarsOf[fn.Name.Name] = append(iVarsOf[fn.Name.Name], iv)
		}
	}
}

// collectCallNames records the selector name of every call `x.Name(...)`.
func collectCallNames(path string, used map[string]bool) {
	f := parseFile(path)
	if f == nil {
		return
	}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			used[sel.Sel.Name] = true
		}
		return true
	})
}

// moduleCallSymbols returns the string arguments of `.Call("_...")` expressions
// that look like exported C symbols.
func moduleCallSymbols(body *ast.BlockStmt) []string {
	var out []string
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Call" || len(call.Args) == 0 {
			return true
		}
		if s, ok := stringLit(call.Args[0]); ok && strings.HasPrefix(s, "_") {
			out = append(out, s)
		}
		return true
	})
	return out
}

// isStub reports whether a function body panics with "not implemented".
func isStub(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "panic" && len(call.Args) == 1 {
			if s, ok := stringLit(call.Args[0]); ok && strings.Contains(s, "not implemented") {
				found = true
			}
		}
		return true
	})
	return found
}

func isBindingVar(name string) bool {
	return len(name) >= 2 && name[0] == 'i' && name[1] >= 'A' && name[1] <= 'Z'
}

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

func parseFile(path string) *ast.File {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", path, err)
		return nil
	}
	return f
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen-wasm-exports: "+format+"\n", args...)
	os.Exit(1)
}
