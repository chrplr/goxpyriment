// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// DLP-IO8-G precision test — characterises the trigger box itself.
//
// Emits a square wave on one DLP-IO8-G output pin for a fixed duration and
// reports how far each edge landed from its target time. No display measurement
// is involved: the window only shows a status line, and the numbers describe the
// serial device and the host's scheduling, not the screen.
//
// Measure the output with an oscilloscope on the chosen pin. The reported jitter
// is the host-side error (when the write was issued relative to target); the
// scope shows the device-side error (when the pin actually changed). The
// difference between the two is the DLP-IO8-G's own latency.
//
// Interpreting the result
//
//	rising/falling jitter SD ≲ 1 ms   host scheduling is adequate for TTL marking
//	occasional multi-ms outliers      normal on a non-realtime kernel
//	systematic offset on the scope    the device's fixed latency; calibrate it out
//
// See docs/SettingPriorityUnderLinux.md if the jitter is larger than expected.
//
// Usage:
//
//	go run ./tests/test_dlpio8                                  # 100 ms period, 50 % duty, 30 s
//	go run ./tests/test_dlpio8 -period-ms 20 -duty 25
//	go run ./tests/test_dlpio8 -port /dev/ttyUSB0 -trigger-pin 3
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/tests/internal/timingstats"
	"github.com/chrplr/goxpyriment/triggers"
)

var (
	fPort       = flag.String("port", "", "Serial port for DLP-IO8-G (empty = auto-detect)")
	fTriggerPin = flag.Int("trigger-pin", 1, "DLP-IO8-G output pin (1–8)")
	fPeriodMs   = flag.Float64("period-ms", 100, "Square-wave period in ms")
	fDuty       = flag.Float64("duty", 50, "Duty cycle 0–100 %")
	fDurationS  = flag.Float64("duration-s", 30, "Duration of square-wave output in seconds")
	fWindowed   = flag.Bool("w", false, "Windowed mode (1024×768 window instead of fullscreen)")
	fDisplay    = flag.Int("d", -1, "Display index (-1 = primary)")
)

// setupTrigger opens the DLP-IO8-G, by explicit port or auto-detection.
func setupTrigger() (triggers.OutputTTLDevice, string) {
	if *fPort != "" {
		d, err := triggers.NewDLPIO8(*fPort)
		if err != nil {
			log.Printf("warning: DLP-IO8 on %s: %v — triggers disabled", *fPort, err)
			return triggers.NullOutputTTLDevice{}, ""
		}
		return d, *fPort
	}
	trig, portName, err := triggers.AutoDetectDLPIO8()
	if err != nil {
		log.Printf("warning: DLP-IO8 auto-detect: %v — triggers disabled", err)
	}
	return trig, portName
}

// sleepUntil sleeps until the given absolute time, with a sub-millisecond
// busy-spin for the last 500 µs to reduce overshoot on Linux.
func sleepUntil(t time.Time) {
	remaining := time.Until(t)
	if remaining > 500*time.Microsecond {
		time.Sleep(remaining - 500*time.Microsecond)
	}
	for time.Now().Before(t) {
		// busy-spin
	}
}

