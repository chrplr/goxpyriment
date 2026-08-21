// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
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

// participantInfoCollected records whether GetParticipantInfo has been called in
// this process. NewExperimentFromFlags consults it so its automatic session-setup
// dialog never opens on top of a program's own explicit GetParticipantInfo call.
var participantInfoCollected bool

// stepFocus returns the field index delta positions along tabOrder from focus,
// wrapping at either end. An unknown focus (nothing focused yet) starts from
// the first entry, so the first TAB lands on the second widget and the first
// Shift-TAB on the last -- what a form does everywhere else.
func stepFocus(tabOrder []int, focus, delta int) int {
	if len(tabOrder) == 0 {
		return -1
	}
	at := 0
	for i, fi := range tabOrder {
		if fi == focus {
			at = i
			break
		}
	}
	return tabOrder[(at+delta+len(tabOrder))%len(tabOrder)]
}

// cycleValue returns what a widget's value becomes when the keyboard operates
// it: the option delta places along a select, wrapping, or the other state of
// a checkbox. Anything else -- a text field -- keeps its value, since there
// the arrow keys and the space bar belong to the text being typed.
//
// A select whose current value is not one of its options starts from the
// first, so a stale cached value cannot leave the arrows doing nothing.
func cycleValue(f InfoField, current string, delta int) string {
	switch f.Type {
	case FieldSelect:
		if len(f.Options) == 0 {
			return current
		}
		at := 0
		for i, opt := range f.Options {
			if current == opt {
				at = i
				break
			}
		}
		return f.Options[(at+delta+len(f.Options))%len(f.Options)]
	case FieldCheckbox:
		if current == "true" {
			return "false"
		}
		return "true"
	}
	return current
}

