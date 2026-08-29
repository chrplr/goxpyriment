// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import "html/template"

// pageCSS is shared by both pages. It is inlined rather than linked: the bucket
// serves static files with no build step, and a single self-contained file
// cannot break because a stylesheet was pruned with an old build.
const pageCSS = `
:root {
  color-scheme: light dark;
  --bg: #ffffff; --fg: #1b1b1b; --muted: #5c5c5c;
  --rule: #e2e2e2; --accent: #0b5fa5; --panel: #f6f7f9;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #17191c; --fg: #e6e6e6; --muted: #a0a4aa;
    --rule: #2e3238; --accent: #6fb2f0; --panel: #202429;
  }
}
* { box-sizing: border-box; }
body {
  margin: 0; padding: 2rem 1.25rem 4rem;
  background: var(--bg); color: var(--fg);
  font: 16px/1.55 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
}
main { max-width: 62rem; margin: 0 auto; }
h1 { font-size: 1.7rem; margin: 0 0 .4rem; }
h2 { font-size: 1.25rem; margin: 2.5rem 0 .3rem; padding-top: 1rem; border-top: 1px solid var(--rule); }
p { margin: .4rem 0; }
a { color: var(--accent); }
.muted { color: var(--muted); font-size: .9rem; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: .87em; }
.build { background: var(--panel); border: 1px solid var(--rule); border-radius: 6px;
         padding: .75rem 1rem; margin: 1.25rem 0; }
.build dl { display: grid; grid-template-columns: max-content 1fr; gap: .15rem .8rem; margin: 0; }
.build dt { color: var(--muted); }
.build dd { margin: 0; }
.scroll { overflow-x: auto; }
table { border-collapse: collapse; width: 100%; margin: .8rem 0 0; font-size: .93rem; }
th, td { text-align: left; padding: .5rem .6rem; border-bottom: 1px solid var(--rule); vertical-align: top; }
th { font-weight: 600; color: var(--muted); font-size: .82rem; white-space: nowrap; }
td.dl { white-space: nowrap; }
td.dl .size { display: block; color: var(--muted); font-size: .78rem; }
td.none { color: var(--muted); }
.name { font-weight: 600; }
.ref { color: var(--muted); font-style: italic; font-size: .86rem; }
.lede { background: var(--panel); border: 1px solid var(--rule); border-radius: 6px;
        padding: .75rem 1rem; margin: 1rem 0; }
a.run { font-weight: 600; }
.notes { margin-top: 2.5rem; padding-top: 1rem; border-top: 1px solid var(--rule); font-size: .92rem; }
.notes li { margin-bottom: .5rem; }
.center { text-align: center; margin-top: 18vh; }
`

// indexTmpl renders the per-commit download page.
var indexTmpl = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>goxpyriment downloads</title>
<style>` + pageCSS + `</style>
</head>
<body>
<main>

<h1>goxpyriment &mdash; ready-to-run programs</h1>
<p class="muted">{{.NumApps}} program{{if ne .NumApps 1}}s{{end}}, built for four platforms. Pick one, or take the whole collection below.</p>
{{if .NumBrowser}}
<p class="lede">{{.NumBrowser}} of them also <strong>run directly in your browser</strong> &mdash;
no download, no install. Use a focused, foreground tab; results arrive as a single
<code>.zip</code> when the session ends. Paradigms needing sub-millisecond stimulus
onset (rapid RSVP, subliminal priming) should still be run natively.</p>
{{end}}

<div class="build">
  <dl>
    {{if .Tag}}<dt>Release</dt><dd>{{.Tag}}</dd>{{end}}
    {{if .Commit}}<dt>Commit</dt><dd><code>{{.ShortSHA}}</code></dd>{{end}}
    <dt>Built</dt><dd>{{.Built}}</dd>
    {{if .CanonURL}}<dt>Permalink</dt><dd><a href="{{.CanonURL}}">{{.CanonURL}}</a></dd>{{end}}
  </dl>
</div>

{{if .Bundles}}
<h2>Download everything</h2>
<p>One archive with every program for a given platform, from the
<a href="https://github.com/chrplr/goxpyriment/releases/latest">latest release</a>
on GitHub. These are several hundred MB each, and unlike the links below they
are kept indefinitely.</p>
<div class="scroll">
<table>
<tbody>
{{range .Bundles}}
<tr>
  <td class="name">{{.Label}}</td>
  <td class="dl"><a href="{{.URL}}">Download</a></td>
</tr>
{{end}}
</tbody>
</table>
</div>
{{end}}

{{range .Sections}}
<h2>{{.Title}}</h2>
<p class="muted">{{.Blurb}}</p>
<div class="scroll">
<table>
<thead>
<tr>
  <th>Program</th>
  <th>Run in<br>browser</th>
  {{range $.Platforms}}<th>{{.OS}}<br>{{.Arch}}</th>{{end}}
</tr>
</thead>
<tbody>
{{range .Apps}}
<tr>
  <td>
    <span class="name">{{.Name}}</span>
    {{if .Description}}<div>{{.Description}}</div>{{end}}
    {{if .Reference}}<div class="ref">{{.Reference}}</div>{{end}}
  </td>
  {{if .RunURL}}<td class="dl"><a class="run" href="{{.RunURL}}">Run &rarr;</a></td>
  {{else}}<td class="dl none">&mdash;</td>{{end}}
  {{range .Downloads}}
    {{if .Present}}<td class="dl"><a href="{{.URL}}">Download</a><span class="size">{{.Size}}</span></td>
    {{else}}<td class="dl none">&mdash;</td>{{end}}
  {{end}}
</tr>
{{end}}
</tbody>
</table>
</div>
{{end}}

<div class="notes">
<h2>Running the programs</h2>
<ul>
  <li><strong>macOS.</strong> The <code>.app</code> bundles are <em>not</em> signed or notarised.
      Right-click the app and choose <em>Open</em> the first time, or clear the quarantine flag:
      <code>xattr -dr com.apple.quarantine &lt;AppName&gt;.app</code></li>
  <li><strong>Linux.</strong> If the binary will not start, restore the executable bit:
      <code>chmod +x &lt;program&gt;</code></li>
  <li><strong>Windows.</strong> SmartScreen may warn about an unrecognised publisher; choose
      <em>More info</em> &rarr; <em>Run anyway</em>.</li>
  <li>Most programs accept <code>-w</code> (windowed), <code>-d N</code> (display index) and
      <code>-s &lt;id&gt;</code> (subject ID). No SDL installation is needed &mdash; it is bundled
      inside each binary.</li>
</ul>
<p class="muted">Only the most recent build is kept here. Source, documentation and the
full release history:
<a href="https://github.com/chrplr/goxpyriment">github.com/chrplr/goxpyriment</a>.</p>
</div>

</main>
</body>
</html>
`))

// redirectTmpl renders the small forwarding page for builds/index.html and
// builds/latest/index.html. It costs ~2 KB instead of duplicating the build.
var redirectTmpl = template.Must(template.New("redirect").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="refresh" content="0; url=../{{.SHA}}/index.html">
<link rel="canonical" href="https://downloads.pallier.org/builds/{{.SHA}}/index.html">
<title>goxpyriment downloads</title>
<style>` + pageCSS + `</style>
</head>
<body>
<main class="center">
<h1>goxpyriment downloads</h1>
<p>Redirecting to the latest build, <code>{{.ShortSHA}}</code>&hellip;</p>
<p><a href="../{{.SHA}}/index.html">Continue to the download page</a></p>
</main>
</body>
</html>
`))
