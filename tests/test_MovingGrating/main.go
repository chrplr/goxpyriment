// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
package main

import (
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	nTrials         = 10
	gratingDiameter = 300
	spatialFreq     = 0.02 // cycles per pixel
	temporalFreq    = 2.0  // Hz
	contrast        = 1.0
	bgLuminance     = 0.5
	trialMs         = 2000 // max display time; SPACE ends the trial early
	isiMs           = 500
)

func main() {
	exp := control.NewExperimentFromFlags("Moving Grating Test", control.Black, control.White, 32)
	defer exp.End()

	// nTrials/2 trials at +45°, nTrials/2 at -45°, order shuffled.
	orientations := make([]float64, nTrials)
	for i := range orientations {
		if i < nTrials/2 {
			orientations[i] = 45
		} else {
			orientations[i] = -45
		}
	}
	design.ShuffleList(orientations)

	for i, orientation := range orientations {
		result, err := stimuli.PresentMovingGratingDisk(
			exp.Screen,
			gratingDiameter,
			control.Origin(),
			orientation,
			spatialFreq,
			temporalFreq,
			contrast,
			bgLuminance,
			trialMs,
			[]control.Keycode{control.K_SPACE},
			false,
		)
		if control.IsEndLoop(err) {
			break
		}
		if err != nil {
			log.Fatalf("trial %d: PresentMovingGratingDisk: %v", i+1, err)
		}
		log.Printf("trial %d: orientation=%+.0f° key=%v RT=%dms", i+1, orientation, result.Key, result.RTms)
		exp.Blank(isiMs)
	}
}
