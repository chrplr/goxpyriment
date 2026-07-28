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
#   SDL_AUDIODRIVER   audio backend            (default: alsa)
#   AUDIO_BUFFSIZE    hardware buffer, frames  (default: 256)
#   REFRESH_HZ        expected refresh rate    (default: 60)
#   OUTDIR            session directory        (default: reports-<hostname>)
#
# Optional BBTK recording — when enabled, each photodiode step starts a
# bbtk-capture in the background, waits until the device is actually recording,
# runs the stimulus inside that window, and waits for the capture to save:
#   BBTK_CAPTURE          set to 1 to enable   (default: off)
#   BBTK_CAPTURE_BIN      path to bbtk-capture (default: bbtk-capture on PATH)
#   BBTK_PORT             serial port          (default: auto-detected)
#   BBTK_MARGIN_S         recorded margin either side of the stimulus (default: 8)
#   BBTK_READY_TIMEOUT_S  give up waiting for the device      (default: 120)
#
# Each capture costs 11-40 s of device setup before the stimulus can start
# (fixed command pacing, plus a variable internal-memory erase), which is why
# the script waits for the device rather than guessing.
#
# On a Prime / hybrid-graphics laptop, force the discrete card:
#   __NV_PRIME_RENDER_OFFLOAD=1 __GLX_VENDOR_LIBRARY_NAME=nvidia ./run-timing-tests.sh

set -u
# Without pipefail, `$BIN ... | tee` reports tee's status, so a test that
# crashed would look like it succeeded and leave a truncated report behind.
set -o pipefail

# SDL reads these from the *environment*, so they must be exported — a plain
# assignment never reaches the binary.
export SDL_AUDIODRIVER="${SDL_AUDIODRIVER:-alsa}"
AUDIO_BUFFSIZE="${AUDIO_BUFFSIZE:-256}"
REFRESH_HZ="${REFRESH_HZ:-60}"

BIN=./Timing-Tests
HOST=$(hostname -s 2>/dev/null || hostname)
# Everything a session produces goes here, under the directory the script was
# launched from: console reports, the .csv/-info.txt data files (via -outdir),
# and any BBTK captures. Override to keep separate sessions apart.
OUTDIR="${OUTDIR:-reports-${HOST}}"

BBTK_CAPTURE="${BBTK_CAPTURE:-0}"
BBTK_CAPTURE_BIN="${BBTK_CAPTURE_BIN:-bbtk-capture}"
BBTK_MARGIN_S="${BBTK_MARGIN_S:-8}"
BBTK_READY_TIMEOUT_S="${BBTK_READY_TIMEOUT_S:-120}"
# Emitted by bbtk-capture the instant the device starts recording. Anything
# earlier in its output (including its own "Capturing events..." line) is
# several seconds premature.
BBTK_READY_MARKER="BBTK-CAPTURE-READY"
BBTK_PID=""

STEPS="sysinfo check display display-gc av av-gc av-visual latency"

if [ "${1:-}" = "-l" ]; then
	echo "Steps: $STEPS"
	exit 0
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
	echo "+ $BIN $* -outdir $OUTDIR"
	"$BIN" "$@" -outdir "$OUTDIR" 2>&1 | tee "$OUTDIR/${name}.txt"
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

