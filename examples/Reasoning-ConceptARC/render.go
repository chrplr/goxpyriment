// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"math"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Layout ───────────────────────────────────────────────────────────────────
//
// Coordinates are center-relative with +Y pointing UP (see apparatus/CLAUDE.md);
// row 0 of a grid is therefore the TOP row and gets the LARGEST Y. Everything
// below is laid out on a fixed 1280×800 logical canvas that SDL scales to the
// window (letterboxed), so the layout is the same on every display.

const (
	logicalW = int32(1280)
	logicalH = int32(800)

	// Left panel: the demonstrations.
	demoLeft, demoRight = float32(-620), float32(60)
	demoTop, demoBottom = float32(330), float32(-380)

	// Right panel: test input above, the editable output below.
	panelLeft, panelRight = float32(100), float32(620)
	testTop, testBottom   = float32(330), float32(90)
	outTop, outBottom     = float32(40), float32(-210)

	headerY = float32(350) // "Demonstrations" / "Test input"
	outHdrY = float32(60)  // "Your output"
	statusY = float32(378) // task counter (left) and feedback (right)

	paletteY   = float32(-250) // centre line of the colour swatches
	swatchSize = float32(38)
	swatchStep = float32(48)

	buttonRow1Y = float32(-310) // rows/cols −/+
	buttonRow2Y = float32(-360) // copy / reset / submit
	buttonH     = float32(40)

	maxCell   = float32(24) // cells never grow past this, however small the grid
	pairGap   = float32(34) // room for the arrow between a demo input and output
	slotPad   = float32(8)  // inner margin of a demonstration slot
	gridInset = float32(1)  // gap between a cell's fill and the grid line
)

// ARC colour palette, indices 0–9, from Chollet's testing interface.
var arcColors = [10]control.Color{
	control.RGB(0x00, 0x00, 0x00), // 0 black
	control.RGB(0x00, 0x74, 0xD9), // 1 blue
	control.RGB(0xFF, 0x41, 0x36), // 2 red
	control.RGB(0x2E, 0xCC, 0x40), // 3 green
	control.RGB(0xFF, 0xDC, 0x00), // 4 yellow
	control.RGB(0xAA, 0xAA, 0xAA), // 5 grey
	control.RGB(0xF0, 0x12, 0xBE), // 6 magenta
	control.RGB(0xFF, 0x85, 0x1B), // 7 orange
	control.RGB(0x7F, 0xDB, 0xFF), // 8 sky blue
	control.RGB(0x87, 0x0C, 0x25), // 9 maroon
}

var (
	bgColor    = control.RGB(240, 240, 240)
	textColor  = control.RGB(30, 30, 30)
	gridLine   = control.RGB(150, 150, 150)
	gridFrame  = control.RGB(60, 60, 60)
	hoverColor = control.RGB(255, 255, 255)
	selectEdge = control.RGB(30, 30, 30)
	okColor    = control.RGB(20, 130, 40)
	errColor   = control.RGB(200, 30, 30)
	buttonFill = control.RGB(215, 215, 215)
	buttonHot  = control.RGB(190, 190, 190)
	buttonEdge = control.RGB(90, 90, 90)
	arrowColor = control.RGB(90, 90, 90)
)

// ── Grid geometry ────────────────────────────────────────────────────────────

// gridBox places a rows×cols grid on screen: x0 is its left edge, yTop its top
// edge (both centre-relative), cell the side of one cell.
type gridBox struct {
	x0, yTop   float32
	cell       float32
	rows, cols int
}

// fitGrid returns the box that centres a rows×cols grid inside the rectangle
// [left,right]×[bottom,top] with the largest cell size that fits (capped).
func fitGrid(rows, cols int, left, right, bottom, top float32) gridBox {
	cell := float32(math.Min(float64((right-left)/float32(cols)), float64((top-bottom)/float32(rows))))
	if cell > maxCell {
		cell = maxCell
	}
	w, h := cell*float32(cols), cell*float32(rows)
	return gridBox{
		x0:   (left+right)/2 - w/2,
		yTop: (top+bottom)/2 + h/2,
		cell: cell, rows: rows, cols: cols,
	}
}

func (b gridBox) width() float32  { return b.cell * float32(b.cols) }
func (b gridBox) height() float32 { return b.cell * float32(b.rows) }

// cellCenter returns the centre of cell (row, col).
func (b gridBox) cellCenter(row, col int) control.FPoint {
	return control.FPoint{
		X: b.x0 + b.cell*(float32(col)+0.5),
		Y: b.yTop - b.cell*(float32(row)+0.5),
	}
}

