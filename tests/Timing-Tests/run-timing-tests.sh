#!/bin/bash
# Timing-Tests session driver — run once on each machine under test.
#
# Reports are written to reports-<hostname>/ so results from several machines
# can be collected side by side without overwriting each other.
#
# The display and frames tests are each run TWICE: once with the garbage
# collector suspended (the default) and once with -gc, which leaves it running.
# That pair is what yields the GC-on/GC-off comparison.
#
# Usage:
#   ./run-timing-tests.sh                 # run every step, prompting between them
#   ./run-timing-tests.sh display frames  # run only the named steps
#   ./run-timing-tests.sh -l              # list step names and exit
#
# Environment overrides:
#   BIN               binary to run            (default: build ./Timing-Tests)
#   SDL_AUDIODRIVER   audio backend            (default: unset = SDL picks)
#   AUDIO_BUFFSIZE    hardware buffer, frames  (default: 1024 — measured best on
#                                              a Pi 4; see below and
#                                              docs/TimingTests.md)
#   OUTDIR            session directory        (default: reports-<hostname>)
#   TONE_HZ           tone frequency, Hz       (default: 440)
#   SQUARE_PX         stimulus square side, px (default: 0 = ¼ of render height)
#   CYCLES            cycles per av step       (default: 1010 = 1000 + warm-up)
#   WARMUP            leading cycles discarded (default: 10)
#   FRAMES_ON         bright frames per cycle  (default: 12)
#   FRAMES_OFF        dark frames per cycle    (default: 18)
#   DISPLAY_IDX       monitor index for -d     (default: unset = primary)
#   EXCLUSIVE_FS      auto | on | off          (default: auto)
#   TRIGGER_DEVICE    dlpio8 | parallel | gpio | ft232h | labjackt4
#                                              (default: dlpio8)
#   TRIGGER_PIN       output pin 1-8           (default: 1)
#   TRIGGER_MS        TTL pulse width, ms      (default: 200)
#   DLP_PORT          DLP-IO8-G serial port    (default: auto-detect; set it when
#                                              BBTK_CAPTURE=1 — see the note below)
#   PARALLEL_PORT     LPT device path          (default: first accessible one)
#   GPIO_CHIP         chip device path         (default: /dev/gpiochip0)
#   GPIO_PINS         8 pins, comma-separated  (default: 17,27,22,5,6,13,19,26)
#   LABJACK_HOST      T4 address               (required for TRIGGER_DEVICE=labjackt4)
#
# Both TRIGGER_DEVICE=parallel and =gpio write via a local ioctl instead of the
# default's USB serial link, which keeps the link's latency out of the measured
# stimulus-onset-vs-TTL figure — parallel on a desktop with an LPT port, gpio on
# a Raspberry Pi or similar. Wiring and the 3.3 V caveat are described beside
# those variables below. Verify the pins first with `go run ./tests/test_linuxgpio`
# or `go run ./tests/test_parallel_port`.
#
# TRIGGER_DEVICE=ft232h (Adafruit FT232H, USB) and =labjackt4 (LabJack T4, over
# the network) exist for rigs that have neither an LPT port nor a GPIO header;
# both put a link back between the flip and the TTL edge, so prefer parallel or
# gpio where the hardware allows. Verify with `go run ./tests/test_ft232h` or
# `go run ./tests/test_labjackt4`.
#
# DISPLAY_IDX is the index printed by `./Timing-Tests -sysinfo`, NOT an X11/
# Wayland output name. Index 0 is always the primary, which on a laptop is
# usually the built-in panel rather than the monitor under test.
#
# Under Wayland the compositor chooses the output regardless of -d, so display
# selection does not work there: configure the machine with a SINGLE active
# output instead. Verified on GNOME/Mutter 2026-07-29 — see the note in
# apparatus/screen_newscreen_notjs.go.
#
# Optional BBTK recording — when enabled, each photodiode step is run BY
# bbtk-capture, which starts the stimulus itself at the instant the device
# begins recording:
#   BBTK_CAPTURE      set to 1 to enable   (default: off)
#   BBTK_CAPTURE_BIN  path to bbtk-capture (default: bbtk-capture on PATH)
#   BBTK_PORT         serial port          (default: bbtk-capture auto-detects)
#   BBTK_MARGIN_S     recorded margin either side of the stimulus (default: 8)
#
# Each capture costs 11-40 s of device setup before the stimulus can start
# (fixed command pacing, plus a variable internal-memory erase). That instant is
# not predictable, which is why the stimulus is launched from inside
# bbtk-capture (its `-- command` form) rather than alongside it. This script used
# to reconstruct the same instant from outside — background the capture, poll its
# output for a readiness marker, then start the stimulus, then wait for the
# download — which took ~70 lines and three failure modes to get right.
#
# The capture's duration is still computed here, in advance: the device enforces
# the window via TIML and cannot be asked to stop early and hand back what it
# has, so a window that turns out too short means re-running the step.
#
# On a Prime / hybrid-graphics laptop, force the discrete card:
#   __NV_PRIME_RENDER_OFFLOAD=1 __GLX_VENDOR_LIBRARY_NAME=nvidia ./run-timing-tests.sh

set -u
# Without pipefail, `$BIN ... | tee` reports tee's status, so a test that
# crashed would look like it succeeded and leave a truncated report behind.
set -o pipefail

