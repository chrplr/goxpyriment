// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("Dual Video Player", control.Black, control.White, 32)
	defer exp.End()

	// 1. Setup Signal Handling for Ctrl-C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	terminate := false // Declared here...

	exp.AddDataVariableNames([]string{"pair_index", "video_left", "video_right", "key", "t_rel_ms"})

	// 3. Identify .mpg files in assets
	files, err := os.ReadDir("assets")
	if err != nil {
		log.Fatalf("failed to read assets: %v", err)
	}

	var videoFiles []string
	for _, f := range files {
		ext := filepath.Ext(f.Name())
		if ext == ".mpg" || ext == ".mpeg" {
			videoFiles = append(videoFiles, filepath.Join("assets", f.Name()))
		}
	}

	if len(videoFiles) < 2 {
		fmt.Println("Error: Need at least two .mpg files in the assets folder.")
		return
	}

	leftPath, rightPath := videoFiles[0], videoFiles[1]

	// 4. Setup Stimuli
	fix := stimuli.NewFixCross(40, 4, control.White)

	leftVid, err := stimuli.NewVideo(exp.Screen, leftPath)
	if err != nil {
		log.Fatalf("Left video error: %v", err)
	}

	rightVid, err := stimuli.NewVideo(exp.Screen, rightPath)
	if err != nil {
		log.Fatalf("Right video error: %v", err)
	}

	fmt.Println("Controls: [SPACE] Pause/Resume, [R] Sync Rewind, [S] Skip, [ESC] Quit")

	leftVid.Play()
	rightVid.Play()
	videoStart := clock.GetTime()

	// 5. Main Experiment Loop
	// FIX: We now actually USE the 'terminate' variable to break sequences
	err = exp.Run(func() error {
		if terminate {
			return control.EndLoop
		}

		select {
		case <-sigChan:
			terminate = true
			return control.EndLoop
		default:
		}

		errL := leftVid.Update()
		errR := rightVid.Update()

		exp.Screen.Clear()

		w, h, _ := exp.Screen.Size()
		screenW, screenH := float32(w), float32(h)

		leftDest := calculateDestRect(leftVid, screenW*0.25, screenH*0.5, screenW/2, screenH)
		rightDest := calculateDestRect(rightVid, screenW*0.75, screenH*0.5, screenW/2, screenH)

		leftVid.DrawAt(exp.Screen, &leftDest)
		rightVid.DrawAt(exp.Screen, &rightDest)
		fix.Present(exp.Screen, false, false)

		exp.Screen.Update()

		if errL == io.EOF && errR == io.EOF {
			return control.EndLoop
		}

		key, _, err := exp.HandleEvents()
		if err == control.EndLoop {
			terminate = true
			return control.EndLoop
		}

		if key != 0 {
			exp.Data.Add(1, filepath.Base(leftPath), filepath.Base(rightPath), key, clock.GetTime()-videoStart)
		}

		if key == control.K_R {
			leftVid.Rewind()
			rightVid.Rewind()
			videoStart = clock.GetTime()
		}

		if key == control.K_SPACE {
			if leftVid.IsPaused() || rightVid.IsPaused() {
				leftVid.Play()
				rightVid.Play()
			} else {
				leftVid.Pause()
				rightVid.Pause()
			}
		}

		if key == control.K_S {
			return control.EndLoop
		}

		return nil
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Printf("Playback error: %v", err)
	}

	leftVid.Close()
	rightVid.Close()
	fmt.Println("Finished playback.")
}

func calculateDestRect(v *stimuli.Video, centerX, centerY, maxW, maxH float32) control.FRect {
	scaleW := maxW / float32(v.Width)
	scaleH := maxH / float32(v.Height)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}
	w := float32(v.Width) * scale
	h := float32(v.Height) * scale
	return control.FRect{X: centerX - w/2, Y: centerY - h/2, W: w, H: h}
}