// cellAt is the inverse of cellCenter: it maps a point to the cell containing
// it. ok is false when the point falls outside the grid.
func (b gridBox) cellAt(x, y float32) (row, col int, ok bool) {
	col = int(math.Floor(float64((x - b.x0) / b.cell)))
	row = int(math.Floor(float64((b.yTop - y) / b.cell)))
	if row < 0 || row >= b.rows || col < 0 || col >= b.cols {
		return row, col, false
	}
	return row, col, true
}

// drawGrid renders a grid in its box: a dark frame, one filled rectangle per
// cell, and the grid lines on top. Hundreds of RenderFillRect calls per frame
// are cheap; nothing here allocates a texture.
func drawGrid(exp *control.Experiment, g Grid, b gridBox) error {
	frame := stimuli.NewRectangle(b.x0+b.width()/2, b.yTop-b.height()/2, b.width()+2, b.height()+2, gridFrame)
	if err := frame.Draw(exp.Screen); err != nil {
		return err
	}
	for r := 0; r < b.rows; r++ {
		for c := 0; c < b.cols; c++ {
			p := b.cellCenter(r, c)
			cell := stimuli.NewRectangle(p.X, p.Y, b.cell-gridInset, b.cell-gridInset, arcColors[g[r][c]])
			if err := cell.Draw(exp.Screen); err != nil {
				return err
			}
		}
	}
	// Grid lines are drawn only when cells are big enough for them to read as
	// lines rather than as noise.
	if b.cell < 6 {
		return nil
	}
	left, right := b.x0, b.x0+b.width()
	top, bottom := b.yTop, b.yTop-b.height()
	for r := 1; r < b.rows; r++ {
		y := b.yTop - float32(r)*b.cell
		if err := stimuli.NewLine(control.Point(left, y), control.Point(right, y), gridLine, 1).Draw(exp.Screen); err != nil {
			return err
		}
	}
	for c := 1; c < b.cols; c++ {
		x := b.x0 + float32(c)*b.cell
		if err := stimuli.NewLine(control.Point(x, top), control.Point(x, bottom), gridLine, 1).Draw(exp.Screen); err != nil {
			return err
		}
	}
	return nil
}

// outlineCell draws a thick outline around one cell (hover cue on the editor).
func outlineCell(exp *control.Experiment, b gridBox, row, col int, color control.Color) error {
	p := b.cellCenter(row, col)
	h := b.cell / 2
	return stimuli.NewPolyLine([]control.FPoint{
		{X: p.X - h, Y: p.Y - h}, {X: p.X + h, Y: p.Y - h},
		{X: p.X + h, Y: p.Y + h}, {X: p.X - h, Y: p.Y + h},
	}, true, 2, color).Draw(exp.Screen)
}

// drawArrow draws a small right-pointing arrow centred at (x, y).
func drawArrow(exp *control.Experiment, x, y float32) error {
	const half, head = float32(11), float32(6)
	pts := []control.FPoint{
		{X: x - half, Y: y}, {X: x + half, Y: y},
		{X: x + half - head, Y: y + head}, {X: x + half, Y: y}, {X: x + half - head, Y: y - head},
	}
	return stimuli.NewPolyLine(pts, false, 2, arrowColor).Draw(exp.Screen)
}

// ── Demonstrations ───────────────────────────────────────────────────────────

// demoLayout is the placement of one demonstration pair.
type demoLayout struct {
	in, out gridBox
	arrowX  float32
	arrowY  float32
}

// layoutDemos arranges the demonstration pairs in the left panel: one column
// for up to three pairs, two columns beyond that (the corpus has at most
// five). Each pair gets the largest cell size that fits its slot.
func layoutDemos(train []Pair) []demoLayout {
	n := len(train)
	ncols := 1
	if n > 3 {
		ncols = 2
	}
	nrows := (n + ncols - 1) / ncols
	slotW := (demoRight - demoLeft) / float32(ncols)
	slotH := (demoTop - demoBottom) / float32(nrows)

	out := make([]demoLayout, n)
	for i, p := range train {
		sl := demoLeft + slotW*float32(i%ncols) + slotPad
		sr := sl + slotW - 2*slotPad
		st := demoTop - slotH*float32(i/ncols) - slotPad
		sb := st - slotH + 2*slotPad

		ci, co := p.Input.Cols(), p.Output.Cols()
		ri, ro := p.Input.Rows(), p.Output.Rows()
		cell := float32(math.Min(float64((sr-sl-pairGap)/float32(ci+co)), float64((st-sb)/float32(max(ri, ro)))))
		if cell > maxCell {
			cell = maxCell
		}
		wIn, wOut := cell*float32(ci), cell*float32(co)
		total := wIn + pairGap + wOut
		x := (sl+sr)/2 - total/2
		cy := (st + sb) / 2
		out[i] = demoLayout{
			in:     gridBox{x0: x, yTop: cy + cell*float32(ri)/2, cell: cell, rows: ri, cols: ci},
			out:    gridBox{x0: x + wIn + pairGap, yTop: cy + cell*float32(ro)/2, cell: cell, rows: ro, cols: co},
			arrowX: x + wIn + pairGap/2,
			arrowY: cy,
		}
	}
	return out
}

