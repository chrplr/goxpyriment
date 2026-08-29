// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import "html/template"

// launcherTmpl renders the standard browser launcher.
//
// It is adapted from examples/Memory_span/web/index.html. Several details in
// here look arbitrary and are not; the comments say which, because getting any
// of them wrong produces a page that loads and then misbehaves in a way that is
// hard to attribute.
var launcherTmpl = template.Must(template.New("launcher").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.App}} — goxpyriment</title>
<style>
  :root {
    --bg: #1a1a1a; --panel: #242424; --fg: #e8e8e8;
    --muted: #9a9a9a; --accent: #4a9eff; --border: #3a3a3a;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0; min-height: 100vh; display: flex;
    align-items: center; justify-content: center;
    background: var(--bg); color: var(--fg);
    font: 16px/1.6 system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  }
  #launcher { max-width: 40rem; padding: 2rem 1.5rem; }
  h1 { margin: 0 0 .3rem; font-size: 1.6rem; }
  h2 { font-size: 1rem; margin: 1.8rem 0 .4rem; color: var(--muted); }
  p { margin: .6rem 0; }
  .sub { color: var(--muted); }
  .ref { color: var(--muted); font-style: italic; font-size: .9rem; }
  a { color: var(--accent); }
  code, kbd { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: .87em; }
  kbd { border: 1px solid var(--border); border-radius: 3px; padding: 0 .3em; }
  ul { padding-left: 1.2rem; }
  li { margin-bottom: .45rem; }
  .field { display: flex; gap: .6rem; align-items: center; margin: 1.5rem 0 .5rem; flex-wrap: wrap; }
  label { color: var(--muted); }
  input {
    background: var(--panel); color: var(--fg); border: 1px solid var(--border);
    border-radius: 4px; padding: .45rem .6rem; width: 6rem; font: inherit;
  }
  button {
    background: var(--accent); color: #06121f; border: 0; border-radius: 4px;
    padding: .5rem 1.4rem; font: inherit; font-weight: 600; cursor: pointer;
  }
  button:disabled { background: var(--border); color: var(--muted); cursor: default; }
  #status { color: var(--muted); font-size: .9rem; min-height: 1.4em; }
  #status.error { color: #ff6b6b; }
  .warn {
    margin-top: 1.2rem; padding: .7rem .9rem; border-radius: 4px;
    background: #3a2f1a; border: 1px solid #5a4a26; font-size: .9rem;
  }
  #focusHint {
    position: fixed; top: 0; left: 0; right: 0; padding: .5rem;
    text-align: center; background: var(--accent); color: #06121f;
    font-weight: 600; display: none; z-index: 10;
  }
  body.running #focusHint.show { display: block; }

  /* The canvas must be laid out before SDL creates its window: the web backend
     takes the window size from the canvas CSS box, and a display:none element
     has none. Hiding it with visibility keeps the box. */
  #canvas { display: block; outline: none; }
  body:not(.running) #canvas { position: absolute; visibility: hidden; }
  body.running #launcher { display: none; }
  body.running { overflow: hidden; }
  /* Scale to fit with a uniform transform only. object-fit: contain breaks
     Emscripten's getBoundingClientRect pointer mapping, so clicks land in the
     wrong place. */
  body.running #canvas { transform: scale(var(--fit, 1)); }
</style>
</head>
<body>
<div id="focusHint">Click here to give the experiment keyboard focus</div>

<div id="launcher">
  <h1>{{.App}}</h1>
  {{if .Description}}<p class="sub">{{.Description}}</p>{{end}}
  {{if .Reference}}<p class="ref">{{.Reference}}</p>{{end}}
  <p class="sub">Built with
    <a href="https://github.com/chrplr/goxpyriment">goxpyriment</a>,
    running in your browser via WebAssembly.</p>

  <div class="field">
    <label for="subject">Participant ID</label>
    <input id="subject" type="text" inputmode="numeric" value="1" autocomplete="off" />
    <button id="start" disabled>Loading&hellip;</button>
  </div>

  <div id="status">Loading the SDL runtime&hellip;</div>

  <h2>Before you start</h2>
  <ul>
    <li>Use a <strong>focused, foreground tab</strong>: browsers throttle
      background tabs, and frame timing degrades when they do.</li>
    <li>Pressing Start also unlocks audio — browsers keep sound suspended until
      the first user gesture.</li>
    <li>When the session ends, your browser downloads <strong>one</strong>
      <code>.zip</code> file holding the <code>.csv</code> of results and the
      matching <code>-info.txt</code> of session metadata. If your browser asks
      where to save it, say yes — that archive is the whole session.</li>
    <li>Press <kbd>ESC</kbd> at any time to abort.</li>
  </ul>
</div>

<canvas id="canvas" tabindex="0" oncontextmenu="event.preventDefault()"></canvas>

