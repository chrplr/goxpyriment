// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package control

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
	"github.com/chrplr/goxpyriment/assets_embed"
)

// sharedSDLLoader and sharedTTFLoader hold SDL/TTF dylib handles loaded by
// GetParticipantInfo so that Initialize() can reuse them instead of loading a
// second copy. On macOS, loading the same dylib from two different temp paths
// causes duplicate Objective-C class registrations and a silent crash.
var (
	sharedSDLLoader interface{ Unload() }
	sharedTTFLoader interface{ Unload() }
)

// consumeSharedLoaders returns any SDL and TTF loader handles cached by
// GetParticipantInfo and resets the package-level cache. Returns nil, nil
// when GetParticipantInfo was not called beforehand.
func consumeSharedLoaders() (sdl, ttf interface{ Unload() }) {
	sdl, ttf = sharedSDLLoader, sharedTTFLoader
	sharedSDLLoader, sharedTTFLoader = nil, nil
	return
}

// FieldType distinguishes between a text input and a checkbox field.
type FieldType int

const (
	FieldText     FieldType = iota // rendered as a text input box
	FieldCheckbox                  // rendered as a tick-box; value is "true" or "false"
	FieldNumber                    // rendered as a text box; validated as a positive number on submit
	FieldSelect                    // rendered as a row of clickable option buttons; value is the selected option
)

// InfoField describes one entry in the GetParticipantInfo dialog.
type InfoField struct {
	Name    string    // key returned in the result map
	Label   string    // human-readable label displayed next to the field
	Default string    // initial value; use "true"/"false" for FieldCheckbox
	Type    FieldType // FieldText (default), FieldCheckbox, FieldNumber, or FieldSelect
	Options []string  // choices for FieldSelect; first entry used if Default is empty
}

// Pre-built field sets for common use cases.
var (
	// ParticipantFields collects basic demographics.
	ParticipantFields = []InfoField{
		{Name: "subject_id", Label: "Subject ID", Default: ""},
		{Name: "age", Label: "Age", Default: ""},
		{Name: "gender", Label: "Gender (M / F / NB)", Default: ""},
		{Name: "handedness", Label: "Handedness (R / L)", Default: "R"},
	}

	// MonitorFields collects display and viewing-setup characteristics.
	MonitorFields = []InfoField{
		{Name: "screen_width_cm", Label: "Screen width (cm)", Default: "30", Type: FieldNumber},
		{Name: "viewing_distance_cm", Label: "Viewing distance (cm)", Default: "50", Type: FieldNumber},
		{Name: "refresh_rate_hz", Label: "Refresh rate (Hz)", Default: "60", Type: FieldNumber},
	}

	// FullscreenField adds a fullscreen / windowed toggle.
	// When unchecked, the experiment should open a 1024×768 windowed screen.
	FullscreenField = InfoField{
		Name:    "fullscreen",
		Label:   "Fullscreen mode",
		Default: "true",
		Type:    FieldCheckbox,
	}

	// DisplayField lets the experimenter choose the monitor on which the
	// experiment window (or fullscreen) will open. 0 = primary display.
	// Use DisplayIDFromInfo to extract the integer value from the result map.
	// FieldText is used (not FieldNumber) because FieldNumber rejects 0.
	DisplayField = InfoField{
		Name:    "display_id",
		Label:   "Display ID (0 = primary monitor)",
		Default: "0",
		Type:    FieldText,
	}

	// StandardFields is ParticipantFields + MonitorFields.
	StandardFields = append(append([]InfoField{}, ParticipantFields...), MonitorFields...)
)

// ErrCancelled is returned by GetParticipantInfo when the user closes or
// cancels the dialog without confirming.
var ErrCancelled = errors.New("info dialog cancelled")

// headlessFlag skips the participant-info dialog when -headless is passed on
// the command line. GetParticipantInfo returns field defaults (plus any cached
// values from the last interactive session) without opening a window.
// Registered at package-init time so it is available before flag.Parse().
var headlessFlag = flag.Bool("headless", false, "skip the participant info dialog and use field defaults")

