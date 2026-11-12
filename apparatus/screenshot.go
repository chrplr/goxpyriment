// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/Zyko0/go-sdl3/sdl"
)

// Capture returns what has been drawn on the window so far, before it is
// presented: the whole window output in its pixels, letterbox bars and the
// effect of the display transform included, as the screen will show it.
//
// SDL discards the backbuffer when a frame is presented, so the frame on the
// screen cannot be read back after Update. Call Capture after drawing and
// before Update, or from a pre-present hook (SetPrePresentHook), which runs
// once the photodiode square is drawn too. To save the next frame Update
// presents, use ScreenshotNextFrame.
//
// Reading pixels back stalls the GPU, so the frame being captured is presented
// late: this is for debugging and documentation, not for frames whose timing
// matters.
func (s *Screen) Capture() (*image.RGBA, error) {
	var surf *sdl.Surface
	read := func() error {
		var err error
		surf, err = s.Renderer.ReadPixels(nil)
		return err
	}
	// ReadPixels reads the window, not a texture that is the render target.
	if prev := s.Renderer.RenderTarget(); prev != nil {
		if err := s.Renderer.SetRenderTarget(nil); err != nil {
			return nil, fmt.Errorf("apparatus.Screen.Capture: selecting the window: %w", err)
		}
		defer func() { _ = s.Renderer.SetRenderTarget(prev) }()
	}
	// ReadPixels(nil) reads the viewport, which the display transform moves
	// and a letterbox (SetLogicalSize) insets, so both are lifted for the read:
	// the transform by withoutTransform, the letterbox by putting the window's
	// own presentation back.
	err := s.withoutTransform(func() error {
		if s.logical == nil {
			return read()
		}
		if err := s.Renderer.SetLogicalPresentation(s.native.w, s.native.h, s.native.mode); err != nil {
			return err
		}
		err := read()
		if rerr := s.Renderer.SetLogicalPresentation(s.logical.w, s.logical.h, s.logical.mode); rerr != nil && err == nil {
			err = rerr
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("apparatus.Screen.Capture: reading the pixels: %w", err)
	}
	defer surf.Destroy()
	// RGBA32 is R, G, B, A in memory order on every platform, as image.RGBA.
	conv, err := surf.Convert(sdl.PIXELFORMAT_RGBA32)
	if err != nil {
		return nil, fmt.Errorf("apparatus.Screen.Capture: converting the pixels: %w", err)
	}
	defer conv.Destroy()
	img := image.NewRGBA(image.Rect(0, 0, int(conv.W), int(conv.H)))
	px, rowLen := conv.Pixels(), 4*int(conv.W)
	for y := range int(conv.H) {
		copy(img.Pix[y*img.Stride:y*img.Stride+rowLen], px[y*int(conv.Pitch):])
	}
	return img, nil
}

// ScreenshotNextFrame saves the next frame that Update presents, as it
// appears on the screen (see Capture), photodiode square included, to path as
// a PNG file. Calling it again before that frame replaces path.
//
// The pixels are read just before the frame is presented, which delays its
// presentation, and the file is written once Update has stamped the frame and
// written its marker code, which delays what comes after it. So it is for
// debugging and documentation, not for runs whose timing is analysed. An error
// reading or writing the frame is returned by that Update.
func (s *Screen) ScreenshotNextFrame(path string) {
	s.screenshotPath = path
}

// takeScreenshot captures the finished frame for a pending ScreenshotNextFrame
// request, if any. Update calls it just before presenting, and calls
// saveScreenshot with its result once the frame is stamped.
func (s *Screen) takeScreenshot() (*image.RGBA, string, error) {
	path := s.screenshotPath
	if path == "" {
		return nil, "", nil
	}
	s.screenshotPath = ""
	img, err := s.Capture()
	if err != nil {
		return nil, "", fmt.Errorf("screenshot %s: %w", path, err)
	}
	return img, path, nil
}

// saveScreenshot writes img to path as a PNG file.
func saveScreenshot(img *image.RGBA, path string) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("screenshot: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("screenshot %s: %w", path, cerr)
		}
	}()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("screenshot %s: %w", path, err)
	}
	return nil
}