# SDL reads this from the *environment*, so it must be exported — a plain
# assignment never reaches the binary.
#
# Left UNSET by default, so SDL probes and picks a backend that works on this
# machine. This used to force alsa, which suited the reference desktop but on a
# Raspberry Pi running PipeWire produced no sound at all: the device opened, the
# tones went nowhere, no error was reported and the run exited normally
# (diagnosed 2026-08-09). Silence that reports success is the worst failure mode
# available to an audio measurement, and hard-coding one machine's backend is
# the wrong default for a harness whose purpose is comparing machines.
#
# Comparability does not depend on pinning it here: the driver SDL actually
# chose is written into every results file as `sys audio_driver:`. Set
# SDL_AUDIODRIVER (alsa, pipewire, pulseaudio, …) to pin it deliberately.
if [ -n "${SDL_AUDIODRIVER:-}" ]; then
	export SDL_AUDIODRIVER
fi
# 1024 frames (21.3 ms at 48 kHz), chosen on a measured sweep rather than on the
# usual "as small as it will go".
#
# Capturing a Pi 4's line output directly at 100 kS/s (PipeWire, 48 kHz,
# 2026-08-17), at 512 and below the audio tears: 2.0 % of 500 tones at 512,
# 20.7 % of 483 at 256. At 512 it is worse than tearing — after ten glitches the
# server ADDED a buffer period mid-run, stepping the latency +10.85 ms and going
# quiet, so the setting is not one the machine holds. 1024 gave zero torn tones
# in 481 and did not move.
#
# Going higher costs jitter proportionally: the tone lands somewhere in the
# current period, so the onset scatters by period/sqrt(12) — 6.17 ms measured at
# 1024, 12.28 at 2048, both within 0.2 % of prediction. Unlike the latency, that
# scatter cannot be compensated by scheduling the tone earlier.
#
# The floor is the HARDWARE, not the audio server. Bypassing PipeWire entirely
# (SDL_AUDIODRIVER=alsa onto hw:2,0, server stopped) gave the lowest latency
# measured — median 4.95 ms — and tore every one of 463 tones, five times each.
# PipeWire's escalation at 512 was it finding the same floor and working around
# it. So do not reach for a "direct" path to fix tearing; sit above the floor.
#
# The ceiling is how much onset scatter the experiment can carry. Both are per
# machine; this is recorded in each run's -info.txt.
AUDIO_BUFFSIZE="${AUDIO_BUFFSIZE:-1024}"
# 440 Hz, the tone Bridges et al. (2020) used, so the audio row of a session can
# be placed beside their Table 2. Timing-Tests defaults to 1000 Hz and this
# script used to leave it there, which is why every session recorded before
# 2026-08-09 carries a 1 kHz tone rather than their 440 Hz. Frequency does not
# affect onset timing, but it does affect where a microphone or line-out
# threshold is crossed, so the two are not interchangeable in a comparison.
TONE_HZ="${TONE_HZ:-440}"
# Stimulus geometry and length. SQUARE_PX must be large enough that each square's
# centre clears the bezel, or a photodiode cannot sit where the separation figure
# assumes it does; CYCLES is worth lowering for a quick placement check before
# committing to a full pass.
SQUARE_PX="${SQUARE_PX:-0}"   # 0 = Timing-Tests picks ¼ of the render height
# 1010, not 1000: WARMUP cycles are discarded from the statistics, so 1000 asked
# for only 990 analysed trials. Bridges et al. report 1000. The warm-up cycles
# are still PRESENTED — they flash and fire the TTL — so this also lengthens the
# capture window, which av_seconds below accounts for.
#
# WARMUP is passed explicitly rather than left to the binary's default, so the
# "CYCLES − WARMUP = 1000 analysed" arithmetic is a property of this script and
# cannot drift if that default changes.
CYCLES="${CYCLES:-1010}"
WARMUP="${WARMUP:-10}"

# Cycle composition. The defaults (12 on / 18 off = 200/300 ms at 60 Hz) are what
# every recorded session so far used, so leave them alone unless the point of the
# run is to change them.
#
# Shortening the cycle changes what a missed deadline looks like. At 12 frames on,
# a late flip only moves an edge inside a long plateau and shows up as a millisecond
# or two of jitter; at 1 frame on, the flash is instead absent or doubled, which the
# photodiode registers as a missing or double-length event. That makes short cycles
# far more sensitive to anything that stalls the render loop — the garbage collector
# in particular.
#
# Two things to watch when you do shorten it:
#
#   - CYCLES is not exposure. GC pauses arrive per unit time, not per cycle, so
#     1000 cycles of 4 frames is ~67 s against ~500 s for 1000 cycles of 30. Scale
#     CYCLES up to keep the wall-clock comparable, or the shorter run will flatter
#     the collector for no reason other than having sampled less of it.
#   - The panel may not keep up. Opto events run ~20 ms longer than the stimulus
#     (threshold hysteresis plus LCD decay) — more than a single frame at 60 Hz. Pilot
#     any 1-frame configuration with a small CYCLES and check N(Opto1) == N(TTLin1)
#     before committing to a long capture, or you will be measuring the display's
#     response rather than the framework's.
FRAMES_ON="${FRAMES_ON:-12}"
FRAMES_OFF="${FRAMES_OFF:-18}"

