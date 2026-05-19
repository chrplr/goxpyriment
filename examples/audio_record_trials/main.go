// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Audio_record_trials runs 10 successive trials, each capturing 1 s of
// microphone audio into trial_01.wav … trial_10.wav beside the session CSV.
package main

import (
	"fmt"
	"path/filepath"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	nTrials  = 10
	recordMs = 1000
)

func main() {
	exp := control.NewExperimentFromFlags("Audio capture — 10 × 1 s", control.Black, control.White, 32)
	defer exp.End()

	exp.AddDataVariableNames([]string{"trial", "wav_file", "pcm_bytes", "recording_device"})

	instructions := fmt.Sprintf(
		"This session records %d trials of %d ms each.\n\n"+
			"You will choose the microphone first.\n"+
			"WAV files are written next to your CSV.\n\nPress SPACE to begin.",
		nTrials, recordMs)

	err := exp.Run(func() error {
		if err := exp.ShowInstructions(instructions); err != nil {
			return err
		}

		mic, err := exp.SelectAudioRecordingDevice("Select microphone")
		if err != nil {
			return err
		}

		spec := stimuli.DefaultRecorderSpec()
		rec, err := exp.OpenAudioRecorderOnDevice(mic.ID, &spec)
		if err != nil {
			return fmt.Errorf("open recorder: %w", err)
		}

		outFmt := rec.OutputFormat()

		for t := 0; t < nTrials; t++ {
			if _, derr := rec.Drain(nil); derr != nil {
				return derr
			}

			label := stimuli.NewTextLine(
				fmt.Sprintf("Trial %d / %d — recording…", t+1, nTrials),
				0, 0, control.White)

			if err := rec.Start(); err != nil {
				return err
			}
			if err := exp.ShowTimed(label, recordMs); err != nil {
				_ = rec.Stop()
				return err
			}
			if err := rec.Stop(); err != nil {
				return err
			}

			pcm, derr := rec.Drain(nil)
			if derr != nil {
				return derr
			}

			wavName := fmt.Sprintf("trial_%02d.wav", t+1)
			wavPath := filepath.Join(exp.Data.Directory, wavName)
			if err := stimuli.WriteFloat32WAV(wavPath, pcm, int(outFmt.Freq), int(outFmt.Channels)); err != nil {
				return err
			}
			exp.Data.Add(t+1, wavName, len(pcm), mic.Name)

			if err := exp.Blank(300); err != nil {
				return err
			}
		}

		return exp.ShowEndMessage("Done. Press any key.")
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
