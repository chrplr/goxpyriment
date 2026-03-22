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
	exp := control.NewExperimentFromFlags("Video Player Example", control.Black, control.White, 32)
	defer exp.End()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	terminate := false

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
		if ext == ".mpg" || ext == ".mpeg" {
			videoFiles = append(videoFiles, filepath.Join("assets", file.Name()))
		}
	}

	if len(videoFiles) == 0 {
		fmt.Println("No .mpg video files found in assets folder.")
		return
	}

	for i, videoPath := range videoFiles {
		if terminate {
			break
		}

		fmt.Printf("Playing video %d/%d: %s\n", i+1, len(videoFiles), videoPath)

		vid, err := stimuli.NewVideo(exp.Screen, videoPath)
		if err != nil {
			log.Printf("failed to load video %s: %v", videoPath, err)
			continue
		}

		vid.Play()

		err = exp.Run(func() error {
			select {
			case <-sigChan:
				terminate = true
				return control.EndLoop
			default:
			}

			if err := vid.Update(); err != nil {
				if err == io.EOF {
					return control.EndLoop
				}
				return err
			}

			exp.Screen.Clear()
			vid.Draw(exp.Screen, 0, 0)
			exp.Screen.Update()

			if !vid.IsPlaying() {
				return control.EndLoop
			}

			key, _, err := exp.HandleEvents()
			if err == control.EndLoop {
				terminate = true
				return control.EndLoop
			}

			if key == control.K_SPACE {
				if vid.IsPaused() {
					vid.Play()
				} else {
					vid.Pause()
				}
			}

			if key == control.K_S {
				return control.EndLoop
			}

			return nil
		})

		vid.Close()

		if i < len(videoFiles)-1 && !terminate {
			exp.Screen.Clear()
			exp.Screen.Update()
			gapStartTime := clock.GetTime()
			for clock.GetTime()-gapStartTime < 4000 {
				key, _, err := exp.HandleEvents()
				if err == control.EndLoop || key != 0 {
					if err == control.EndLoop {
						terminate = true
					}
					break
				}
				select {
				case <-sigChan:
					terminate = true
					break
				default:
				}
				if terminate {
					break
				}
				clock.Wait(10)
			}
		}
	}
	fmt.Println("Finished.")
}