// GetParticipantInfo opens a graphical SDL dialog before the experiment starts,
// lets the experimenter fill in the provided fields, and returns the collected
// values as a map[field.Name → value].
//
// Call this before exp.Initialize(). The function initialises SDL internally
// and shuts it down cleanly before returning, so the subsequent Initialize()
// call starts from a fresh state.
//
// Previous session values are loaded from the user cache directory and
// pre-filled automatically. "subject_id" is always reset to its default.
// All other values are saved on OK.
//
// When the -headless flag is set, the dialog is skipped entirely: each field
// receives its cached value (or its Default if no cache entry exists).
//
// Returns ErrCancelled if the user presses Escape, clicks Cancel, or closes
// the window without confirming.
func GetParticipantInfo(title string, fields []InfoField) (map[string]string, error) {
	// Headless mode: return defaults (+ cache) without opening any window.
	// SDL is not loaded here; Initialize() will load it normally.
	if *headlessFlag {
		cache := loadInfoCache()
		values := make(map[string]string, len(fields))
		for _, f := range fields {
			if cached, ok := cache[f.Name]; ok && f.Name != "subject_id" {
				values[f.Name] = cached
			} else {
				values[f.Name] = f.Default
			}
		}
		// Ensure FieldSelect values are valid options.
		for _, f := range fields {
			if f.Type == FieldSelect && len(f.Options) > 0 {
				valid := false
				for _, opt := range f.Options {
					if values[f.Name] == opt {
						valid = true
						break
					}
				}
				if !valid {
					values[f.Name] = f.Options[0]
				}
			}
		}
		return values, nil
	}

	// Load SDL/TTF dylibs once and cache them for reuse by Initialize().
	// On macOS, loading two separate copies of the same dylib (from different
	// temp paths) registers duplicate Objective-C classes and causes a crash.
	if sharedSDLLoader == nil {
		sharedSDLLoader = loadSDL()
	}
	if sharedTTFLoader == nil {
		sharedTTFLoader = loadTTF()
	}

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		return nil, err
	}
	defer sdl.Quit()

	if err := ttf.Init(); err != nil {
		return nil, err
	}
	defer ttf.Quit()

	font, err := FontFromMemory(assets_embed.InconsolataFont, 18)
	if err != nil {
		return nil, err
	}
	defer font.Close()

	// Populate initial values from cache (subject_id always cleared).
	cache := loadInfoCache()
	values := make(map[string]string, len(fields))
	for _, f := range fields {
		if cached, ok := cache[f.Name]; ok && f.Name != "subject_id" {
			values[f.Name] = cached
		} else {
			values[f.Name] = f.Default
		}
	}

	// Split fields by type for layout and event handling.
	var textIdx   []int // positions in fields where Type == FieldText or FieldNumber
	var selectIdx []int // positions in fields where Type == FieldSelect
	var checkIdx  []int // positions in fields where Type == FieldCheckbox
	for i, f := range fields {
		switch f.Type {
		case FieldCheckbox:
			checkIdx = append(checkIdx, i)
		case FieldSelect:
			selectIdx = append(selectIdx, i)
		default:
			textIdx = append(textIdx, i)
		}
	}

	// For FieldSelect, ensure the initial value is one of the declared options.
	for _, fi := range selectIdx {
		f := fields[fi]
		if len(f.Options) == 0 {
			continue
		}
		valid := false
		for _, opt := range f.Options {
			if values[f.Name] == opt {
				valid = true
				break
			}
		}
		if !valid {
			values[f.Name] = f.Options[0]
		}
	}

	// ── Geometry ─────────────────────────────────────────────────────────────
	const (
		winW        = 620
		margin      = 30
		boxW        = winW - 2*margin
		boxH        = 28
		rowH        = 58 // label + box + gap per text field
		selectRowH  = 58 // label + button row + gap per select field
		labelH      = 20 // approximate text height at 18 pt
		checkRowH   = 32 // height per checkbox row
		headerH     = 58 // title + separator
		footerH     = 65 // OK / Cancel strip
	)

	winH := headerH + len(textIdx)*rowH + len(selectIdx)*selectRowH + len(checkIdx)*checkRowH + footerH

	// SDL_WINDOW_HIGH_PIXEL_DENSITY + logical presentation keep the dialog
	// correct on HiDPI displays: SDL maps the fixed logical size to however
	// many physical pixels the display uses. Coordinates remain in the
	// logical [0,winW]×[0,winH] space throughout the event and draw code.
	window, renderer, err := sdl.CreateWindowAndRenderer(title, winW, winH, sdl.WINDOW_HIGH_PIXEL_DENSITY)
	if err != nil {
		return nil, err
	}
	defer window.Destroy()
	defer renderer.Destroy()
	renderer.SetLogicalPresentation(int32(winW), int32(winH), sdl.LOGICAL_PRESENTATION_STRETCH)

	window.StartTextInput()
	defer window.StopTextInput()

	// ── Colours ───────────────────────────────────────────────────────────────
	colBg     := sdl.Color{R: 245, G: 245, B: 245, A: 255}
	colBlack  := sdl.Color{R: 0, G: 0, B: 0, A: 255}
	colWhite  := sdl.Color{R: 255, G: 255, B: 255, A: 255}
	colFocus  := sdl.Color{R: 0, G: 100, B: 220, A: 255}
	colBorder := sdl.Color{R: 180, G: 180, B: 180, A: 255}
	colGreen  := sdl.Color{R: 0, G: 140, B: 0, A: 255}
	colRed    := sdl.Color{R: 180, G: 0, B: 0, A: 255}
	colCheck  := sdl.Color{R: 0, G: 150, B: 0, A: 255}

	// ── Render helpers ────────────────────────────────────────────────────────

	renderText := func(text string, x, y float32, color sdl.Color) {
		if text == "" {
			return
		}
		surf, err := font.RenderTextBlended(text, color)
		if err != nil || surf == nil {
			return
		}
		defer surf.Destroy()
		tex, err := renderer.CreateTextureFromSurface(surf)
		if err != nil {
			return
		}
		defer tex.Destroy()
		renderer.RenderTexture(tex, nil, &sdl.FRect{
			X: x, Y: y, W: float32(surf.W), H: float32(surf.H),
		})
	}

	renderCentered := func(text string, rect sdl.FRect, color sdl.Color) {
		tw, th, err := font.StringSize(text)
		if err != nil {
			return
		}
		renderText(text,
			rect.X+(rect.W-float32(tw))/2,
			rect.Y+(rect.H-float32(th))/2,
			color)
	}

	// boxY returns the Y of the input box for the ti-th text field.
	boxY := func(ti int) float32 {
		return float32(headerH + ti*rowH + labelH + 4)
	}

	// selY returns the Y of the button row for the si-th select field.
	selY := func(si int) float32 {
		return float32(headerH + len(textIdx)*rowH + si*selectRowH + labelH + 4)
	}

	// cbY returns the Y of the checkbox for the ci-th checkbox field.
	cbY := func(ci int) float32 {
		return float32(headerH + len(textIdx)*rowH + len(selectIdx)*selectRowH + ci*checkRowH + 6)
	}

	okBtn := sdl.FRect{
		X: float32(winW/2 - 120), Y: float32(winH - footerH + 15), W: 100, H: 36,
	}
	cancelBtn := sdl.FRect{
		X: float32(winW/2 + 20), Y: float32(winH - footerH + 15), W: 100, H: 36,
	}

	// Start with the first text field focused, if any.
	focusTI := -1
	if len(textIdx) > 0 {
		focusTI = 0
	}

	// invalidFields tracks FieldNumber fields whose current value is not a
	// positive number. Populated on submit; cleared when the field is edited.
	invalidFields := map[string]bool{}

	// validateForm checks all FieldNumber fields and returns true if all pass.
	validateForm := func() bool {
		invalidFields = map[string]bool{}
		for _, f := range fields {
			if f.Type == FieldNumber {
				v := strings.TrimSpace(values[f.Name])
				n, err := strconv.ParseFloat(v, 64)
				if err != nil || n <= 0 {
					invalidFields[f.Name] = true
				}
			}
		}
		return len(invalidFields) == 0
	}

	// ── Event loop ────────────────────────────────────────────────────────────
	for {
		var ev sdl.Event
		for sdl.PollEvent(&ev) {
			switch ev.Type {
			case sdl.EVENT_QUIT:
				return nil, ErrCancelled

			case sdl.EVENT_MOUSE_BUTTON_DOWN:
				me := ev.MouseButtonEvent()
				mx, my := me.X, me.Y

				// Click on a text field → focus it.
				focusTI = -1
				for ti, fi := range textIdx {
					y := boxY(ti)
					if mx >= float32(margin) && mx <= float32(margin+boxW) &&
						my >= y && my <= y+boxH {
						focusTI = ti
						_ = fi
						break
					}
				}

				// Click on a checkbox → toggle it.
				for ci, fi := range checkIdx {
					y := cbY(ci)
					if mx >= float32(margin) && mx <= float32(margin+300) &&
						my >= y && my <= y+float32(checkRowH) {
						if values[fields[fi].Name] == "true" {
							values[fields[fi].Name] = "false"
						} else {
							values[fields[fi].Name] = "true"
						}
					}
				}

				// Click on a select option → select it.
				for si, fi := range selectIdx {
					y := selY(si)
					f := fields[fi]
					nOpts := len(f.Options)
					if nOpts == 0 {
						continue
					}
					btnW := float32(boxW-(nOpts-1)*4) / float32(nOpts)
					for oi, opt := range f.Options {
						bx := float32(margin) + float32(oi)*(btnW+4)
						if mx >= bx && mx <= bx+btnW && my >= y && my <= y+boxH {
							values[f.Name] = opt
						}
					}
				}

				// OK button.
				if mx >= okBtn.X && mx <= okBtn.X+okBtn.W &&
					my >= okBtn.Y && my <= okBtn.Y+okBtn.H {
					if validateForm() {
						saveInfoCache(values, fields)
						return values, nil
					}
				}

				// Cancel button.
				if mx >= cancelBtn.X && mx <= cancelBtn.X+cancelBtn.W &&
					my >= cancelBtn.Y && my <= cancelBtn.Y+cancelBtn.H {
					return nil, ErrCancelled
				}

			case sdl.EVENT_TEXT_INPUT:
				if focusTI >= 0 && focusTI < len(textIdx) {
					fi := textIdx[focusTI]
					values[fields[fi].Name] += ev.TextInputEvent().Text
					delete(invalidFields, fields[fi].Name)
				}

			case sdl.EVENT_KEY_DOWN:
				ke := ev.KeyboardEvent()
				switch ke.Key {
				case sdl.K_ESCAPE:
					return nil, ErrCancelled

				case sdl.K_RETURN, sdl.K_KP_ENTER:
					if validateForm() {
						saveInfoCache(values, fields)
						return values, nil
					}

				case sdl.K_BACKSPACE:
					if focusTI >= 0 && focusTI < len(textIdx) {
						fi := textIdx[focusTI]
						s := values[fields[fi].Name]
						if len(s) > 0 {
							_, size := utf8.DecodeLastRuneInString(s)
							values[fields[fi].Name] = s[:len(s)-size]
						}
					}

				case sdl.K_TAB:
					if len(textIdx) > 0 {
						if ke.Mod&sdl.KMOD_SHIFT != 0 {
							focusTI = (focusTI - 1 + len(textIdx)) % len(textIdx)
						} else {
							focusTI = (focusTI + 1) % len(textIdx)
						}
					}
				}
			}
		}

		// ── Draw ─────────────────────────────────────────────────────────────

		renderer.SetDrawColor(colBg.R, colBg.G, colBg.B, colBg.A)
		renderer.Clear()

		// Title
		renderText(title, float32(margin), 18, colBlack)

		// Separator below title
		renderer.SetDrawColor(colBorder.R, colBorder.G, colBorder.B, colBorder.A)
		renderer.RenderLine(
			float32(margin), float32(headerH-5),
			float32(winW-margin), float32(headerH-5),
		)

		// Text input fields
		for ti, fi := range textIdx {
			f := fields[fi]
			y := boxY(ti)
			val := values[f.Name]

			renderText(f.Label+":", float32(margin), y-float32(labelH)-2, colBlack)

			box := sdl.FRect{X: float32(margin), Y: y, W: float32(boxW), H: boxH}
			renderer.SetDrawColor(colWhite.R, colWhite.G, colWhite.B, colWhite.A)
			renderer.RenderFillRect(&box)
			switch {
			case invalidFields[f.Name]:
				renderer.SetDrawColor(colRed.R, colRed.G, colRed.B, colRed.A)
			case focusTI == ti:
				renderer.SetDrawColor(colFocus.R, colFocus.G, colFocus.B, colFocus.A)
			default:
				renderer.SetDrawColor(colBorder.R, colBorder.G, colBorder.B, colBorder.A)
			}
			renderer.RenderRect(&box)

			// Truncate display if the value is very long.
			display := val
			if len(display) > 60 {
				display = "…" + display[len(display)-59:]
			}
			renderText(display, float32(margin)+6, y+4, colBlack)
		}

		// Select fields
		for si, fi := range selectIdx {
			f := fields[fi]
			y := selY(si)
			nOpts := len(f.Options)
			renderText(f.Label+":", float32(margin), y-float32(labelH)-2, colBlack)
			if nOpts > 0 {
				btnW := float32(boxW-(nOpts-1)*4) / float32(nOpts)
				for oi, opt := range f.Options {
					bx := float32(margin) + float32(oi)*(btnW+4)
					btn := sdl.FRect{X: bx, Y: y, W: btnW, H: boxH}
					selected := values[f.Name] == opt
					if selected {
						renderer.SetDrawColor(colFocus.R, colFocus.G, colFocus.B, colFocus.A)
					} else {
						renderer.SetDrawColor(colWhite.R, colWhite.G, colWhite.B, colWhite.A)
					}
					renderer.RenderFillRect(&btn)
					renderer.SetDrawColor(colBorder.R, colBorder.G, colBorder.B, colBorder.A)
					renderer.RenderRect(&btn)
					tc := colBlack
					if selected {
						tc = colWhite
					}
					renderCentered(opt, btn, tc)
				}
			}
		}

		// Checkbox fields
		const cs float32 = 20 // checkbox square side length
		for ci, fi := range checkIdx {
			f := fields[fi]
			y := cbY(ci)
			checked := values[f.Name] == "true"

			box := sdl.FRect{X: float32(margin), Y: y, W: cs, H: cs}
			renderer.SetDrawColor(colWhite.R, colWhite.G, colWhite.B, colWhite.A)
			renderer.RenderFillRect(&box)
			renderer.SetDrawColor(colBlack.R, colBlack.G, colBlack.B, colBlack.A)
			renderer.RenderRect(&box)
			if checked {
				mark := sdl.FRect{X: float32(margin) + 4, Y: y + 4, W: cs - 8, H: cs - 8}
				renderer.SetDrawColor(colCheck.R, colCheck.G, colCheck.B, colCheck.A)
				renderer.RenderFillRect(&mark)
			}
			renderText(f.Label, float32(margin)+cs+10, y+1, colBlack)
		}

		// Separator above buttons
		renderer.SetDrawColor(colBorder.R, colBorder.G, colBorder.B, colBorder.A)
		renderer.RenderLine(
			float32(margin), float32(winH-footerH+5),
			float32(winW-margin), float32(winH-footerH+5),
		)

		// OK button (green)
		renderer.SetDrawColor(colGreen.R, colGreen.G, colGreen.B, colGreen.A)
		renderer.RenderFillRect(&okBtn)
		renderCentered("OK", okBtn, colWhite)

		// Cancel button (red)
		renderer.SetDrawColor(colRed.R, colRed.G, colRed.B, colRed.A)
		renderer.RenderFillRect(&cancelBtn)
		renderCentered("Cancel", cancelBtn, colWhite)

		renderer.Present()
		sdl.Delay(16)
	}
}