for _v in CYCLES FRAMES_ON FRAMES_OFF; do
	eval "_n=\$$_v"
	# shellcheck disable=SC2154  # _n is assigned by the eval above
	case "$_n" in
		'' | *[!0-9]*) echo "error: $_v must be a positive integer (got '$_n')" >&2; exit 1 ;;
	esac
	[ "$_n" -gt 0 ] || { echo "error: $_v must be greater than zero" >&2; exit 1; }
done
unset _v _n

# WARMUP is checked apart from the loop above because zero is a legitimate value
# here — it means "analyse every cycle" — whereas zero cycles or zero frames is
# not. The upper bound is the one that matters: Timing-Tests rejects a warm-up
# that discards every cycle, and catching it here saves opening a fullscreen
# window (and, with BBTK_CAPTURE=1, spending a capture window) to be told so.
case "$WARMUP" in
	'' | *[!0-9]*) echo "error: WARMUP must be a non-negative integer (got '$WARMUP')" >&2; exit 1 ;;
esac
if [ "$WARMUP" -ge "$CYCLES" ]; then
	echo "error: WARMUP=$WARMUP discards all $CYCLES cycles; lower it or raise CYCLES" >&2
	exit 1
fi

# Display and presentation mode. Both are passed to EVERY step that opens a
# window, so a session cannot end up with half its steps on one monitor and half
# on another — which is exactly what happened on 2026-07-29, invisibly, because
# the compositor placed each run by focus.
#
# DISPLAY_IDX is left unset by default rather than defaulting to 0: an explicit
# "primary" is wrong on any machine where the monitor under test is secondary,
# and silently measuring the wrong panel is worse than not passing the flag.
DISPLAY_IDX="${DISPLAY_IDX:-}"
EXCLUSIVE_FS="${EXCLUSIVE_FS:-auto}"

# Assembled once, then appended to every run. Keeping it in one place is what
# guarantees the steps stay comparable.
DISPLAY_ARGS="-exclusive-fullscreen $EXCLUSIVE_FS"
if [ -n "$DISPLAY_IDX" ]; then
	DISPLAY_ARGS="$DISPLAY_ARGS -d $DISPLAY_IDX"
fi

# TTL output device. The default dlpio8 reaches the box over USB serial, which
# puts the link's latency and jitter between the flip and the TTL edge — inside
# the stimulus-onset-vs-TTL figure, not cancelling out of it. Both alternatives
# are a local ioctl and avoid that:
#
#   TRIGGER_DEVICE=parallel   an LPT port via ppdev — the pick on a desktop that
#                             still has one. 5 V logic, so it drives a BBTK or
#                             EEG input directly.
#   TRIGGER_DEVICE=gpio       the GPIO header of a single-board computer
#                             (Raspberry Pi, Rock Pi, …). 3.3 V, see below.
#
# Two more exist for rigs with neither, and neither avoids a link:
#
#   TRIGGER_DEVICE=ft232h     an Adafruit FT232H breakout over USB. 3.3 V.
#   TRIGGER_DEVICE=labjackt4  a LabJack T4 over Modbus TCP — the write crosses
#                             the network. 3.3 V. Needs LABJACK_HOST.
#
# Timing-Tests records which device fired in the results header, because they do
# not yield the same number.
#
# TRIGGER_PIN is 1-8 on all five, but it names something different on each:
# a DLP-IO8 terminal-block number; a parallel data line (pin 1 = D0 = DB25 pin
# 2, ground on DB25 18-25); a position in GPIO_PINS (pin 1 = BCM 17 on the
# defaults); an FT232H D-bus line (pin 1 = AD0); or a position in the T4's
# DIO4-DIO11 (pin 1 = DIO4 = screw terminal FIO4). Timing-Tests prints the
# connector pin it resolves to — probe that.
#
# Pi GPIO swings 0-3.3 V, not 5 V, as do the FT232H and the T4. Verify the
# recorder triggers at 3.3 V on a short run before committing to a long capture.
# Parallel and DLP-IO8 are 5 V.
TRIGGER_DEVICE="${TRIGGER_DEVICE:-dlpio8}"
TRIGGER_PIN="${TRIGGER_PIN:-1}"
DLP_PORT="${DLP_PORT:-}"             # empty = auto-detect by probing every serial port
PARALLEL_PORT="${PARALLEL_PORT:-}"   # empty = first accessible /dev/parportN
GPIO_CHIP="${GPIO_CHIP:-/dev/gpiochip0}"
GPIO_PINS="${GPIO_PINS:-17,27,22,5,6,13,19,26}"
LABJACK_HOST="${LABJACK_HOST:-}"     # no auto-detect: the T4 is reached by address
# 200 ms, matching the TTL pulse Bridges et al. (2020) sent alongside the
# stimulus — the same reason FRAMES_ON/TONE_HZ carry their values rather than
# the binary's. Timing-Tests itself defaults to 5 ms, and this script used to
# leave it there, so every session recorded before 2026-08-09 carries a 5 ms
# pulse. Only the leading edge is used to measure onset, but the width decides
# whether a recorder registers the pulse at all: a threshold-and-debounce input
# that latches a static level can still miss a 5 ms one.
TRIGGER_MS="${TRIGGER_MS:-200}"

