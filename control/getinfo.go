// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package control

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/bin/binttf"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
	"github.com/chrplr/goxpyriment/assets_embed"
)

// FieldType distinguishes between a text input and a checkbox field.
type FieldType int

const (
	FieldText     FieldType = iota // rendered as a text input box
	FieldCheckbox                  // rendered as a tick-box; value is "true" or "false"
)

// InfoField describes one entry in the GetParticipantInfo dialog.
type InfoField struct {
	Name    string    // key returned in the result map
	Label   string    // human-readable label displayed next to the field
	Default string    // initial value; use "true"/"false" for FieldCheckbox
	Type    FieldType // FieldText (default) or FieldCheckbox
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
		{Name: "screen_width_cm", Label: "Screen width (cm)", Default: "53"},
		{Name: "viewing_distance_cm", Label: "Viewing distance (cm)", Default: "57"},
		{Name: "refresh_rate_hz", Label: "Refresh rate (Hz)", Default: "60"},
	}

	// FullscreenField adds a fullscreen / windowed toggle.
	// When unchecked, the experiment should open a 1024×768 windowed screen.
	FullscreenField = InfoField{
		Name:    "fullscreen",
		Label:   "Fullscreen mode",
		Default: "true",
		Type:    FieldCheckbox,
	}

	// StandardFields is ParticipantFields + MonitorFields.
	StandardFields = append(append([]InfoField{}, ParticipantFields...), MonitorFields...)
)

// ErrCancelled is returned by GetParticipantInfo when the user closes or
// cancels the dialog without confirming.
var ErrCancelled = errors.New("info dialog cancelled")

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
// Returns ErrCancelled if the user presses Escape, clicks Cancel, or closes
// the window without confirming.
func GetParticipantInfo(title string, fields []InfoField) (map[string]string, error) {
	sdlLoader := binsdl.Load()
	defer sdlLoader.Unload()
	ttfLoader := binttf.Load()
	defer ttfLoader.Unload()

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
	var textIdx  []int // positions in fields where Type == FieldText
	var checkIdx []int // positions in fields where Type == FieldCheckbox
	for i, f := range fields {
		if f.Type == FieldCheckbox {
			checkIdx = append(checkIdx, i)
		} else {
			textIdx = append(textIdx, i)
		}
	}

	// ── Geometry ─────────────────────────────────────────────────────────────
	const (
		winW      = 620
		margin    = 30
		boxW      = winW - 2*margin
		boxH      = 28
		rowH      = 58 // label + box + gap per text field
		labelH    = 20 // approximate text height at 18 pt
		checkRowH = 32 // height per checkbox row
		headerH   = 58 // title + separator
		footerH   = 65 // OK / Cancel strip
	)

	winH := headerH + len(textIdx)*rowH + len(checkIdx)*checkRowH + footerH

	window, renderer, err := sdl.CreateWindowAndRenderer(title, winW, winH, 0)
	if err != nil {
		return nil, err
	}
	defer window.Destroy()
	defer renderer.Destroy()

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

	// cbY returns the Y of the checkbox for the ci-th checkbox field.
	cbY := func(ci int) float32 {
		return float32(headerH + len(textIdx)*rowH + ci*checkRowH + 6)
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

				// OK button.
				if mx >= okBtn.X && mx <= okBtn.X+okBtn.W &&
					my >= okBtn.Y && my <= okBtn.Y+okBtn.H {
					saveInfoCache(values, fields)
					return values, nil
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
				}

			case sdl.EVENT_KEY_DOWN:
				ke := ev.KeyboardEvent()
				switch ke.Key {
				case sdl.K_ESCAPE:
					return nil, ErrCancelled

				case sdl.K_RETURN, sdl.K_KP_ENTER:
					saveInfoCache(values, fields)
					return values, nil

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
			if focusTI == ti {
				renderer.SetDrawColor(colFocus.R, colFocus.G, colFocus.B, colFocus.A)
			} else {
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