<script>
  const canvasElement = document.getElementById('canvas');
  const startButton = document.getElementById('start');
  const subjectField = document.getElementById('subject');
  const statusLine = document.getElementById('status');
  const launcher = document.getElementById('launcher');
  const focusHint = document.getElementById('focusHint');

  function setStatus(text, isError) {
    statusLine.textContent = text;
    statusLine.classList.toggle('error', !!isError);
  }

  const initialSubject = new URLSearchParams(location.search).get('s');
  if (initialSubject) subjectField.value = initialSubject;

  // Cross-origin isolation is what lets the browser expose its full timer
  // resolution, which SDL timestamps — and therefore reaction times — inherit:
  // ~5 us isolated, ~100 us otherwise. Say so rather than silently recording
  // coarser RTs.
  if (!window.crossOriginIsolated) {
    const warning = document.createElement('div');
    warning.className = 'warn';
    warning.innerHTML = 'This page is <strong>not cross-origin isolated</strong>: ' +
      'timestamps tick at ~100&nbsp;µs instead of ~5&nbsp;µs. That is still far ' +
      'finer than any behavioural response, so the experiment is usable; it is ' +
      'the server that would need to send the ' +
      '<code>Cross-Origin-Opener-Policy: same-origin</code> and ' +
      '<code>Cross-Origin-Embedder-Policy: require-corp</code> headers.';
    launcher.appendChild(warning);
  }

  canvasElement.addEventListener('webglcontextlost', (e) => {
    e.preventDefault();
    setStatus('The WebGL context was lost. Reload the page to restart.', true);
  }, false);

  // Emscripten module hosting SDL3 (+ttf, +image, +mixer). MODULARIZE=0, so
  // ` + "`Module`" + ` is the global that sdl.js configures itself from.
  var Module = {
    // sdl.wasm is published once for the whole collection, not per app; the
    // Emscripten glue fetches it by name, so redirect it.
    locateFile(path) { return '{{.RuntimeDir}}' + path; },
    print(...args) { console.log(...args); },
    printErr(...args) { console.error(...args); },
    canvas: canvasElement,
    onRuntimeInitialized() { sdlReady = true; maybeEnableStart(); },
  };

  let sdlReady = false;
  let goReady = false;
  let goInstance = null;

  function maybeEnableStart() {
    if (!sdlReady || !goReady) return;
    startButton.disabled = false;
    startButton.textContent = 'Start';
    setStatus('Ready.');
  }
</script>

<script src="{{.RuntimeDir}}sdl.js"></script>
<script src="{{.RuntimeDir}}wasm_exec.js"></script>
<script>
  if (!WebAssembly.instantiateStreaming) {
    WebAssembly.instantiateStreaming = async (resp, importObject) => {
      const source = await (await resp).arrayBuffer();
      return await WebAssembly.instantiate(source, importObject);
    };
  }

  const go = new Go();
  WebAssembly.instantiateStreaming(fetch('main.wasm'), go.importObject)
    .then((result) => { goInstance = result.instance; goReady = true; maybeEnableStart(); })
    .catch((err) => {
      console.error(err);
      setStatus('Could not load the experiment: ' + err, true);
    });

  startButton.addEventListener('click', () => {
    startButton.disabled = true;

    // The Go side reads its flags from location.search at startup
    // (control.platformPrepareFlags), so the participant ID has to be in the
    // query string before it runs. replaceState changes it without a reload.
    const params = new URLSearchParams(location.search);
    params.set('s', subjectField.value.trim() || '0');
    history.replaceState(null, '', location.pathname + '?' + params.toString());

    document.body.classList.add('running');
    canvasElement.focus();
    void canvasElement.offsetWidth; // force layout before SDL measures the canvas
    fitCanvas();

    // Run on a fresh task, not inside this click handler: Go's wasm scheduler
    // can only park a blocked main goroutine when it is not nested in another
    // callback's stack.
    setTimeout(() => go.run(goInstance), 0);
  });

  function fitCanvas() {
    if (!document.body.classList.contains('running')) return;
    const w = canvasElement.width || canvasElement.clientWidth;
    const h = canvasElement.height || canvasElement.clientHeight;
    if (!w || !h) return;
    const fit = Math.min(1, window.innerWidth / w, window.innerHeight / h);
    document.documentElement.style.setProperty('--fit', fit);
  }
  window.addEventListener('resize', fitCanvas);
  setInterval(fitCanvas, 500); // the canvas is resized by SDL, which fires no event

  // Keys only reach SDL while the canvas has focus. Losing it — an alt-tab, a
  // stray click — otherwise looks exactly like the experiment having hung.
  function trackFocus() {
    const running = document.body.classList.contains('running');
    focusHint.classList.toggle('show', running && document.activeElement !== canvasElement);
  }
  window.addEventListener('blur', trackFocus);
  window.addEventListener('focus', trackFocus);
  focusHint.addEventListener('click', () => { canvasElement.focus(); trackFocus(); });
  setInterval(trackFocus, 500);

  subjectField.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !startButton.disabled) startButton.click();
  });
</script>
</body>
</html>
`))