# An array, unlike the string $DISPLAY_ARGS above, so the expansion below can be
# quoted: the steps that use it sit inside a `step … && run_recorded …` chain,
# where a `# shellcheck disable=SC2086` directive cannot be placed (it would fall
# mid-command). "${TRIGGER_ARGS[@]}" needs no suppression and cannot word-split a
# path containing a space.
case "$TRIGGER_DEVICE" in
	dlpio8)   TRIGGER_ARGS=(-trigger-device dlpio8 -trigger-pin "$TRIGGER_PIN")
	          # Same "only when set" rule as PARALLEL_PORT below.
	          #
	          # Worth setting under BBTK_CAPTURE=1 even though auto-detect works:
	          # the BBTK is itself a USB serial device, and auto-detect probes
	          # every candidate port in turn by opening it and writing a byte.
	          # Linux does not lock tty devices by default, and bbtk-capture has
	          # the recorder's port open by the time the stimulus starts — so the
	          # probe can write into the recorder's command stream. The DLP
	          # answers a ping so the wrong device is never *selected*, but the
	          # stray byte is sent before that is known. Naming the port skips
	          # the probing entirely.
	          if [ -n "$DLP_PORT" ]; then
	          	TRIGGER_ARGS+=(-port "$DLP_PORT")
	          fi
	          ;;
	parallel) TRIGGER_ARGS=(-trigger-device parallel -trigger-pin "$TRIGGER_PIN")
	          # Only pass -parallel-port when set: empty would be a literal
	          # empty argument, not the "auto-detect" the binary means by it.
	          if [ -n "$PARALLEL_PORT" ]; then
	          	TRIGGER_ARGS+=(-parallel-port "$PARALLEL_PORT")
	          fi
	          ;;
	gpio)     TRIGGER_ARGS=(-trigger-device gpio -trigger-pin "$TRIGGER_PIN"
	                        -gpio-chip "$GPIO_CHIP" -gpio-pins "$GPIO_PINS") ;;
	ft232h)   TRIGGER_ARGS=(-trigger-device ft232h -trigger-pin "$TRIGGER_PIN") ;;
	labjackt4)
	          # Refuse here rather than let Timing-Tests refuse: this script runs
	          # several steps in sequence, and the address is missing for all of
	          # them, so failing at the first is the whole session wasted.
	          if [ -z "$LABJACK_HOST" ]; then
	          	echo "error: TRIGGER_DEVICE=labjackt4 needs LABJACK_HOST (e.g. LABJACK_HOST=192.168.1.100)" >&2
	          	exit 1
	          fi
	          TRIGGER_ARGS=(-trigger-device labjackt4 -trigger-pin "$TRIGGER_PIN"
	                        -labjack-host "$LABJACK_HOST")
	          ;;
	*)        echo "error: TRIGGER_DEVICE must be dlpio8, parallel, gpio, ft232h or labjackt4 (got '$TRIGGER_DEVICE')" >&2; exit 1 ;;
esac

case "$TRIGGER_MS" in
	'' | *[!0-9]*) echo "error: TRIGGER_MS must be a positive integer (got '$TRIGGER_MS')" >&2; exit 1 ;;
esac
[ "$TRIGGER_MS" -gt 0 ] || { echo "error: TRIGGER_MS must be greater than zero" >&2; exit 1; }

TRIGGER_ARGS+=(-trigger-ms "$TRIGGER_MS")

# Overridable so the session plumbing (capture handshake, output paths) can be
# exercised without opening a fullscreen window on someone's display. Supplying
# BIN also suppresses the build below — the caller is then responsible for what
# it points at.
#
# The default is resolved against the script's own directory rather than the
# working directory, so it names the same binary however the script is invoked.
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
if [ -n "${BIN:-}" ]; then
	BIN_PREBUILT=1
else
	BIN="$SCRIPT_DIR/Timing-Tests"
	BIN_PREBUILT=0
fi
HOST=$(hostname -s 2>/dev/null || hostname)
# Everything a session produces goes here, under the directory the script was
# launched from: console reports, the .csv/-info.txt data files (via -outdir),
# and any BBTK captures. Override to keep separate sessions apart.
OUTDIR="${OUTDIR:-reports-${HOST}}"

# Kept before the default is applied, so "unset" and "deliberately 0" stay
# distinguishable — see the photodiode-step guard below.
BBTK_CAPTURE_RAW="${BBTK_CAPTURE-}"
BBTK_CAPTURE="${BBTK_CAPTURE:-0}"
BBTK_CAPTURE_BIN="${BBTK_CAPTURE_BIN:-bbtk-capture}"
BBTK_MARGIN_S="${BBTK_MARGIN_S:-8}"

STEPS="sysinfo check display display-gc av av-gc av-visual latency"

if [ "${1:-}" = "-l" ]; then
	echo "Steps: $STEPS"
	exit 0
fi

# Build the binary this session is about to run.
#
# $BIN is NOT what build-all.sh or `make tests` produce — those write to
# _build/ and never touch this directory. A binary left here by an earlier
# `go build .` is therefore invisible to both, and survives a git pull intact:
# on 2026-08-09 a long debugging session went into a GPIO fault that had already
# been fixed in the tree, because this script kept running the binary from
# before the fix while a freshly built test program worked correctly.
#
# Rebuilding costs nothing when nothing changed (Go caches), and it removes the
# only way this harness can report measurements from code that is not the code
# in the working tree. CGO_ENABLED=0 matches build-all.sh and the Makefile.
if [ "$BIN_PREBUILT" = "0" ]; then
	echo "Building $BIN ..."
	if ! ( cd "$SCRIPT_DIR" && CGO_ENABLED=0 go build -o "$BIN" . ); then
		echo "error: failed to build Timing-Tests" >&2
		echo "       Fix the build before measuring — a session is worth nothing" >&2
		echo "       if it did not come from the code you think it did." >&2
		exit 1
	fi