// DisplayIDFromInfo extracts the display_id value from a GetParticipantInfo
// result map (e.g. one that included DisplayField). It returns the integer
// monitor index, or 0 (primary display) if the key is absent or not a
// non-negative integer.
//
// Typical usage:
//
//	info, _ := control.GetParticipantInfo(title, append(fields, control.DisplayField))
//	exp.ScreenNumber = control.DisplayIDFromInfo(info)
func DisplayIDFromInfo(info map[string]string) int {
	v, ok := info["display_id"]
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// ─── Session cache ────────────────────────────────────────────────────────────

type infoCache struct {
	Fields map[string]string `json:"fields"`
}

func infoCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "goxpyriment", "last_session.json")
}

func loadInfoCache() map[string]string {
	path := infoCachePath()
	if path == "" {
		return map[string]string{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	var c infoCache
	if err := json.Unmarshal(data, &c); err != nil || c.Fields == nil {
		return map[string]string{}
	}
	return c.Fields
}

func saveInfoCache(values map[string]string, fields []InfoField) {
	path := infoCachePath()
	if path == "" {
		return
	}
	// Never persist subject_id — it must be entered fresh each session.
	toSave := make(map[string]string, len(values))
	for _, f := range fields {
		if f.Name != "subject_id" {
			toSave[f.Name] = values[f.Name]
		}
	}
	data, err := json.Marshal(infoCache{Fields: toSave})
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, data, 0o644)
}
