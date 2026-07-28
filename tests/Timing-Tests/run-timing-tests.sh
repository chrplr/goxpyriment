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
#
# On a Prime / hybrid-graphics laptop, force the discrete card:
#   __NV_PRIME_RENDER_OFFLOAD=1 __GLX_VENDOR_LIBRARY_NAME=nvidia ./run-timing-tests.sh
#
# On a Raspberry Pi, fullscreen needs the software renderer (see CLAUDE.md):
#   SDL_RENDER_DRIVER=software SDL_VIDEODRIVER=wayland ./run-timing-tests.sh

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
OUTDIR="reports-${HOST}"

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
	echo "+ $BIN $*"
	"$BIN" "$@" 2>&1 | tee "$OUTDIR/${name}.txt"
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

if [ ! -x "$BIN" ]; then
	echo "error: $BIN not found or not executable — build it first:" >&2
	echo "       (from the repo root)  go build -o tests/Timing-Tests/Timing-Tests ./tests/Timing-Tests" >&2
	exit 1
fi

mkdir -p "$OUTDIR"
echo "Host:    $HOST"
echo "Reports: $OUTDIR/"
echo "Audio:   SDL_AUDIODRIVER=$SDL_AUDIODRIVER  buffer=$AUDIO_BUFFSIZE frames"

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
step av "visual+audio+TTL, GC SUSPENDED (500 s)" &&
	run av-gc-off -test av -audio-frames "$AUDIO_BUFFSIZE" -hz "$REFRESH_HZ" \
		-frames-on 12 -frames-off 18 -cycles 1000

step av-gc "visual+audio+TTL, GC RUNNING (500 s)" &&
	run av-gc-on -test av -audio-frames "$AUDIO_BUFFSIZE" -hz "$REFRESH_HZ" \
		-frames-on 12 -frames-off 18 -cycles 1000 -gc

# ── Photodiode only: visual path in isolation, no audio device, no trigger.
#                    Isolates the display when the combined run looks wrong.
step av-visual "visual only, no audio, no TTL (500 s)" &&
	run av-visual -test av -no-sound -no-ttl \
		-frames-on 12 -frames-off 18 -cycles 1000

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