fi

# The frame period comes from the DISPLAY, via the binary that is about to run
# it, not from a variable someone typed.
#
# This replaces REFRESH_HZ (and the -hz flag it fed). Between them they produced
# two typos in two days: 60.197, a valid float that silently shortened every tone
# by 0.6 ms, and 60.0.197, which the flag parser rejected after bbtk-capture had
# already armed — a nine-minute session lost. The program has always been able to
# read the rate for itself, and now does; -print-frame-ms exposes the same figure
# to this script, honouring -d so a multi-monitor machine is asked about the
# right one.
#
# The value matters here for more than tidiness: it sizes the BBTK capture
# window, and the device enforces that window itself, so an underestimate
# truncates a recording with no way to recover it.
FRAME_MS=$("$BIN" -print-frame-ms $DISPLAY_ARGS 2>/dev/null)
case "$FRAME_MS" in
'' | *[!0-9.]*)
	echo "warning: could not read the frame period from $BIN (-print-frame-ms);" >&2
	echo "         falling back to 60 Hz for capture-window arithmetic. The stimulus" >&2
	echo "         still reads the true rate itself, so only the window is affected." >&2
	FRAME_MS=16.666667
	;;
esac

# A pulse that outlasts its own cycle merges into the next one, leaving the line
# permanently high and the TTL channel uncountable — the trace then has no trial
# boundaries at all. That is a whole capture wasted, so refuse it here rather
# than after the fact. It bites when the cycle is shortened (FRAMES_ON=1,
# FRAMES_OFF=2 is 50 ms at 60 Hz, a quarter of the default pulse).
CYCLE_MS=$(awk -v on="$FRAMES_ON" -v off="$FRAMES_OFF" -v f="$FRAME_MS" \
	'BEGIN { printf "%.1f", (on+off)*f }')
if awk -v t="$TRIGGER_MS" -v c="$CYCLE_MS" 'BEGIN { exit !(t >= c) }'; then
	echo "error: TRIGGER_MS=$TRIGGER_MS ms does not fit in a ${CYCLE_MS} ms cycle" >&2
	echo "       (${FRAMES_ON}+${FRAMES_OFF} frames of ${FRAME_MS} ms); consecutive pulses" >&2
	echo "       would merge and the TTL channel could not be counted." >&2
	echo "       Lower TRIGGER_MS, or lengthen the cycle." >&2
	exit 1
fi

SELECTED="$*"
ANY_SELECTED=$#

# selected <step> — with no command-line arguments, run the default steps only;
# otherwise run exactly the steps named, including the optional ones.
selected() {
	if [ "$ANY_SELECTED" -eq 0 ]; then
		case " $STEPS " in *" $1 "*) return 0 ;; esac
		return 1
	fi
	case " $SELECTED " in *" $1 "*) return 0 ;; esac
	return 1
}

CURRENT_STEP=""

# step <name> <label> — prompt before running, unless the step was deselected.
step() {
	selected "$1" || return 1
	CURRENT_STEP=$1
	printf '\n──── %s : %s ────\n' "$1" "$2"
	read -r -p 'Press Enter to start (Ctrl-C to abort the session) ' _
	return 0
}

FAILED=""

# run <report-name> <args...> — show the command, run it, save the output to
# $OUTDIR/<report-name>.txt *and* keep it visible on the terminal.
#
# A failing test is reported loudly and recorded, but does not abort the
# session: the remaining steps are still worth collecting, and the summary at
# the end says what to redo.
run() {
	name=$1
	shift
	# $DISPLAY_ARGS is deliberately unquoted: it carries two or four separate
	# words, not one argument. It goes on every run so no step can differ.
	echo "+ $BIN $* $DISPLAY_ARGS -outdir $OUTDIR"
	# shellcheck disable=SC2086
	"$BIN" "$@" $DISPLAY_ARGS -outdir "$OUTDIR" 2>&1 | tee "$OUTDIR/${name}.txt"
	rc=$?
	if [ "$rc" -ne 0 ]; then
		printf '\n!! %s FAILED (exit %d) — %s/%s.txt is incomplete\n' \
			"$name" "$rc" "$OUTDIR" "$name" | tee -a "$OUTDIR/${name}.txt"
		# Record the STEP name, not the report name: the step name is what
		# selected() matches, so the re-run hint below is copy-pasteable.
		FAILED="${FAILED} ${CURRENT_STEP}"
	fi
	return 0
}

