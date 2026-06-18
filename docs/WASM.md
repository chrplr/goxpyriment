# Running goxpyriment experiments in a web browser (WASM)

Hopefuly, one day, goxpyiment experiments will be runnable in a browser.
This is a work in progress at the moment.

This document records our attemps to (unsuccessfuly) implement this functionality in early 2026.

The setup requires two WASM runtimes in the same page:

1. **SDL3.wasm** — SDL3 compiled by Emscripten, which provides the graphics, audio, and event-loop APIs.
2. **main.wasm** — the Go experiment binary compiled with `GOOS=js GOARCH=wasm`.

## Timing caveat

The browser's `requestAnimationFrame` callback delivers frames at ~16.7 ms intervals with ±1–3 ms jitter. The hardware-locked VSYNC timing available on desktop (DRM ioctl, CoreVideo) is not available in a browser context. Experiments that require sub-millisecond stimulus onset precision (rapid RSVP, subliminal priming) should be run natively. Cognitive tasks (choice RT, memory, attention) work fine.

---

## Step 1 — Install Emscripten

```bash
git clone https://github.com/emscripten-core/emsdk.git ~/git_stuff/emsdk
cd ~/git_stuff/emsdk
./emsdk install latest
./emsdk activate latest
source ./emsdk_env.sh   # add to ~/.bashrc or ~/.zshrc for persistence
```

---

## Step 2 — Build SDL3 with Emscripten

```bash
cd ~/git_stuff

git clone https://github.com/libsdl-org/SDL.git -b release-3.2.x SDL3-src

emcmake cmake -S SDL3-src -B SDL3-wasm \
  -DCMAKE_BUILD_TYPE=Release \
  -DSDL_SHARED=OFF -DSDL_STATIC=ON \
  -DSDL_TESTS=OFF -DSDL_EXAMPLES=OFF

emmake make -C SDL3-wasm -j$(nproc)

# Install to a local prefix so SDL3_ttf can find it
cmake --install SDL3-wasm --prefix ~/git_stuff/emscripten-prefix
```

`cmake --install` just copies headers and the static lib — it does not execute
any WASM code, so it is safe for cross-compiled builds.

---

## Step 3 — Build FreeType with Emscripten (SDL3_ttf dependency)

```bash
cd ~/git_stuff

git clone https://gitlab.freedesktop.org/freetype/freetype.git freetype-src

emcmake cmake -S freetype-src -B freetype-wasm \
  -DCMAKE_BUILD_TYPE=Release

emmake make -C freetype-wasm -j$(nproc)

cmake --install freetype-wasm --prefix ~/git_stuff/emscripten-prefix
```

---

## Step 4 — Build SDL3_ttf with Emscripten

```bash
cd ~/git_stuff

git clone https://github.com/libsdl-org/SDL_ttf.git -b release-3.x SDL3_ttf-src

# Check that SDL3_ttf's required SDL3 version matches what you built:
grep "find_package(SDL3" SDL3_ttf-src/CMakeLists.txt | head -3
grep "SDL_MAJOR_VERSION\|SDL_MINOR_VERSION\|SDL_MICRO_VERSION" \
  SDL3-src/include/SDL3/SDL_version.h | head -3
# If there is a version mismatch, re-clone SDL3 from `main` (or the exact tag
# SDL3_ttf requires) and redo Step 2.

emcmake cmake -S SDL3_ttf-src -B SDL3_ttf-wasm \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_PREFIX_PATH=~/git_stuff/emscripten-prefix \
  -DSDL3TTF_HARFBUZZ=OFF \
  -DSDL3TTF_SAMPLES=OFF

emmake make -C SDL3_ttf-wasm -j$(nproc)

cmake --install SDL3_ttf-wasm --prefix ~/git_stuff/emscripten-prefix
```

Key fix vs the first attempt: use `-DCMAKE_PREFIX_PATH` pointing to the installed
prefix (where `SDL3Config.cmake` lives) rather than `-DSDL3_DIR` pointing at the
raw build directory.

---

## Step 5 — Link everything into a single SDL3.js + SDL3.wasm

All SDL3 and SDL3_ttf symbols must live in **one** Emscripten module because
`go-sdl3`'s WASM layer shares a single Emscripten heap (`HEAPU8`, `stackAlloc`, etc.).