# ── Optional BBTK recording ───────────────────────────────────────────────
#
# bbtk_resolve_port — set BBTK_PORT if the caller did not.
#
# /dev/ttyUSBn numbering is assigned in enumeration order, so it changes whenever
# the BBTK is replugged or power-cycled — exactly when you least want to be
# editing a command line. The by-id symlink is derived from the USB descriptors
# and is stable across both. Prefer it, and fall back to bbtk-detect-port (which
# has to talk to each candidate port to identify it).
bbtk_resolve_port() {
	[ -n "${BBTK_PORT:-}" ] && return 0
	for link in /dev/serial/by-id/*BBTK*; do
		[ -e "$link" ] || continue
		BBTK_PORT="$link"
		export BBTK_PORT
		return 0
	done
	# Prefer the bbtk-detect-port sitting beside bbtk-capture: it is from the
	# same build, whereas one on PATH may be an older install.
	detector="$(dirname "$BBTK_CAPTURE_BIN")/bbtk-detect-port"
	[ -x "$detector" ] || detector=$(command -v bbtk-detect-port 2>/dev/null)
	if [ -n "$detector" ] && [ -x "$detector" ]; then
		detected=$("$detector" 2>/dev/null | sed -n 's/^BBTK found at //p' | head -1)
		if [ -n "$detected" ]; then
			BBTK_PORT="$detected"
			export BBTK_PORT
			return 0
		fi
	fi
	return 1
}

# capture_start <name> <stimulus-seconds> — launch bbtk-capture and BLOCK until
# the device is actually recording. No-op unless BBTK_CAPTURE=1.
#
# The wait is the whole point. bbtk-capture needs 11-40 s between launch and the
# device recording (fixed command pacing plus a variable internal-memory erase),
# and its own progress output goes quiet several seconds BEFORE that instant, so
# the only safe synchronisation is the marker it prints right after sending RUDS.
#
# stdin is closed deliberately: bbtk-capture puts the terminal into raw mode to
# watch for Esc, and that terminal is shared with Timing-Tests. With </dev/null
# the raw-mode call fails harmlessly and the stimulus keeps its own input.
capture_start() {
	BBTK_PID=""
	[ "$BBTK_CAPTURE" = "1" ] || return 0

	cap_name=$1
	cap_dur=$(( $2 + 2 * BBTK_MARGIN_S ))
	cap_log="$OUTDIR/bbtk-${cap_name}.log"
	: >"$cap_log"

	echo "+ $BBTK_CAPTURE_BIN -d $cap_dur ${OUTDIR}/bbtk-${cap_name}"
	"$BBTK_CAPTURE_BIN" -d "$cap_dur" -no-countdown "$OUTDIR/bbtk-${cap_name}" \
		</dev/null >"$cap_log" 2>&1 &
	BBTK_PID=$!

	printf 'waiting for the BBTK to start recording (up to %ss)' "$BBTK_READY_TIMEOUT_S"
	cap_waited=0
	while ! grep -q "$BBTK_READY_MARKER" "$cap_log" 2>/dev/null; do
		if ! kill -0 "$BBTK_PID" 2>/dev/null; then
			printf '\n!! bbtk-capture exited before recording started — see %s\n' "$cap_log"
			tail -n 5 "$cap_log" 2>/dev/null
			printf '   If it could not open the serial port, set BBTK_PORT.\n'
			printf '   bbtk-detect-port will find the device.\n' 
			FAILED="${FAILED} ${CURRENT_STEP}"
			BBTK_PID=""
			return 1
		fi
		if [ "$cap_waited" -ge "$BBTK_READY_TIMEOUT_S" ]; then
			printf '\n!! BBTK did not report %s within %ss — killing capture\n' \
				"$BBTK_READY_MARKER" "$BBTK_READY_TIMEOUT_S"
			printf '   (the device may be recording anyway; see %s)\n' "$cap_log"
			kill "$BBTK_PID" 2>/dev/null
			wait "$BBTK_PID" 2>/dev/null
			FAILED="${FAILED} ${CURRENT_STEP}"
			BBTK_PID=""
			return 1
		fi
		sleep 1
		cap_waited=$(( cap_waited + 1 ))
		printf '.'
	done
	printf ' recording (%ss window, stimulus starts now)\n' "$cap_dur"
	return 0
}

# capture_wait — block until the running capture has downloaded and saved.
# Files appear only when bbtk-capture exits, so this must complete before the
# next step's prompt or the results look missing.
capture_wait() {
	[ -n "$BBTK_PID" ] || return 0
	echo "waiting for the BBTK to download and save..."
	if ! wait "$BBTK_PID"; then
		printf '!! bbtk-capture failed — see %s/bbtk-*.log\n' "$OUTDIR"
		FAILED="${FAILED} ${CURRENT_STEP}"
	fi
	BBTK_PID=""
	return 0
}

# A capture left running past the end of the script would be orphaned mid-record
# and lose everything, so wait it out rather than exiting from under it.
# bbtk-capture traps SIGINT itself, so a Ctrl-C reaching both still saves.
cleanup() {
	if [ -n "$BBTK_PID" ]; then
		echo "waiting for the running BBTK capture to save before exiting..."
		wait "$BBTK_PID" 2>/dev/null
	fi
}
trap cleanup EXIT

# av_seconds <cycles> — how long an av run of <cycles> takes, in whole seconds.
# One cycle is frames-on + frames-off frames at the display refresh rate.
av_seconds() {
	awk -v c="$1" -v on=12 -v off=18 -v hz="$REFRESH_HZ" \
		'BEGIN { printf "%d", (c * (on + off) / hz) + 1 }'
}

if [ ! -x "$BIN" ]; then
	echo "error: $BIN not found or not executable — build it first:" >&2
	echo "       (from the repo root)  go build -o tests/Timing-Tests/Timing-Tests ./tests/Timing-Tests" >&2
	exit 1
fi

mkdir -p "$OUTDIR"
echo "Host:    $HOST"
echo "Session: $OUTDIR/  (reports, data files and any BBTK captures)"
echo "Audio:   SDL_AUDIODRIVER=$SDL_AUDIODRIVER  buffer=$AUDIO_BUFFSIZE frames"
if [ "$BBTK_CAPTURE" = "1" ]; then
	echo "BBTK:    recording enabled ($BBTK_CAPTURE_BIN, ${BBTK_MARGIN_S}s margin either side)"
	if bbtk_resolve_port; then
		echo "         BBTK_PORT=$BBTK_PORT"
	else
		echo "error: cannot find the BBTK's serial port" >&2
		echo "       no /dev/serial/by-id/*BBTK* symlink, and bbtk-detect-port found nothing." >&2
		echo "       Check the device is connected and powered, then set BBTK_PORT by hand." >&2
		exit 1
	fi
	if ! command -v "$BBTK_CAPTURE_BIN" >/dev/null 2>&1 && [ ! -x "$BBTK_CAPTURE_BIN" ]; then
		echo "error: BBTK_CAPTURE=1 but '$BBTK_CAPTURE_BIN' is not executable or not on PATH" >&2
		echo "       set BBTK_CAPTURE_BIN to its full path, or unset BBTK_CAPTURE" >&2
		exit 1
	fi
	# Probe for handshake support before touching the device. A bbtk-capture
	# built before the marker existed never prints it, so without this check the
	# first recorded step would simply hang until BBTK_READY_TIMEOUT_S expires.
	if ! "$BBTK_CAPTURE_BIN" -V 2>/dev/null | grep -q "$BBTK_READY_MARKER"; then
		echo "error: '$BBTK_CAPTURE_BIN' does not support the capture handshake" >&2
		echo "       its -V output does not advertise $BBTK_READY_MARKER, so it" >&2
		echo "       predates the change that added it and would never report" >&2
		echo "       when the device starts recording." >&2
		echo "       Rebuild bbtkv3 (make build) and point BBTK_CAPTURE_BIN at" >&2
		echo "       the new binary — check with: $BBTK_CAPTURE_BIN -V" >&2
		exit 1
	fi
else
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
step av "visual+audio+TTL, GC SUSPENDED (500 s)" && {
	# Skip the stimulus if the capture never started: a recorded step without a
	# recording is 500 s spent producing data nothing can be correlated against.
	if capture_start av-gc-off "$(av_seconds 1000)"; then
		run av-gc-off -test av -audio-frames "$AUDIO_BUFFSIZE" -hz "$REFRESH_HZ" \
			-frames-on 12 -frames-off 18 -cycles 1000
		capture_wait
	fi
}

step av-gc "visual+audio+TTL, GC RUNNING (500 s)" && {
	# Skip the stimulus if the capture never started: a recorded step without a
	# recording is 500 s spent producing data nothing can be correlated against.
	if capture_start av-gc-on "$(av_seconds 1000)"; then
		run av-gc-on -test av -audio-frames "$AUDIO_BUFFSIZE" -hz "$REFRESH_HZ" \
			-frames-on 12 -frames-off 18 -cycles 1000 -gc
		capture_wait
	fi
}

# ── Photodiode only: visual path in isolation, no audio device, no trigger.
#                    Isolates the display when the combined run looks wrong.
step av-visual "visual only, no audio, no TTL (500 s)" && {
	# Skip the stimulus if the capture never started: a recorded step without a
	# recording is 500 s spent producing data nothing can be correlated against.
	if capture_start av-visual "$(av_seconds 1000)"; then
		run av-visual -test av -no-sound -no-ttl \
			-frames-on 12 -frames-off 18 -cycles 1000
		capture_wait
	fi
}

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