# run_recorded <report-name> <stimulus-seconds> <args...> — like run, but with
# the BBTK recording around it. Falls back to a plain run when BBTK_CAPTURE is
# not 1, so the same step definition serves both.
#
# bbtk-capture owns the whole step here: it sets the device up, starts the
# stimulus the instant recording begins, and stops the stimulus if it outruns the
# window. Nothing needs coordinating from this side, which is why there is no
# longer a background PID, a readiness poll, a download wait, or an EXIT trap —
# and no way for the stimulus to run against a capture that never started, since
# a bbtk-capture that cannot reach the device exits before launching anything.
#
# The two output streams are kept apart by bbtk-capture: the stimulus gets stdout
# (all of its statistics), the capture reports on stderr. So the report file
# stays the stimulus report it always was, and the capture log holds the device
# side. The stimulus's own stderr — crashes, the Esc notice — is inherited and
# therefore lands in the capture log, next to the device state that explains it.
#
# Both streams are teed rather than merely redirected: a capture is 11-40 s of
# device setup and then several minutes of stimulus, and a silent terminal
# through all of that is indistinguishable from a hang.
run_recorded() {
	name=$1
	rec_dur=$(( $2 + 2 * BBTK_MARGIN_S ))
	shift 2

	if [ "$BBTK_CAPTURE" != "1" ]; then
		run "$name" "$@"
		return 0
	fi

	rec_log="$OUTDIR/bbtk-${name}.log"
	# shellcheck disable=SC2086
	echo "+ $BBTK_CAPTURE_BIN -d $rec_dur $OUTDIR/bbtk-${name} -- $BIN $* $DISPLAY_ARGS -outdir $OUTDIR"
	# $DISPLAY_ARGS is deliberately unquoted: it carries two or four separate
	# words, not one argument.
	# shellcheck disable=SC2086
	"$BBTK_CAPTURE_BIN" -d "$rec_dur" "$OUTDIR/bbtk-${name}" \
		-- "$BIN" "$@" $DISPLAY_ARGS -outdir "$OUTDIR" \
		2> >(tee "$rec_log" >&2) | tee "$OUTDIR/${name}.txt"
	rc=$?
	if [ "$rc" -ne 0 ]; then
		printf '\n!! %s FAILED (exit %d) — see %s and %s/%s.txt\n' \
			"$name" "$rc" "$rec_log" "$OUTDIR" "$name" | tee -a "$OUTDIR/${name}.txt"
		printf '   A non-zero status here means the stimulus failed or was\n'
		printf '   aborted, or the capture could not reach the device. The\n'
		printf '   recording cannot be salvaged either way — re-run the step.\n'
		printf '   If the device could not be opened, set BBTK_PORT;\n'
		printf '   bbtk-detect-port will find it.\n'
		FAILED="${FAILED} ${CURRENT_STEP}"
	fi
	return 0
}

# av_seconds <cycles> — how long an av run of <cycles> takes, in whole seconds.
# One cycle is frames-on + frames-off frames at the display refresh rate.
#
# This must stay in step with the -frames-on/-frames-off actually passed to the
# steps below: it sizes the BBTK capture window, and the device enforces that
# window itself, so an underestimate truncates the recording with no way to
# recover it. Hence FRAMES_ON/FRAMES_OFF rather than the literals this used to
# carry.
av_seconds() {
	awk -v c="$1" -v on="$FRAMES_ON" -v off="$FRAMES_OFF" -v f="$FRAME_MS" \
		'BEGIN { printf "%d", (c * (on + off) * f / 1000) + 1 }'
}

# Computed once so the step labels can quote the real figure rather than the
# 500 s they used to hard-code, which was only ever right at the defaults.
AV_SECONDS=$(av_seconds "$CYCLES")

if [ ! -x "$BIN" ]; then
	echo "error: $BIN not found or not executable — build it first:" >&2
	echo "       (from the repo root)  go build -o tests/Timing-Tests/Timing-Tests ./tests/Timing-Tests" >&2
	exit 1
fi

mkdir -p "$OUTDIR"
echo "Host:    $HOST"
echo "Session: $OUTDIR/  (reports, data files and any BBTK captures)"
echo "Audio:   SDL_AUDIODRIVER=${SDL_AUDIODRIVER:-<unset — SDL picks>}  buffer=$AUDIO_BUFFSIZE frames  tone=${TONE_HZ} Hz"
echo "         (the backend actually opened is recorded as 'sys audio_driver')"
if [ -n "$DISPLAY_IDX" ]; then
	echo "Display: -d $DISPLAY_IDX  (check it against ./Timing-Tests -sysinfo)"
else
	echo "Display: primary (set DISPLAY_IDX to target another monitor)"
fi
echo "         fullscreen mode: $EXCLUSIVE_FS  (recorded as sys fullscreen_mode)"
case "$TRIGGER_DEVICE" in
	gpio)
		echo "Trigger: gpio $GPIO_CHIP pins=$GPIO_PINS, -trigger-pin $TRIGGER_PIN, ${TRIGGER_MS} ms pulse"
		echo "         (3.3 V lines — confirm the recorder triggers at 3.3 V on a short run)"
		;;
	parallel)
		echo "Trigger: parallel ${PARALLEL_PORT:-(first accessible /dev/parportN)}, -trigger-pin $TRIGGER_PIN, ${TRIGGER_MS} ms pulse"
		;;
	ft232h)
		echo "Trigger: ft232h (USB, MPSSE), -trigger-pin $TRIGGER_PIN = AD$((TRIGGER_PIN - 1)), ${TRIGGER_MS} ms pulse"
		echo "         (3.3 V lines — confirm the recorder triggers at 3.3 V on a short run)"
		;;
	labjackt4)
		echo "Trigger: labjackt4 $LABJACK_HOST (Modbus TCP), -trigger-pin $TRIGGER_PIN = DIO$((TRIGGER_PIN + 3)), ${TRIGGER_MS} ms pulse"
		echo "         (3.3 V lines, and the write crosses the network — confirm both on a short run)"
		;;
	*)
		echo "Trigger: dlpio8 (USB serial), -trigger-pin $TRIGGER_PIN, ${TRIGGER_MS} ms pulse"
		;;