```bash
cd ~/git_stuff

emcc -o SDL3.js \
  emscripten-prefix/lib/libSDL3.a \
  emscripten-prefix/lib/libSDL3_ttf.a \
  emscripten-prefix/lib/libfreetype.a \
  -s MODULARIZE=0 \
  -s ENVIRONMENT=web \
  -s ALLOW_TABLE_GROWTH=1 \
  -s EXPORTED_RUNTIME_METHODS='["HEAPU8","stackAlloc","stackSave","stackRestore","getValue","setValue","stringToUTF8OnStack","UTF8ToString","addFunction"]' \
  -s EXPORTED_FUNCTIONS='["_malloc","_free","_SDL_free","_SDL_GetError","_SDL_Init","_SDL_Quit","_SDL_CreateWindowAndRenderer","_SDL_DestroyWindow","_SDL_DestroyRenderer","_SDL_RenderPresent","_SDL_RenderClear","_SDL_SetRenderDrawColor","_SDL_RenderFillRect","_SDL_CreateTextureFromSurface","_SDL_DestroyTexture","_SDL_RenderTexture","_SDL_GetTicks","_SDL_GetTicksNS","_SDL_PollEvent","_SDL_GetWindowSize","_SDL_LoadBMP_IO","_SDL_GetCurrentVideoDriver","_SDL_GetVersion","_SDL_GetDisplays","_SDL_GetPrimaryDisplay","_SDL_GetDisplayForWindow","_SDL_GetCurrentDisplayMode","_SDL_PumpEvents","_SDL_SetCursor","_SDL_CreateSystemCursor","_SDL_GetMouseState","_TTF_Init","_TTF_Quit","_TTF_OpenFontIO","_TTF_CloseFont","_TTF_SetFontSize","_TTF_CreateRendererTextEngine","_TTF_CreateText","_TTF_SetTextColor","_TTF_SetTextColorFloat","_TTF_DrawRendererText","_TTF_RenderText_Solid","_TTF_RenderText_Blended","_TTF_GetTextSize"]'
```

> **Note:** This exported-functions list covers `hello_world`. More complex examples
> that load images or use additional SDL features will need extra entries. If you
> get a runtime error like `Module._SDL_Foo is not a function`, add `"_SDL_Foo"` to
> the list and relink.

This produces `SDL3.js` and `SDL3.wasm` in `~/git_stuff/`.

---

## Step 6 — Build the Go WASM binary

From the goxpyriment repo root:

```bash
make wasm-hello_world
# Produces: examples/hello_world/main.wasm and examples/hello_world/wasm_exec.js
```

Or manually:

```bash
GOOS=js GOARCH=wasm go build -o examples/hello_world/main.wasm ./examples/hello_world/
cp $(go env GOROOT)/lib/wasm/wasm_exec.js examples/hello_world/wasm_exec.js
```

---

## Step 7 — Serve locally

```bash
cp ~/git_stuff/SDL3.js  examples/hello_world/
cp ~/git_stuff/SDL3.wasm examples/hello_world/

make wasm-hello_world-serve
# Opens http://localhost:8080
```

WASM cannot be loaded from `file://` URLs due to browser security policy.
`python3 -m http.server 8080` is sufficient for local testing.

---

## Troubleshooting

**`Module._SDL_Foo is not a function`** — that function is missing from
`EXPORTED_FUNCTIONS` in Step 5. Add it and relink.

**`runtimeInitialized` never becomes true** — `SDL3.js` did not finish loading
before `main.wasm` started. Check that `<script src="SDL3.js">` appears before
`<script src="wasm_exec.js">` in `index.html`.

**Version mismatch between SDL3 and SDL3_ttf** — SDL3_ttf's `CMakeLists.txt`
specifies a minimum SDL3 version. Clone SDL3 from `main` (or the exact tag
SDL3_ttf requires) and redo Steps 2–4.

**Black/blank canvas** — SDL3's Emscripten backend renders to a `<canvas>` element.
Make sure `index.html` contains a `<canvas>` tag and that it is not hidden by CSS.

---

## What was changed in goxpyriment to support WASM

### Vendor patches (`vendor/github.com/Zyko0/go-sdl3/`)

| File | Change |
|------|--------|
| `sdl/sdl_functions_js.go` | `iDelay`, `iDelayNS` → no-op; `iShowCursor`, `iHideCursor`, `iSetWindowFullscreen` → no-op returning true |
| `ttf/ttf_functions_js.go` | `iCloseFont`, `iSetFontSize`, `iQuit` → panic removed |

These patches are lost if you run `go mod vendor`. Re-apply them manually.

### Library changes (`apparatus/`)

`screen.go` was split into three files:

| File | Build tag | Contents |
|------|-----------|----------|
| `screen.go` | *(none)* | Type definitions, Screen struct, all Screen methods |
| `screen_newscreen_notjs.go` | `//go:build !js` | Desktop `NewScreen`, `displayByIndex`, `ListDisplays` |
| `screen_newscreen_js.go` | `//go:build js` | WASM `NewScreen` using `sdl.CreateWindowAndRenderer` |

The WASM `NewScreen` ignores fullscreen/multi-monitor and defaults to 1024×768.