// dialogPresentation maps the fixed-size participant dialog onto a rendering
// output that is not necessarily the size that was asked for. Where there is no
// window manager -- a KMSDRM console, an SDL backend that only does fullscreen
// -- the window covers the whole display, and stretching the dialog across it
// applies a different factor horizontally and vertically, distorting every
// glyph.
//
// The dialog is therefore shown at its natural size and only ever scaled down.
// The returned logical size is the output divided by the scale wanted, not the
// dialog's own size: at scale 1 one logical unit is one output pixel and the
// dialog draws 1:1, and the viewport centres it. displayScale is the target
// scale, so a HiDPI panel still shows the dialog at its intended physical size;
// an output too small for the dialog gets a uniform reduction, which
// LOGICAL_PRESENTATION_LETTERBOX then applies without changing the aspect
// ratio. fillsOutput reports whether the dialog covers the whole output, i.e.
// whether anything of the surrounding area will be seen.
func dialogPresentation(outW, outH, dialogW, dialogH int32, displayScale float32) (logicalW, logicalH int32, viewport sdl.Rect, fillsOutput bool) {
	logicalW, logicalH = dialogW, dialogH
	viewport = sdl.Rect{W: dialogW, H: dialogH}
	if outW <= 0 || outH <= 0 || dialogW <= 0 || dialogH <= 0 {
		return logicalW, logicalH, viewport, true
	}
	if displayScale <= 0 {
		displayScale = 1
	}
	scale := min(displayScale,
		float32(outW)/float32(dialogW), float32(outH)/float32(dialogH))
	if scale <= 0 {
		return logicalW, logicalH, viewport, true
	}
	logicalW = int32(float32(outW) / scale)
	logicalH = int32(float32(outH) / scale)
	viewport = sdl.Rect{
		X: (logicalW - dialogW) / 2,
		Y: (logicalH - dialogH) / 2,
		W: dialogW,
		H: dialogH,
	}
	return logicalW, logicalH, viewport,
		viewport.X == 0 && viewport.Y == 0 && logicalW == dialogW && logicalH == dialogH
}

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
// The dialog is fully operable from the keyboard, since a console (a KMSDRM
// tty, a rig with no mouse on it) may have no pointer at all: TAB and the up
// and down arrows move between every widget, left and right change the focused
// select or checkbox, SPACE operates it, ENTER submits, ESCAPE cancels. The
// focused widget is outlined in blue.
//
// Returns ErrCancelled if the user presses Escape, clicks Cancel, or closes
// the window without confirming.
func GetParticipantInfo(title string, fields []InfoField) (map[string]string, error) {
	// Record that this program collects participant info explicitly, so
	// NewExperimentFromFlags does not also open its automatic setup dialog.
	participantInfoCollected = true

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
		return nil, fmt.Errorf("control.GetParticipantInfo: initializing SDL: %w", err)
	}
	defer sdl.Quit()

	if err := ttf.Init(); err != nil {
		return nil, fmt.Errorf("control.GetParticipantInfo: initializing TTF: %w", err)
	}
	defer ttf.Quit()

	font, err := FontFromMemory(assets_embed.InconsolataFont, 18)
	if err != nil {
		return nil, fmt.Errorf("control.GetParticipantInfo: loading font: %w", err)
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
	var textIdx []int   // positions in fields where Type == FieldText or FieldNumber
	var selectIdx []int // positions in fields where Type == FieldSelect
	var checkIdx []int  // positions in fields where Type == FieldCheckbox
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
		winW       = 620
		margin     = 30
		boxW       = winW - 2*margin
		boxH       = 28
		rowH       = 58 // label + box + gap per text field
		selectRowH = 58 // label + button row + gap per select field
		labelH     = 20 // approximate text height at 18 pt
		checkRowH  = 32 // height per checkbox row
		headerH    = 58 // title + separator
		footerH    = 65 // OK / Cancel strip
	)

	winH := headerH + len(textIdx)*rowH + len(selectIdx)*selectRowH + len(checkIdx)*checkRowH + footerH

	// SDL_WINDOW_HIGH_PIXEL_DENSITY + logical presentation keep the dialog
	// correct on HiDPI displays: SDL maps the fixed logical size to however
	// many physical pixels the display uses. Coordinates remain in the
	// logical [0,winW]×[0,winH] space throughout the event and draw code.
	window, renderer, err := sdl.CreateWindowAndRenderer(title, winW, winH, sdl.WINDOW_HIGH_PIXEL_DENSITY)
	if err != nil {
		return nil, fmt.Errorf("control.GetParticipantInfo: creating window: %w", err)
	}
	defer window.Destroy()
	defer renderer.Destroy()

	// Present the dialog at its natural size, scaling it down only when it
	// does not fit. The output is not always the size that was asked for: with
	// no window manager -- a KMSDRM console, an SDL fullscreen backend -- the
	// window covers the whole display, and stretching a 620-wide dialog across
	// it applies a different factor horizontally and vertically, which distorts
	// every glyph.
	//
	// So the logical size is the output divided by the scale we want rather
	// than the dialog's own size: at scale 1 one logical unit is one pixel and
	// the dialog draws 1:1, and the viewport then centres it. The display's own
	// scale factor is the target, which is what keeps a HiDPI panel showing the
	// dialog at its intended physical size; a screen too small for it takes a
	// uniform LETTERBOX reduction instead.
	outW, outH, sizeErr := renderer.CurrentOutputSize()
	if sizeErr != nil {
		outW, outH = int32(winW), int32(winH) // fall back to what was asked for
	}
	displayScale := float32(1)
	if ds, dsErr := window.DisplayScale(); dsErr == nil && ds > 0 {
		displayScale = ds
	}
	logicalW, logicalH, viewport, fillsOutput :=
		dialogPresentation(outW, outH, int32(winW), int32(winH), displayScale)
	renderer.SetLogicalPresentation(logicalW, logicalH, sdl.LOGICAL_PRESENTATION_LETTERBOX)
	// Drawing is clipped and translated to the dialog, so every coordinate in
	// the event and draw code below stays in the logical [0,winW]×[0,winH]
	// space it was written for, wherever the dialog ends up on screen.
	renderer.SetViewport(&viewport)

	window.StartTextInput()
	defer window.StopTextInput()

	// The dialog is clicked, so it needs a pointer regardless of what the
	// experiment did to cursor visibility. Initialize() hides the cursor by
	// default, and although this function owns its own SDL lifecycle (so it
	// normally runs before any of that), it can also be called mid-session
	// between blocks. Show the cursor for the dialog's lifetime and put it back
	// the way it was on the way out.
	cursorWasVisible := sdl.CursorVisible()
	// On a bare TTY (KMS/DRM) there is no compositor to supply a cursor shape,
	// and SDL draws one only if it has been given one -- ShowCursor on its own
	// makes "nothing" visible, which is indistinguishable from having no mouse
	// at all. Giving SDL a shape is what apparatus.NewScreen does for the same
	// reason; the dialog opens before any Screen exists, so it has to do it too.
	if cursor, curErr := sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_DEFAULT); curErr == nil {
		_ = sdl.SetCursor(cursor)
	} else {
		log.Printf("control.GetParticipantInfo: could not create a cursor shape: %v", curErr)
	}
	if err := sdl.ShowCursor(); err != nil {
		log.Printf("control.GetParticipantInfo: could not show the mouse cursor: %v", err)
	}
	defer func() {
		if !cursorWasVisible {
			_ = sdl.HideCursor()
		}
	}()

	// ── Colours ───────────────────────────────────────────────────────────────
	colBg := sdl.Color{R: 245, G: 245, B: 245, A: 255}
	colBlack := sdl.Color{R: 0, G: 0, B: 0, A: 255}
	colWhite := sdl.Color{R: 255, G: 255, B: 255, A: 255}
	colFocus := sdl.Color{R: 0, G: 100, B: 220, A: 255}
	colBorder := sdl.Color{R: 180, G: 180, B: 180, A: 255}
	colBackdrop := sdl.Color{R: 60, G: 60, B: 65, A: 255} // around the dialog on a larger screen
	colGreen := sdl.Color{R: 0, G: 140, B: 0, A: 255}
	colRed := sdl.Color{R: 180, G: 0, B: 0, A: 255}
	colCheck := sdl.Color{R: 0, G: 150, B: 0, A: 255}

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

	// Every widget is reachable from the keyboard, in the order it is drawn.
	// A console has no pointer -- a KMSDRM tty, a testing rig without a mouse
	// on it -- and a dialog whose checkboxes and option rows can only be
	// clicked cannot be filled in at all there.
	tabOrder := make([]int, 0, len(fields))
	tabOrder = append(tabOrder, textIdx...)
	tabOrder = append(tabOrder, selectIdx...)
	tabOrder = append(tabOrder, checkIdx...)

	// focus indexes fields, not one of the per-type slices, so it can name any
	// widget. -1 is nothing focused.
	focus := -1
	if len(tabOrder) > 0 {
		focus = tabOrder[0]
	}
	moveFocus := func(delta int) { focus = stepFocus(tabOrder, focus, delta) }
	// operate applies the keyboard to the focused widget: the next or previous
	// option of a select, the other state of a checkbox.
	operate := func(delta int) {
		if focus >= 0 {
			f := fields[focus]
			values[f.Name] = cycleValue(f, values[f.Name], delta)
		}
	}
	// editingText reports whether typing goes into the focused widget.
	editingText := func() bool {
		return focus >= 0 &&
			(fields[focus].Type == FieldText || fields[focus].Type == FieldNumber)
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
				// Window coordinates are not the logical ones the widgets are
				// laid out in as soon as the output size differs from the
				// dialog size (a KMSDRM console, a HiDPI panel). SDL applies
				// the logical presentation, the scale and the viewport for us.
				mx, my := me.X, me.Y
				if rx, ry, convErr := renderer.RenderCoordinatesFromWindow(me.X, me.Y); convErr == nil {
					mx, my = rx, ry
				}

				// Click on a text field → focus it.
				focus = -1
				for ti, fi := range textIdx {
					y := boxY(ti)
					if mx >= float32(margin) && mx <= float32(margin+boxW) &&
						my >= y && my <= y+boxH {
						focus = fi
						break
					}
				}

				// Click on a checkbox → toggle it.
				for ci, fi := range checkIdx {
					y := cbY(ci)
					if mx >= float32(margin) && mx <= float32(margin+300) &&
						my >= y && my <= y+float32(checkRowH) {
						focus = fi
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
							focus = fi
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
				if editingText() {
					values[fields[focus].Name] += ev.TextInputEvent().Text
					delete(invalidFields, fields[focus].Name)
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
					if editingText() {
						s := values[fields[focus].Name]
						if len(s) > 0 {
							_, size := utf8.DecodeLastRuneInString(s)
							values[fields[focus].Name] = s[:len(s)-size]
						}
					}

				case sdl.K_TAB:
					if ke.Mod&sdl.KMOD_SHIFT != 0 {
						moveFocus(-1)
					} else {
						moveFocus(1)
					}

				case sdl.K_DOWN:
					moveFocus(1)

				case sdl.K_UP:
					moveFocus(-1)

				case sdl.K_RIGHT:
					operate(1)

				case sdl.K_LEFT:
					operate(-1)

				case sdl.K_SPACE:
					// A space belongs to the text being typed; anywhere else it
					// is the usual "operate this widget" key. EVENT_TEXT_INPUT
					// delivers the character itself, so this only has to keep
					// out of the way.
					if !editingText() {
						operate(1)
					}
				}
			}
		}

		// ── Draw ─────────────────────────────────────────────────────────────

		// Clear ignores the viewport, so this paints the whole output: the
		// backdrop when the dialog is smaller than the screen, and the dialog's
		// own background when it is not.
		if fillsOutput {
			renderer.SetDrawColor(colBg.R, colBg.G, colBg.B, colBg.A)
		} else {
			renderer.SetDrawColor(colBackdrop.R, colBackdrop.G, colBackdrop.B, colBackdrop.A)
		}
		renderer.Clear()
		if !fillsOutput {
			panel := sdl.FRect{X: 0, Y: 0, W: winW, H: float32(winH)}
			renderer.SetDrawColor(colBg.R, colBg.G, colBg.B, colBg.A)
			renderer.RenderFillRect(&panel)
			renderer.SetDrawColor(colBorder.R, colBorder.G, colBorder.B, colBorder.A)
			renderer.RenderRect(&panel)
		}

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
			case focus == fi:
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
			if focus == fi {
				// The row has the focus; the filled button inside it is the
				// current choice. Two different things, so two different marks.
				ring := sdl.FRect{X: float32(margin) - 4, Y: y - 4,
					W: float32(boxW) + 8, H: boxH + 8}
				renderer.SetDrawColor(colFocus.R, colFocus.G, colFocus.B, colFocus.A)
				renderer.RenderRect(&ring)
			}
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
			if focus == fi {
				ring := sdl.FRect{X: float32(margin) - 4, Y: y - 4, W: cs + 8, H: cs + 8}
				renderer.SetDrawColor(colFocus.R, colFocus.G, colFocus.B, colFocus.A)
				renderer.RenderRect(&ring)
			}
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
		renderCentered("Go!", okBtn, colWhite)

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