esac
if [ "$SQUARE_PX" -eq 0 ]; then
	echo "Stim:    squares sized to ¼ of the render height, $CYCLES cycles per av step"
else
	echo "Stim:    ${SQUARE_PX} px squares, $CYCLES cycles per av step"
fi
echo "         ($CYCLES presented, first $WARMUP discarded as warm-up: $((CYCLES - WARMUP)) analysed)"
# Print the cycle in frames AND milliseconds: the frame counts are what the flags
# carry, but the millisecond figures are what a reader of the report needs, and
# they depend on the frame period, which is read from the display.
awk -v on="$FRAMES_ON" -v off="$FRAMES_OFF" -v f="$FRAME_MS" -v secs="$AV_SECONDS" \
	'BEGIN { printf "         cycle: %d on + %d off = %d frames (%.1f + %.1f = %.1f ms at %.4f Hz, from the display)\n         av step duration: %d s per run\n", \
		on, off, on+off, on*f, off*f, (on+off)*f, 1000/f, secs }'
if [ "$FRAMES_ON" -le 2 ]; then
	echo "         NOTE: frames-on=$FRAMES_ON is at or below the panel's response time on"
	echo "               typical LCDs — verify N(Opto1) == N(TTLin1) on a short pilot"
	echo "               before trusting a long capture at this setting."
fi
# Say which onset path the run will use, and refuse a value that would silently
# select the wrong one.
#
# The library accepts exactly "on"; anything else — ON, 1, true, yes — reads as
# unset and gives the default, which is indistinguishable from the experimental
# arm until the info file is read eight minutes later. An A/B whose two arms both
# ran the default looks like a null result rather than a switch that never took,
# so this is checked before the device is touched rather than diagnosed after.
case "${GOXPY_VBLANK:-}" in
	'')   echo "Vblank:  onsets from the present's return, or the pacing schedule on"
	      echo "         frames where the driver did not block (default) — each run's"
	      echo "         'sys pacing:' line records which it turned out to be" ;;
	on)   echo "Vblank:  onsets anchored on kernel vblank stamps (GOXPY_VBLANK=on — experimental)" ;;
	*)    echo "error: GOXPY_VBLANK must be 'on' or unset (got '$GOXPY_VBLANK')" >&2
	      echo "       Any other value is treated as unset by the library, so this run" >&2
	      echo "       would silently use the default path and be indistinguishable" >&2
	      echo "       from a baseline arm." >&2
	      exit 1 ;;
esac

# Reject a value that reads as unset, exactly as GOXPY_VBLANK above does:
# anything that is not 1 disables recording, so BBTK_CAPTURE=yes or =true would
# quietly produce a session with no photodiode data and no complaint.
case "$BBTK_CAPTURE_RAW" in
'' | 0 | 1) ;;
*)
	echo "error: BBTK_CAPTURE must be 0 or 1 (got '$BBTK_CAPTURE_RAW')" >&2
	echo "       Any other value disables recording, so this session would run" >&2
	echo "       the photodiode steps with nothing capturing them." >&2
	exit 1
	;;
esac

if [ "$BBTK_CAPTURE" = "1" ]; then
	echo "BBTK:    recording enabled ($BBTK_CAPTURE_BIN, ${BBTK_MARGIN_S}s margin either side)"
	if [ -n "${BBTK_PORT:-}" ]; then
		echo "         BBTK_PORT=$BBTK_PORT"
	else
		echo "         port: auto-detected by bbtk-capture (set BBTK_PORT to override)"
	fi
	if ! command -v "$BBTK_CAPTURE_BIN" >/dev/null 2>&1 && [ ! -x "$BBTK_CAPTURE_BIN" ]; then
		echo "error: BBTK_CAPTURE=1 but '$BBTK_CAPTURE_BIN' is not executable or not on PATH" >&2
		echo "       set BBTK_CAPTURE_BIN to its full path, or unset BBTK_CAPTURE" >&2
		exit 1
	fi
	# Check for -- support before touching the device. A bbtk-capture built
	# before it would read "--" as a stray argument and reject the whole command
	# line, and version skew has bitten this script before; a clear message here
	# beats "a basefilename argument is required".
	if ! "$BBTK_CAPTURE_BIN" -h 2>&1 | grep -q -- '-- command'; then
		echo "error: '$BBTK_CAPTURE_BIN' cannot launch the stimulus itself" >&2
		echo "       its usage does not mention '-- command', so it predates the" >&2
		echo "       change that lets a capture run the stimulus inside its own" >&2
		echo "       window." >&2
		echo "       Rebuild bbtkv3 (make build) and point BBTK_CAPTURE_BIN at" >&2
		echo "       the new binary — check with: $BBTK_CAPTURE_BIN -h" >&2
		exit 1
	fi