func main() {
	flag.Parse()

	trig, portName := setupTrigger()
	if _, isNull := trig.(triggers.NullOutputTTLDevice); isNull {
		log.Fatal("test_dlpio8 requires a DLP-IO8-G (no device found).\n" +
			"Pass -port /dev/ttyUSBn explicitly, or check the device is connected.")
	}
	fmt.Printf("DLP-IO8-G found on %s (pin %d)\n", portName, *fTriggerPin)

	width, height, fullscreen := 0, 0, true
	if *fWindowed {
		width, height, fullscreen = 1024, 768, false
	}
	exp := control.NewExperiment("DLP-IO8 Precision Test", width, height, fullscreen,
		control.Black, control.White, 24)
	if *fDisplay >= 0 {
		exp.ScreenNumber = *fDisplay
	}
	if err := exp.Initialize(); err != nil {
		exp.End()
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// Save data and drop the pin on Ctrl-C. Do not call exp.End here: it reaches
	// SDL through CGo, and the main goroutine may be inside an SDL call.
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		_ = trig.SetLow(*fTriggerPin)
		if exp.Data != nil {
			exp.Data.WriteEndTime()
			if err := exp.Data.Save(); err == nil {
				log.Printf("Results saved in %s", exp.Data.FullPath)
			}
		}
		os.Exit(0)
	}()

	period := time.Duration(*fPeriodMs * float64(time.Millisecond))
	highDur := time.Duration(float64(period) * *fDuty / 100.0)
	totalDur := time.Duration(*fDurationS * float64(time.Second))
	expectedCycles := int(totalDur / period)

	fmt.Printf("square: period=%.1f ms  duty=%.0f %%  pin=%d  duration=%.0f s  (~%d cycles)\n",
		*fPeriodMs, *fDuty, *fTriggerPin, *fDurationS, expectedCycles)

	exp.Data.WriteComment(fmt.Sprintf("test=dlpio8 period-ms=%.1f duty=%.0f pin=%d duration-s=%.0f",
		*fPeriodMs, *fDuty, *fTriggerPin, *fDurationS))
	exp.AddDataVariableNames([]string{
		"cycle", "edge", "target_ms", "actual_ms", "jitter_ms",
	})

	var riseJitter, fallJitter []float64

	err := exp.Run(func() error {
		status := stimuli.NewTextLine(
			fmt.Sprintf("Square wave: %.1f ms period, %.0f%% duty, pin %d — press ESC to stop",
				*fPeriodMs, *fDuty, *fTriggerPin),
			0, 0, control.White)
		if err := exp.Show(status); err != nil {
			return err
		}

		start := time.Now()
		deadline := start.Add(totalDur)

		for cycle := 0; time.Now().Before(deadline); cycle++ {
			// ── Rising edge ────────────────────────────────────────────────
			targetRise := start.Add(time.Duration(cycle) * period)
			sleepUntil(targetRise)
			tRise := time.Now()
			if err := trig.SetHigh(*fTriggerPin); err != nil {
				return err
			}
			jRise := tRise.Sub(targetRise).Seconds() * 1000
			riseJitter = append(riseJitter, jRise)
			exp.Data.Add(cycle, "rise",
				fmt.Sprintf("%.3f", targetRise.Sub(start).Seconds()*1000),
				fmt.Sprintf("%.3f", tRise.Sub(start).Seconds()*1000),
				fmt.Sprintf("%.3f", jRise))

			// ── Falling edge ───────────────────────────────────────────────
			targetFall := targetRise.Add(highDur)
			sleepUntil(targetFall)
			tFall := time.Now()
			if err := trig.SetLow(*fTriggerPin); err != nil {
				return err
			}
			jFall := tFall.Sub(targetFall).Seconds() * 1000
			fallJitter = append(fallJitter, jFall)
			exp.Data.Add(cycle, "fall",
				fmt.Sprintf("%.3f", targetFall.Sub(start).Seconds()*1000),
				fmt.Sprintf("%.3f", tFall.Sub(start).Seconds()*1000),
				fmt.Sprintf("%.3f", jFall))

			if exp.PollEvents(nil).QuitRequested {
				break
			}

			// Sleep out the low phase so the loop stays near-idle.
			nextRise := targetRise.Add(period)
			if slack := time.Until(nextRise) - 2*time.Millisecond; slack > 0 {
				time.Sleep(slack)
			}
		}

		_ = trig.SetLow(*fTriggerPin)
		timingstats.PrintStats("Rising-edge jitter (ms from target)", timingstats.ComputeStats(riseJitter, 0), 0)
		timingstats.PrintStats("Falling-edge jitter (ms from target)", timingstats.ComputeStats(fallJitter, 0), 0)
		return control.EndLoop
	})
	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("test error: %v", err)
	}
}
