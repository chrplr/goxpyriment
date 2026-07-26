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

# SDL reads these from the *environment*, so they must be exported — a plain
# assignment never reaches the binary.
export SDL_AUDIODRIVER="${SDL_AUDIODRIVER:-alsa}"
AUDIO_BUFFSIZE="${AUDIO_BUFFSIZE:-256}"
REFRESH_HZ="${REFRESH_HZ:-60}"

BIN=./Timing-Tests
HOST=$(hostname -s 2>/dev/null || hostname)
OUTDIR="reports-${HOST}"

STEPS="sysinfo check display display-gc frames frames-gc tones trigger av"

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

# step <name> <label> — prompt before running, unless the step was deselected.
step() {
	selected "$1" || return 1
	printf '\n──── %s : %s ────\n' "$1" "$2"
	read -r -p 'Press Enter to start (Ctrl-C to abort the session) ' _
	return 0
}

# run <report-name> <args...> — show the command, run it, save the output to
# $OUTDIR/<report-name>.txt *and* keep it visible on the terminal.
run() {
	name=$1
	shift
	echo "+ $BIN $*"
	"$BIN" "$@" 2>&1 | tee "$OUTDIR/${name}.txt"
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

# ── Tier 1: frame-interval distributions, GC suspended vs. GC running.
#            This pair is the garbage-collector comparison.
step display "frame intervals, GC SUSPENDED (300 s)" &&
	run display-gc-off -test display -duration-s 300 -hz "$REFRESH_HZ"

step display-gc "frame intervals, GC RUNNING (300 s)" &&
	run display-gc-on -test display -duration-s 300 -hz "$REFRESH_HZ" -gc

# ── Tier 3: stimulus timing validation (photodiode on the screen).
#            Also run as a GC-suspended / GC-running pair.
step frames "single-frame flashes, GC SUSPENDED (300 s)" &&
	run frames-gc-off -test frames -frames-on 1 -frames-off 2 -cycles 6000

step frames-gc "single-frame flashes, GC RUNNING (300 s)" &&
	run frames-gc-on -test frames -frames-on 1 -frames-off 2 -cycles 6000 -gc

step tones "tone stream, audio onset jitter (300 s)" &&
	run tones -test tones -audio-frames "$AUDIO_BUFFSIZE" \
		-tone-ms 50 -iti-ms 450 -cycles 600

# ── Tier 2: trigger device characterisation (DLP-IO8-G on a TTL input line).
step trigger "TTL square wave, DLP-IO8-G (300 s)" &&
	run trigger -test trigger -period-ms 100 -duty 10 -duration-s 300

# ── Tier 3 (cont.): audio–visual synchrony.
step av "audio-visual synchrony (500 s)" &&
	run av -test av -audio-frames "$AUDIO_BUFFSIZE" \
		-frames-on 12 -frames-off 18 -cycles 1000

# ── Tier 4: response timing. COMMENTED OUT until the BBTK response actuator
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