// ── Buttons ──────────────────────────────────────────────────────────────────

// button is a labelled clickable rectangle. Hit-testing is done in the
// centre-based coordinate space that Screen.MousePosition already returns.
// (Adapted from examples/Geometry-Triangle-Completion/ui.go.)
type button struct {
	label      string
	cx, cy     float32
	w, h       float32
	hovered    bool
	labelStim  *stimuli.TextLine
	background *stimuli.Rectangle
	border     *stimuli.PolyLine
}

func newButton(label string, cx, cy, w, h float32) *button {
	b := &button{label: label, cx: cx, cy: cy, w: w, h: h}
	b.background = stimuli.NewRectangle(cx, cy, w, h, buttonFill)
	b.border = stimuli.NewPolyLine([]control.FPoint{
		{X: cx - w/2, Y: cy - h/2}, {X: cx + w/2, Y: cy - h/2},
		{X: cx + w/2, Y: cy + h/2}, {X: cx - w/2, Y: cy + h/2},
	}, true, 2, buttonEdge)
	b.labelStim = stimuli.NewTextLine(label, cx, cy, textColor)
	return b
}

func (b *button) contains(x, y float32) bool {
	return x >= b.cx-b.w/2 && x <= b.cx+b.w/2 && y >= b.cy-b.h/2 && y <= b.cy+b.h/2
}

func (b *button) draw(exp *control.Experiment) error {
	b.background.Color = buttonFill
	if b.hovered {
		b.background.Color = buttonHot
	}
	if err := b.background.Draw(exp.Screen); err != nil {
		return err
	}
	if err := b.border.Draw(exp.Screen); err != nil {
		return err
	}
	return b.labelStim.Draw(exp.Screen)
}

// updateHover refreshes every button's hover state from the cursor position and
// returns the index under the cursor, or -1.
func updateHover(buttons []*button, mx, my float32) int {
	hit := -1
	for i, b := range buttons {
		b.hovered = b.contains(mx, my)
		if b.hovered {
			hit = i
		}
	}
	return hit
}

// ── Palette ──────────────────────────────────────────────────────────────────

// swatchCenter returns the centre of palette swatch i (0–9), laid out in one
// row centred on the right panel.
func swatchCenter(i int) control.FPoint {
	x := (panelLeft+panelRight)/2 - swatchStep*4.5 + swatchStep*float32(i)
	return control.FPoint{X: x, Y: paletteY}
}

// swatchAt returns the palette index under (x, y), or -1.
func swatchAt(x, y float32) int {
	for i := range arcColors {
		c := swatchCenter(i)
		if x >= c.X-swatchSize/2 && x <= c.X+swatchSize/2 && y >= c.Y-swatchSize/2 && y <= c.Y+swatchSize/2 {
			return i
		}
	}
	return -1
}

// drawPalette draws the ten swatches, outlining the selected one.
func drawPalette(exp *control.Experiment, selected int) error {
	for i, col := range arcColors {
		c := swatchCenter(i)
		if err := stimuli.NewRectangle(c.X, c.Y, swatchSize+2, swatchSize+2, gridLine).Draw(exp.Screen); err != nil {
			return err
		}
		if err := stimuli.NewRectangle(c.X, c.Y, swatchSize, swatchSize, col).Draw(exp.Screen); err != nil {
			return err
		}
		if i == selected {
			h := swatchSize/2 + 4
			sel := stimuli.NewPolyLine([]control.FPoint{
				{X: c.X - h, Y: c.Y - h}, {X: c.X + h, Y: c.Y - h},
				{X: c.X + h, Y: c.Y + h}, {X: c.X - h, Y: c.Y + h},
			}, true, 3, selectEdge)
			if err := sel.Draw(exp.Screen); err != nil {
				return err
			}
		}
	}
	return nil
}
