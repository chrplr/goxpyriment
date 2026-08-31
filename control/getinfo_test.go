// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import "testing"

// The dialog must never be stretched: whatever the output, the horizontal and
// vertical factors have to match, or the glyphs distort. That is what went
// wrong on a KMSDRM console, where the window covers the whole display.
func TestDialogPresentation(t *testing.T) {
	const dw, dh = 620, 400

	cases := []struct {
		name           string
		outW, outH     int32
		displayScale   float32
		wantLogical    [2]int32
		wantViewportXY [2]int32
		wantFills      bool
	}{
		{"window manager honours the request", 620, 400, 1,
			[2]int32{620, 400}, [2]int32{0, 0}, true},
		{"KMSDRM console, dialog centred at 1:1", 1920, 1080, 1,
			[2]int32{1920, 1080}, [2]int32{650, 340}, false},
		{"HiDPI panel keeps its 2x physical size", 1240, 800, 2,
			[2]int32{620, 400}, [2]int32{0, 0}, true},
		{"screen too small: uniform reduction", 400, 300, 1,
			[2]int32{620, 465}, [2]int32{0, 32}, false},
		{"output size unavailable", 0, 0, 1,
			[2]int32{620, 400}, [2]int32{0, 0}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lw, lh, vp, fills := dialogPresentation(c.outW, c.outH, dw, dh, c.displayScale)
			if lw != c.wantLogical[0] || lh != c.wantLogical[1] {
				t.Errorf("logical size: got %dx%d, want %dx%d",
					lw, lh, c.wantLogical[0], c.wantLogical[1])
			}
			if vp.X != c.wantViewportXY[0] || vp.Y != c.wantViewportXY[1] {
				t.Errorf("viewport origin: got (%d,%d), want (%d,%d)",
					vp.X, vp.Y, c.wantViewportXY[0], c.wantViewportXY[1])
			}
			if vp.W != dw || vp.H != dh {
				t.Errorf("viewport size: got %dx%d, want the dialog's %dx%d",
					vp.W, vp.H, dw, dh)
			}
			if fills != c.wantFills {
				t.Errorf("fillsOutput: got %v, want %v", fills, c.wantFills)
			}
			// The point of the exercise: one scale factor, not two.
			if c.outW > 0 && c.outH > 0 {
				sx := float64(c.outW) / float64(lw)
				sy := float64(c.outH) / float64(lh)
				if d := sx - sy; d > 0.005 || d < -0.005 {
					t.Errorf("anisotropic scaling: x %.4f vs y %.4f", sx, sy)
				}
				if sx > float64(c.displayScale)+0.005 {
					t.Errorf("upscaled past the display scale: %.4f > %v",
						sx, c.displayScale)
				}
			}
			// The dialog must sit inside the logical area, never clipped.
			if vp.X < 0 || vp.Y < 0 || vp.X+vp.W > lw || vp.Y+vp.H > lh {
				t.Errorf("dialog %v does not fit the logical %dx%d", vp, lw, lh)
			}
		})
	}
}

// Without a pointer -- a KMSDRM console, a rig with no mouse -- every widget
// has to be reachable and operable from the keyboard, so the two pieces of
// logic that make that work are checked here.
func TestStepFocus(t *testing.T) {
	// Three text fields (0,1,2), one select (3), one checkbox (4): the drawing
	// order, which is the order TAB follows.
	order := []int{0, 1, 2, 3, 4}

	for _, c := range []struct{ from, delta, want int }{
		{0, 1, 1}, {2, 1, 3}, {3, 1, 4}, // forwards, into the select and checkbox
		{4, 1, 0},              // wraps
		{0, -1, 4}, {3, -1, 2}, // backwards, and wraps
		{-1, 1, 1}, {-1, -1, 4}, // nothing focused yet
		{99, 1, 1}, // a stale index behaves like none
	} {
		if got := stepFocus(order, c.from, c.delta); got != c.want {
			t.Errorf("stepFocus(from=%d, delta=%d) = %d, want %d",
				c.from, c.delta, got, c.want)
		}
	}
	if got := stepFocus(nil, -1, 1); got != -1 {
		t.Errorf("stepFocus with no widgets = %d, want -1", got)
	}
}

func TestCycleValue(t *testing.T) {
	sel := InfoField{Name: "hand", Type: FieldSelect,
		Options: []string{"left", "right", "both"}}
	check := InfoField{Name: "fullscreen", Type: FieldCheckbox}
	text := InfoField{Name: "age", Type: FieldText}

	for _, c := range []struct {
		name    string
		f       InfoField
		current string
		delta   int
		want    string
	}{
		{"select forwards", sel, "left", 1, "right"},
		{"select wraps forwards", sel, "both", 1, "left"},
		{"select backwards", sel, "right", -1, "left"},
		{"select wraps backwards", sel, "left", -1, "both"},
		{"a stale value starts from the first", sel, "gone", 1, "right"},
		{"checkbox on", check, "false", 1, "true"},
		{"checkbox off", check, "true", 1, "false"},
		{"checkbox ignores direction", check, "true", -1, "false"},
		{"text is left to the typing", text, "42", 1, "42"},
		{"a select with no options", InfoField{Type: FieldSelect}, "x", 1, "x"},
	} {
		if got := cycleValue(c.f, c.current, c.delta); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDefaultInfoValues(t *testing.T) {
	fields := []InfoField{
		{Name: "subject_id", Label: "Subject ID", Default: ""},
		{Name: "level", Label: "Level", Type: FieldSelect,
			Options: []string{"one", "two", "three"}},
		{Name: "fullscreen", Label: "Fullscreen", Type: FieldCheckbox, Default: "true"},
		{Name: "note", Label: "Note", Default: "hello"},
	}

	got := defaultInfoValues(fields)

	if got["note"] != "hello" {
		t.Errorf("note = %q, want the field Default", got["note"])
	}
	if got["fullscreen"] != "true" {
		t.Errorf("fullscreen = %q, want true", got["fullscreen"])
	}
	// A select with no Default must land on its first option rather than the
	// empty string, which no branch of an experiment's switch would match.
	if got["level"] != "one" {
		t.Errorf("level = %q, want the first option", got["level"])
	}
	if _, ok := got["subject_id"]; !ok {
		t.Error("subject_id missing from the returned map")
	}
}

func TestNormaliseSelectsForcesAValidOption(t *testing.T) {
	fields := []InfoField{
		{Name: "level", Type: FieldSelect, Options: []string{"one", "two"}},
		{Name: "free", Type: FieldText},
	}
	values := map[string]string{"level": "nonsense", "free": "nonsense"}

	normaliseSelects(fields, values)

	if values["level"] != "one" {
		t.Errorf("level = %q, want the first option after normalising", values["level"])
	}
	// A text field is not a select and must be left exactly as it was.
	if values["free"] != "nonsense" {
		t.Errorf("free = %q, want it untouched", values["free"])
	}

	values["level"] = "two"
	normaliseSelects(fields, values)
	if values["level"] != "two" {
		t.Errorf("level = %q, a valid option must survive normalising", values["level"])
	}
}
