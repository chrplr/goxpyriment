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
	"github.com/chrplr/goxpyriment/misc"
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
	exp := control.NewExperiment("Video Player Example", width, height, fullscreen)
	exp.SubjectID = *subject
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

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
		if ext == ".mp4" || ext == ".mov" || ext == ".mkv" {
			videoFiles = append(videoFiles, filepath.Join("assets", file.Name()))
		}
	}

	if len(videoFiles) == 0 {
		fmt.Println("No video files found in assets folder.")
		return
	}

	fmt.Println("Controls: [SPACE] to pause/resume, [S] to skip video, [ESC] or close window to quit.")

	// 4. Play each video
	for i, videoPath := range videoFiles {
		if terminate {
			break
		}

		fmt.Printf("Playing video %d/%d: %s\n", i+1, len(videoFiles), videoPath)

		vid := stimuli.NewVideo(videoPath)
		if err := vid.Preload(); err != nil {
			log.Printf("failed to preload video %s: %v", videoPath, err)
			continue
		}
		if err := vid.PreloadDevice(exp.Screen, exp.AudioDevice); err != nil {
			log.Printf("failed to prepare device for video %s: %v", videoPath, err)
			vid.Unload()
			continue
		}

		// Play the video
		if err := vid.Play(); err != nil {
			log.Printf("failed to play video %s: %v", videoPath, err)
			vid.Unload()
			continue
		}

		// Main loop for the current video
		err = exp.Run(func() error {
			// Check for Ctrl-C
			select {
			case <-sigChan:
				terminate = true
				return sdl.EndLoop
			default:
			}

			// Update video decoding and audio
			if err := vid.Update(); err != nil {
				return err
			}

			// Draw current frame
			if err := vid.Present(exp.Screen, true, true); err != nil {
				return err
			}

			// Exit the loop when video ends
			if !vid.IsPlaying() {
				return sdl.EndLoop
			}

			// Handle events
			key, _, err := exp.HandleEvents()
			if err == sdl.EndLoop {
				// ESC or Window Close detected
				terminate = true
				return sdl.EndLoop
			}
			
			// Pause/Resume toggle
			if key == sdl.K_SPACE {
				if vid.IsPaused() {
					fmt.Println("Resuming...")
					vid.Play()
				} else {
					fmt.Println("Pausing...")
					vid.Pause()
				}
			}
			
			// Skip current video
			if key == sdl.K_S {
				fmt.Println("Skipping...")
				return sdl.EndLoop
			}

			return nil
		})

		if err != nil && err != sdl.EndLoop {
			log.Printf("error during video playback: %v", err)
		}

		vid.Unload()

		// 5. 4-second gap (responsive to quit signals)
		if i < len(videoFiles)-1 && !terminate {
			fmt.Println("Waiting for 4 seconds...")
			exp.Screen.Clear()
			exp.Screen.Update()
			
			gapStartTime := misc.GetTime()
			for misc.GetTime()-gapStartTime < 4000 {
				// Poll events to keep window responsive and check for ESC/Quit
				key, _, err := exp.HandleEvents()
				if err == sdl.EndLoop {
					terminate = true
					break
				}
				if key != 0 {
					// Any key skips the gap
					break
				}
				
				// Check for Ctrl-C
				select {
				case <-sigChan:
					terminate = true
					break
				default:
				}
				
				if terminate {
					break
				}
				sdl.Delay(10)
			}
		}
	}

	fmt.Println("Finished.")
}
