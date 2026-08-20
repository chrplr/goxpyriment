// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

// spritesheet.go — one texture holding many stimuli, drawn by sub-rectangle.
//
// RATIONALE: the idiom of packing every stimulus of a condition into a single
// image and selecting one by its source rectangle comes from game development
// and from Psychtoolbox, where `Screen('DrawTexture', win, tex, sourceRect)`
// takes the same rectangle. It is worth having here for the same reason it is
// used there: uploading one texture is much cheaper than uploading N.
//
// The cost this avoids is concentrated exactly where this library cares most.
// PresentStreamOfImages preloads every visual and disables the collector
// before its VSYNC-locked loop (see stream.go), because a texture allocation
// inside the loop costs a frame. With N separate Pictures that is N image
// decodes, N CreateTextureFromSurface calls and N resident textures; with a
// sheet it is one of each, and drawing item k is a change of source rectangle
// with no allocation and no texture rebind. It also matters for GOOS=js: the
// browser build must //go:embed its assets, and one embedded file decoded once
// beats several hundred.

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/img"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// SpriteSheet is a single image holding a grid of stimuli, and the one GPU
// texture they all share.
//
// The texture is uploaded lazily on first use, following the same convention
// as Picture. Call PreloadVisualOnScreen on any of its sprites (or on the
// sheet itself) to force the upload before a timing-critical loop.
//
// A SpriteSheet owns its texture: the sprites it hands out do not. Call
// Unload on the sheet, once, when finished.
type SpriteSheet struct {
	FilePath string
	Memory   []byte
	Texture  *sdl.Texture
	Width    float32 // of the whole sheet, known after preload
	Height   float32

	// ScaleMode is applied to the texture when it is created. It defaults to
	// sdl.SCALEMODE_NEAREST: with linear filtering, sampling near a sprite's
	// edge reaches across the clip boundary and pulls in a sliver of the
	// neighbouring sprite whenever the destination is not pixel-aligned.
	ScaleMode sdl.ScaleMode
}

// Sprite is one cell of a SpriteSheet: a visual stimulus that draws a
// sub-rectangle of the sheet's texture.
//
// Sprites are cheap. They hold no GPU resources of their own, so Unload is a
// no-op — destroying the shared texture is the sheet's job, and doing it from
// a sprite would blank every other sprite cut from the same sheet.
type Sprite struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload
	Sheet      *SpriteSheet
	Clip       sdl.FRect // source rectangle within the sheet
	Width      float32   // destination size; defaults to the clip size
	Height     float32
}

// NewSpriteSheet creates a sprite sheet from an image file.
func NewSpriteSheet(filePath string) *SpriteSheet {
	return &SpriteSheet{FilePath: filePath, ScaleMode: sdl.SCALEMODE_NEAREST}
}

// NewSpriteSheetFromMemory creates a sprite sheet from embedded image data,
// as returned by //go:embed. This is the form the browser build needs.
func NewSpriteSheetFromMemory(data []byte) *SpriteSheet {
	return &SpriteSheet{Memory: data, ScaleMode: sdl.SCALEMODE_NEAREST}
}

// preload decodes the image and uploads it as a single texture.
func (s *SpriteSheet) preload(screen *apparatus.Screen) error {
	if s.Texture != nil {
		return nil
	}

	var surface *sdl.Surface
	var err error
	if s.Memory != nil {
		ioStream, err := sdl.IOFromBytes(s.Memory)
		if err != nil {
			return fmt.Errorf("stimuli.SpriteSheet: reading image from memory: %w", err)
		}
		defer ioStream.Close()
		surface, err = img.LoadIO(ioStream, false)
		if err != nil {
			return fmt.Errorf("stimuli.SpriteSheet: decoding image from memory: %w", err)
		}
	} else {
		surface, err = img.Load(s.FilePath)
		if err != nil {
			return fmt.Errorf("stimuli.SpriteSheet: loading image %q: %w", s.FilePath, err)
		}
	}
	defer surface.Destroy()

	s.Width = float32(surface.W)
	s.Height = float32(surface.H)

	texture, err := screen.Renderer.CreateTextureFromSurface(surface)
	if err != nil {
		return fmt.Errorf("stimuli.SpriteSheet: creating texture: %w", err)
	}
	if err := texture.SetScaleMode(s.ScaleMode); err != nil {
		texture.Destroy()
		return fmt.Errorf("stimuli.SpriteSheet: setting scale mode: %w", err)
	}
	s.Texture = texture
	return nil
}

// Size returns the sheet's pixel dimensions, uploading the texture if needed.
//
// Grid does not need this — it derives the cell size from the sheet once the
// texture exists — but a caller laying out a display may want it earlier.
func (s *SpriteSheet) Size(screen *apparatus.Screen) (float32, float32, error) {
	if err := s.preload(screen); err != nil {
		return 0, 0, fmt.Errorf("stimuli.SpriteSheet.Size: %w", err)
	}
	return s.Width, s.Height, nil
}

