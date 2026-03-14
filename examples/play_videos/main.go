// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/stimuli"

	"github.com/Zyko0/go-sdl3/sdl"
)

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	terminate := false

	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}
	exp := control.NewExperiment("Video Player Example", width, height, fullscreen, control.Black, control.White, 32)
	exp.SubjectID = *subject
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	files, err := os.ReadDir("assets")
	if err != nil {
		log.Fatalf("failed to read assets directory: %v", err)
	}

	var videoFiles []string
	for _, file := range files {
		if file.IsDir() { continue }
		ext := filepath.Ext(file.Name())
		if ext == ".mpg" || ext == ".mpeg" {
			videoFiles = append(videoFiles, filepath.Join("assets", file.Name()))
		}
	}

	if len(videoFiles) == 0 {
		fmt.Println("No .mpg video files found in assets folder.")
		return
	}

	for i, videoPath := range videoFiles {
		if terminate { break }

		fmt.Printf("Playing video %d/%d: %s\n", i+1, len(videoFiles), videoPath)

		vid, err := stimuli.NewVideo(exp.Screen.Renderer, videoPath)
		if err != nil {
			log.Printf("failed to load video %s: %v", videoPath, err)
			continue
		}

		vid.Play()

		err = exp.Run(func() error {
			select {
			case <-sigChan:
				terminate = true
				return sdl.EndLoop
			default:
			}

			if err := vid.Update(exp.Screen.Renderer); err != nil {
				if err == io.EOF { return sdl.EndLoop }
				return err
			}

			exp.Screen.Clear()
			vid.Draw(exp.Screen.Renderer, 0, 0)
			exp.Screen.Update()

			if !vid.IsPlaying() { return sdl.EndLoop }

			key, _, err := exp.HandleEvents()
			if err == sdl.EndLoop {
				terminate = true
				return sdl.EndLoop
			}
			
			if key == sdl.K_SPACE {
				if vid.IsPaused() {
					vid.Play()
				} else {
					vid.Pause()
				}
			}
			
			if key == sdl.K_S { return sdl.EndLoop }

			return nil
		})

		vid.Close()

		if i < len(videoFiles)-1 && !terminate {
			exp.Screen.Clear()
			exp.Screen.Update()
			gapStartTime := clock.GetTime()
			for clock.GetTime()-gapStartTime < 4000 {
				key, _, err := exp.HandleEvents()
				if err == sdl.EndLoop || key != 0 {
					if err == sdl.EndLoop { terminate = true }
					break
				}
				select {
				case <-sigChan:
					terminate = true
					break
				default:
				}
				if terminate { break }
				sdl.Delay(10)
			}
		}
	}
	fmt.Println("Finished.")
}
