#!/bin/bash
# run-bbtk.sh — measure MEG TTL box pulse timing with a Black Box ToolKit.
#
# Wraps `test_megttlbox -bbtk` in a bbtk-capture recording, so the capture and
# the pulse sequence cannot get out of step. bbtk-capture owns the whole run: it
# sets the device up and starts the sequence the instant recording begins, via
# its `-- command` form. There is nothing to arm by hand and no way to emit
# pulses into a capture that never started — a bbtk-capture that cannot reach
# the device exits before launching anything.
#
# Usage:
#   ./run-bbtk.sh                  # record to reports-<hostname>/
#   ./run-bbtk.sh -n               # dry run: print the plan, record nothing
#
# Environment overrides:
#   MEGTTL_PORT       Arduino serial port     (default: /dev/ttyACM0)
#   BBTK_PORT         BBTK serial port        (default: bbtk-capture's own)
#   BBTK_CAPTURE_BIN  path to bbtk-capture    (default: bbtk-capture on PATH)
#   BBTK_MARGIN_S     recorded margin either side of the sequence (default: 8)
#   OUTDIR            session directory       (default: reports-<hostname>)
#
# WIRING (line -> Mega pin -> BBTK input):
#     line 0 -> D30 -> TTLin2
#     line 1 -> D31 -> TTLin1
#     GND    -> GND        <- required; TTL without a shared reference gives
#                             unreliable edges
#
# Smoothing is not a concern here: on the BBTK it applies only to the Opto* and
# Mic* channels, never to TTL inputs.
#
# The capture window is computed from the sequence itself (-bbtk-seconds) rather
# than hard-coded, because the device enforces the window and cannot be asked to
# stop early and hand back what it has — a window that turns out too short means
# re-running the whole thing.

set -u
set -o pipefail

MEGTTL_PORT="${MEGTTL_PORT:-/dev/ttyACM0}"
BBTK_CAPTURE_BIN="${BBTK_CAPTURE_BIN:-bbtk-capture}"
BBTK_MARGIN_S="${BBTK_MARGIN_S:-8}"
HOST=$(hostname -s 2>/dev/null || hostname)
OUTDIR="${OUTDIR:-reports-${HOST}}"

DRY_RUN=0
[ "${1:-}" = "-n" ] && DRY_RUN=1

cd "$(dirname "$0")" || exit 1

if [ ! -e "$MEGTTL_PORT" ]; then
	echo "error: $MEGTTL_PORT not found — is the Arduino plugged in?" >&2
	echo "       set MEGTTL_PORT, or list ports with: go run . -list" >&2
	exit 1
fi

# Build rather than `go run`: inside a capture window the compile would burn
# recording time before the first pulse.
BIN=./test_megttlbox
echo "+ go build -o $BIN ."
go build -o "$BIN" . || exit 1

# Ask the sequence how long it is, so the two can never drift apart.
SEQ_S=$("$BIN" -bbtk-seconds) || exit 1
REC_S=$(( SEQ_S + 2 * BBTK_MARGIN_S ))

echo
echo "Host:     $HOST"
echo "Session:  $OUTDIR/"
echo "Arduino:  $MEGTTL_PORT"
echo "BBTK:     ${BBTK_PORT:-auto (bbtk-capture default)}"
echo "Sequence: ${SEQ_S}s + 2x${BBTK_MARGIN_S}s margin = ${REC_S}s recorded"
echo
echo "Wiring:   line 0 -> D30 -> TTLin2"
echo "          line 1 -> D31 -> TTLin1"
echo "          GND    -> GND  (required)"
echo

if [ "$DRY_RUN" = "1" ]; then
	echo "+ $BBTK_CAPTURE_BIN -d $REC_S $OUTDIR/bbtk-megttlbox -- $BIN -device $MEGTTL_PORT -bbtk -no-prompt"
	echo "(dry run — nothing recorded)"
	exit 0
fi

if ! command -v "$BBTK_CAPTURE_BIN" >/dev/null 2>&1 && [ ! -x "$BBTK_CAPTURE_BIN" ]; then
	echo "error: '$BBTK_CAPTURE_BIN' is not executable or not on PATH" >&2
	echo "       set BBTK_CAPTURE_BIN to its full path" >&2
	exit 1
fi
# Check for -- support before touching the device: a bbtk-capture built before
# it would read "--" as a stray argument and reject the whole command line.
if ! "$BBTK_CAPTURE_BIN" -h 2>&1 | grep -q -- '-- command'; then
	echo "error: '$BBTK_CAPTURE_BIN' cannot launch the sequence itself" >&2
	echo "       its usage does not mention '-- command'. Rebuild bbtkv3 and" >&2
	echo "       point BBTK_CAPTURE_BIN at the new binary." >&2
	exit 1
fi

mkdir -p "$OUTDIR"
BASE="$OUTDIR/bbtk-megttlbox"
LOG="$OUTDIR/bbtk-megttlbox.log"

PORT_ARGS=""
[ -n "${BBTK_PORT:-}" ] && PORT_ARGS="-p $BBTK_PORT"

echo "+ $BBTK_CAPTURE_BIN $PORT_ARGS -d $REC_S $BASE -- $BIN -device $MEGTTL_PORT -bbtk -no-prompt"
# $PORT_ARGS is deliberately unquoted: it is either empty or two separate words.
# Both streams are teed: the device spends 11-40 s setting up before the first
# pulse, and a silent terminal through that is indistinguishable from a hang.
# shellcheck disable=SC2086
"$BBTK_CAPTURE_BIN" $PORT_ARGS -d "$REC_S" "$BASE" \
	-- "$BIN" -device "$MEGTTL_PORT" -bbtk -no-prompt \
	2> >(tee "$LOG" >&2) | tee "$OUTDIR/megttlbox-sequence.txt"
rc=$?

if [ "$rc" -ne 0 ]; then
	printf '\n!! capture FAILED (exit %d) — see %s\n' "$rc" "$LOG" >&2
	printf '   A non-zero status means the sequence failed or was aborted, or\n' >&2
	printf '   the capture could not reach the device. The recording cannot be\n' >&2
	printf '   salvaged either way — re-run. If the device could not be opened,\n' >&2
	printf '   set BBTK_PORT (bbtk-detect-port will find it).\n' >&2
	exit 1
fi

printf '\nDone. Files in %s/\n' "$OUTDIR"
for f in "$OUTDIR"/*megttlbox*; do [ -e "$f" ] && printf '  %s\n' "$f"; done
printf '\nReading the capture:\n'
printf '  - Widths up to 1 ms BELOW the request are expected (millis() truncation).\n'
printf '  - Block D: compare the TTLin1 and TTLin2 rising edges. They should fall\n'
printf '    inside one sample; visible skew would be a finding.\n'
printf '  - The spread within a block is the jitter figure worth quoting.\n'