// Grid cuts the sheet into cols*rows equal cells and returns one Sprite per
// cell, in row-major order: left to right, then top to bottom.
//
// The parameters are named rather than positional-by-convention on purpose;
// passing a sheet's rows where its columns belong transposes the whole set and
// shows up only as scrambled stimuli.
//
// Grid assumes the cells tile the image exactly, with no border around the
// sheet and no gap between cells — the layout of an authored sprite sheet. Use
// GridWithSpacing when the sheet has a margin or gutters, and note that a
// generated contact sheet whose cells are not exactly regular needs its cells
// measured rather than assumed.
//
// The sheet's texture is uploaded here, so Grid needs the Screen.
func (s *SpriteSheet) Grid(screen *apparatus.Screen, cols, rows int) ([]*Sprite, error) {
	return s.GridWithSpacing(screen, cols, rows, 0, 0)
}

// GridWithSpacing cuts the sheet into cols*rows cells, skipping `margin`
// pixels around the whole sheet and `spacing` pixels between adjacent cells.
// This is the layout convention used by tileset editors.
//
//	cell width = (sheetWidth - 2*margin - (cols-1)*spacing) / cols
func (s *SpriteSheet) GridWithSpacing(screen *apparatus.Screen, cols, rows int, margin, spacing float32) ([]*Sprite, error) {
	if err := s.preload(screen); err != nil {
		return nil, fmt.Errorf("stimuli.SpriteSheet.Grid: %w", err)
	}
	clips, err := gridClips(s.Width, s.Height, cols, rows, margin, spacing)
	if err != nil {
		return nil, err
	}
	sprites := make([]*Sprite, 0, len(clips))
	for _, clip := range clips {
		sprites = append(sprites, &Sprite{
			Sheet:  s,
			Clip:   clip,
			Width:  clip.W,
			Height: clip.H,
		})
	}
	return sprites, nil
}

// gridClips computes the source rectangles of a cols x rows grid over a sheet
// of the given size. Split out from GridWithSpacing so the layout arithmetic
// can be tested without a renderer.
func gridClips(sheetW, sheetH float32, cols, rows int, margin, spacing float32) ([]sdl.FRect, error) {
	if cols < 1 || rows < 1 {
		return nil, fmt.Errorf("stimuli.SpriteSheet.Grid: cols and rows must be >= 1, got %dx%d", cols, rows)
	}
	if margin < 0 || spacing < 0 {
		return nil, fmt.Errorf("stimuli.SpriteSheet.Grid: margin and spacing must be >= 0, got %g and %g", margin, spacing)
	}
	cw := (sheetW - 2*margin - float32(cols-1)*spacing) / float32(cols)
	ch := (sheetH - 2*margin - float32(rows-1)*spacing) / float32(rows)
	if cw <= 0 || ch <= 0 {
		return nil, fmt.Errorf(
			"stimuli.SpriteSheet.Grid: %dx%d cells with margin %g and spacing %g "+
				"do not fit in a %gx%g sheet", cols, rows, margin, spacing, sheetW, sheetH)
	}
	clips := make([]sdl.FRect, 0, cols*rows)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			clips = append(clips, sdl.FRect{
				X: margin + float32(c)*(cw+spacing),
				Y: margin + float32(r)*(ch+spacing),
				W: cw,
				H: ch,
			})
		}
	}
	return clips, nil
}

// Sprites cuts the sheet into the given explicit source rectangles, for sheets
// whose cells are not on a regular grid.
func (s *SpriteSheet) Sprites(screen *apparatus.Screen, clips []sdl.FRect) ([]*Sprite, error) {
	if err := s.preload(screen); err != nil {
		return nil, fmt.Errorf("stimuli.SpriteSheet.Sprites: %w", err)
	}
	sprites := make([]*Sprite, 0, len(clips))
	for _, clip := range clips {
		sprites = append(sprites, &Sprite{
			Sheet:  s,
			Clip:   clip,
			Width:  clip.W,
			Height: clip.H,
		})
	}
	return sprites, nil
}

// Unload destroys the shared texture. Every Sprite cut from this sheet stops
// being drawable, so call it when the whole set is finished with.
func (s *SpriteSheet) Unload() error {
	return destroyTexture(&s.Texture)
}

// Draw renders the sprite's cell of the sheet, centred on its position.
func (sp *Sprite) Draw(screen *apparatus.Screen) error {
	if sp.Sheet == nil {
		return fmt.Errorf("stimuli.Sprite.Draw: sprite has no sheet")
	}
	if sp.Sheet.Texture == nil {
		if err := sp.Sheet.preload(screen); err != nil {
			return fmt.Errorf("stimuli.Sprite.Draw: %w", err)
		}
	}
	w, h := sp.Width, sp.Height
	if w == 0 {
		w = sp.Clip.W
	}
	if h == 0 {
		h = sp.Clip.H
	}
	destRect := screen.CenteredRect(sp.Position, w, h)
	return screen.Renderer.RenderTexture(sp.Sheet.Texture, &sp.Clip, destRect)
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (sp *Sprite) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(sp, screen, clear, update)
}

// Unload is deliberately a no-op: the texture belongs to the SpriteSheet, and
// destroying it here would blank every other sprite cut from the same sheet.
// Call SpriteSheet.Unload instead. (BaseVisual.Unload already does nothing;
// this override exists to state why.)
func (sp *Sprite) Unload() error {
	return nil
}