else
	# A photodiode step with nothing recording cannot answer the question it
	# exists for, and the cost of finding out afterwards is the whole session:
	# av at the default 1010 cycles is ~8.5 minutes per step, and this preamble
	# has scrolled well out of view by the time the stimulus starts. A Pi 4
	# session was lost exactly this way on 2026-08-16.
	#
	# Unset and explicitly-0 are treated differently, on the same principle as
	# the GOXPY_VBLANK check above: "software-side only, deliberately" is a
	# decision worth honouring, "I forgot to export it" is not, and only the
	# second is worth stopping. Steps that need no photodiode are unaffected.
	NEEDS_BBTK=""
	for _s in av av-gc av-visual; do
		selected "$_s" && NEEDS_BBTK="$NEEDS_BBTK $_s"
	done
	if [ -n "$NEEDS_BBTK" ] && [ "$BBTK_CAPTURE_RAW" != "0" ]; then
		echo "error: these steps record nothing without the BBTK:$NEEDS_BBTK" >&2
		echo "       They fire a TTL and flash a patch for the photodiode to see," >&2
		echo "       so with no capture running the session produces software" >&2
		echo "       timestamps only — which cannot be checked against photons." >&2
		echo "" >&2
		echo "       BBTK_CAPTURE=1 ...   record the photodiode steps" >&2
		echo "       BBTK_CAPTURE=0 ...   run them software-side anyway (deliberate)" >&2
		echo "" >&2
		echo "       No photodiode on this machine? Name the steps that need none:" >&2
		echo "         ./run-timing-tests.sh sysinfo check display display-gc latency" >&2
		exit 1
	fi
	echo "BBTK:    recording disabled (set BBTK_CAPTURE=1 to record the photodiode steps)"
fi

# ── Tier 0: sanity check — catch a dead display or silent audio before
#            committing to the long measurements.
step sysinfo "system information" &&
	run sysinfo -sysinfo

step check "display flash + audio sanity check" &&
	run check -test check -audio-frames "$AUDIO_BUFFSIZE"

# ── No hardware: frame-interval distributions, GC suspended vs. GC running.
#                 This pair is the garbage-collector comparison.
step display "frame intervals, GC SUSPENDED (300 s)" &&
	run display-gc-off -test display -duration-s 300

step display-gc "frame intervals, GC RUNNING (300 s)" &&
	run display-gc-on -test display -duration-s 300 -gc

# ── No hardware: audio pipeline drain time at the chosen buffer size.
step latency "audio pipeline latency" &&
	run latency -test latency -audio-frames "$AUDIO_BUFFSIZE" -drain-reps 20

# ── Photodiode + TTL: the main stimulus timing measurement, run as a
#                     GC-suspended / GC-running pair. Put photodiodes on a top
#                     and a bottom square to also capture the scan-out gradient.
step av "visual+audio+TTL, GC SUSPENDED (${AV_SECONDS} s)" &&
	run_recorded av-gc-off "$AV_SECONDS" \
		-test av -audio-frames "$AUDIO_BUFFSIZE" \
		-freq-hz "$TONE_HZ" "${TRIGGER_ARGS[@]}" \
		-frames-on "$FRAMES_ON" -frames-off "$FRAMES_OFF" \
		-cycles "$CYCLES" -warmup "$WARMUP" -square-px "$SQUARE_PX"

step av-gc "visual+audio+TTL, GC RUNNING (${AV_SECONDS} s)" &&
	run_recorded av-gc-on "$AV_SECONDS" \
		-test av -audio-frames "$AUDIO_BUFFSIZE" \
		-freq-hz "$TONE_HZ" "${TRIGGER_ARGS[@]}" \
		-frames-on "$FRAMES_ON" -frames-off "$FRAMES_OFF" \
		-cycles "$CYCLES" -warmup "$WARMUP" -square-px "$SQUARE_PX" -gc

# ── Photodiode only: visual path in isolation, no audio device, no trigger.
#                    Isolates the display when the combined run looks wrong.
step av-visual "visual only, no audio, no TTL (${AV_SECONDS} s)" &&
	run_recorded av-visual "$AV_SECONDS" \
		-test av -no-sound -no-ttl \
		-frames-on "$FRAMES_ON" -frames-off "$FRAMES_OFF" \
		-cycles "$CYCLES" -warmup "$WARMUP" -square-px "$SQUARE_PX"

# ── Response timing. COMMENTED OUT until the BBTK response actuator
#            arrives (expected ~mid-August 2026).
#
# This test needs a device that presses a key at a *known* delay after the
# flash — BBTK response actuators, or a USB-HID microcontroller. Pressing keys
# by hand measures the experimenter, not the framework, so there is no ground
# truth to compare against.
#
# To re-enable: add "rt" back to STEPS above and uncomment the two lines below.
# Keep this session's frames-gc-off.txt — the rt numbers are interpreted against
# that photodiode onset to get the full stimulus-onset → keypress chain.
#
# step rt "reaction-time accuracy, actuator-generated responses (~120 s)" &&
# 	run rt -test rt -cycles 120 -iti-ms 1000

printf '\nDone. Reports in %s/\n' "$OUTDIR"
ls -1 "$OUTDIR"

if [ -n "$FAILED" ]; then
	printf '\n!! These steps FAILED and must be re-run:%s\n' "$FAILED"
	printf '   e.g.  %s%s\n' "$0" "$FAILED"
	exit 1
fi
printf '\nAll steps completed successfully.\n'
