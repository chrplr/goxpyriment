// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/io"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/stimuli"

	"github.com/Zyko0/go-sdl3/sdl"
)

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	// 1. Setup Signal Handling for Ctrl-C in console
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	terminate := false

	// 2. Create and initialize the experiment
	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}
	exp := control.NewExperiment("Dual Video Player Example", width, height, fullscreen, control.Black, control.White, 32)
	exp.SubjectID = *subject
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// Set up data recording: one row per key press
	// subject_id, pair_index, phase, video_left, video_right, key, t_rel_ms
	exp.Data.AddVariableNames([]string{"pair_index", "phase", "video_left", "video_right", "key", "t_rel_ms"})

	// 3. Identify video files in assets
	files, err := os.ReadDir("assets")
	if err != nil {
		log.Fatalf("failed to read assets directory: %v", err)
	}

	var videoFiles []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		ext := filepath.Ext(file.Name())
		if  ext == ".mov" || ext == ".mkv" {
			videoFiles = append(videoFiles, filepath.Join("assets", file.Name()))
		}
	}

	if len(videoFiles) < 2 {
		fmt.Println("Need at least two video files in assets folder.")
		return
	}

	// Use the first two videos found
	leftPath := videoFiles[0]
	rightPath := videoFiles[1]
	pairIndex := 1

	fmt.Println("Controls: [SPACE] to pause/resume both, [S] to skip pair, [ESC] or close window to quit.")
	fmt.Printf("Left video: %s\nRight video: %s\n", leftPath, rightPath)

	// 4. Create fixation cross (centered)
	fix := stimuli.NewFixCross(40, 4, control.DefaultTextColor)

	// 5. Prepare videos
	leftVid := stimuli.NewVideo(leftPath)
	rightVid := stimuli.NewVideo(rightPath)

	if err := leftVid.Preload(); err != nil {
		log.Fatalf("failed to preload left video %s: %v", leftPath, err)
	}
	if err := rightVid.Preload(); err != nil {
		log.Fatalf("failed to preload right video %s: %v", rightPath, err)
	}

	if err := leftVid.PreloadDevice(exp.Screen, exp.AudioDevice); err != nil {
		log.Fatalf("failed to prepare device for left video %s: %v", leftPath, err)
	}
	if err := rightVid.PreloadDevice(exp.Screen, exp.AudioDevice); err != nil {
		log.Fatalf("failed to prepare device for right video %s: %v", rightPath, err)
	}

	// 6. Start both videos
	if err := leftVid.Play(); err != nil {
		log.Fatalf("failed to play left video %s: %v", leftPath, err)
	}
	if err := rightVid.Play(); err != nil {
		log.Fatalf("failed to play right video %s: %v", rightPath, err)
	}

	videoStart := clock.GetTime()

	// 7. Main loop for the pair of videos
	err = exp.Run(func() error {
		// Check for Ctrl-C
		select {
		case <-sigChan:
			terminate = true
			return sdl.EndLoop
		default:
		}

		// Update both videos (decoding + audio)
		if err := leftVid.Update(); err != nil {
			return err
		}
		if err := rightVid.Update(); err != nil {
			return err
		}

		// Clear screen once
		if err := exp.Screen.Clear(); err != nil {
			return err
		}

		// Draw left and right videos manually with positioning, scaling each to fit
		w, h, _ := exp.Screen.Renderer.RenderOutputSize()
		screenW := float32(w)
		screenH := float32(h)

		// Each video must fit within half the screen width and full height
		maxVideoW := screenW / 2
		maxVideoH := screenH

		// Compute scaled size for left video (preserve aspect ratio)
		leftScaleW := maxVideoW / float32(leftVid.Width)
		leftScaleH := maxVideoH / float32(leftVid.Height)
		leftScale := leftScaleW
		if leftScaleH < leftScale {
			leftScale = leftScaleH
		}
		leftW := float32(leftVid.Width) * leftScale
		leftH := float32(leftVid.Height) * leftScale

		// Compute scaled size for right video (preserve aspect ratio)
		rightScaleW := maxVideoW / float32(rightVid.Width)
		rightScaleH := maxVideoH / float32(rightVid.Height)
		rightScale := rightScaleW
		if rightScaleH < rightScale {
			rightScale = rightScaleH
		}
		rightW := float32(rightVid.Width) * rightScale
		rightH := float32(rightVid.Height) * rightScale

		// Center positions for left and right videos (quarters of screen)
		leftCenterX := screenW * 0.25
		rightCenterX := screenW * 0.75
		centerY := screenH * 0.5

		// Destination rects for left and right videos (centered on their respective positions)
		leftDest := sdl.FRect{
			X: leftCenterX - leftW/2,
			Y: centerY - leftH/2,
			W: leftW,
			H: leftH,
		}
		rightDest := sdl.FRect{
			X: rightCenterX - rightW/2,
			Y: centerY - rightH/2,
			W: rightW,
			H: rightH,
		}

		// Draw current frame of each video into its rect
		if err := drawVideoToRect(leftVid, exp.Screen, &leftDest); err != nil {
			return err
		}
		if err := drawVideoToRect(rightVid, exp.Screen, &rightDest); err != nil {
			return err
		}

		// Draw fixation cross in the center
		if err := fix.Present(exp.Screen, false, false); err != nil {
			return err
		}

		// Update screen once
		if err := exp.Screen.Update(); err != nil {
			return err
		}

		// Exit when both videos are done (or one finished, depending on taste)
		if !leftVid.IsPlaying() && !rightVid.IsPlaying() {
			return sdl.EndLoop
		}

		// Handle events
		key, _, err := exp.HandleEvents()
		if err == sdl.EndLoop {
			terminate = true
			return sdl.EndLoop
		}

		// Record key presses during video playback
		if key != 0 {
			now := clock.GetTime()
			exp.Data.Add([]interface{}{
				pairIndex,
				"video",
				filepath.Base(leftPath),
				filepath.Base(rightPath),
				key,
				now - videoStart,
			})
		}

		// Pause/Resume toggle for both
		if key == sdl.K_SPACE {
			if leftVid.IsPaused() || rightVid.IsPaused() {
				fmt.Println("Resuming both videos...")
				leftVid.Play()
				rightVid.Play()
			} else {
				fmt.Println("Pausing both videos...")
				leftVid.Pause()
				rightVid.Pause()
			}
		}

		// Skip both videos
		if key == sdl.K_S {
			fmt.Println("Skipping both videos...")
			return sdl.EndLoop
		}

		return nil
	})

	if err != nil && err != sdl.EndLoop {
		log.Printf("error during dual video playback: %v", err)
	}

	// Cleanup
	leftVid.Unload()
	rightVid.Unload()

	// 8. Record keypresses during a 2-second blank screen that follows
	if !terminate {
		fmt.Println("Blank screen for 2 seconds (keys will be recorded)...")
		if err := exp.Screen.Clear(); err == nil {
			_ = exp.Screen.Update()
		}

		blankStart := clock.GetTime()
		for {
			now := clock.GetTime()
			if now-blankStart >= 2000 {
				break
			}
			key, _, err := exp.HandleEvents()
			if err == sdl.EndLoop {
				terminate = true
				break
			}
			if key != 0 {
				exp.Data.Add([]interface{}{
					pairIndex,
					"blank",
					filepath.Base(leftPath),
					filepath.Base(rightPath),
					key,
					now - blankStart,
				})
			}
			sdl.Delay(10)
		}
	}
}

// drawVideoToRect draws the current frame of v into the given destination rectangle.
func drawVideoToRect(v *stimuli.Video, screen *io.Screen, dest *sdl.FRect) error {
	return v.DrawAt(screen, dest)
}

