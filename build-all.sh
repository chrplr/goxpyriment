#!/bin/bash
#
# build-all.sh — build every example and test into _build/
#
# This is a no-make equivalent of `make all` for users who do not have
# `make` installed (notably on Windows running this from Git Bash).
# It builds the same binaries into the same _build/ folder.
#
# Usage (from the repository root):
#   bash build-all.sh

set -e

# Resolve the repo root (directory of this script) so it works from anywhere.
ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$ROOT"

mkdir -p _build

# Executable suffix for the target platform (".exe" on Windows, empty elsewhere).
# Go only appends this automatically when `-o` is NOT given; since we pass an
# explicit output path below, we must add it ourselves or Windows binaries end
# up without the .exe extension.
EXE="$(go env GOEXE)"

# How many times to attempt a build whose only problem is a locked file.
ATTEMPTS=3

failed=()
saw_lock=0

# build_one builds one program, retrying when the failure is a locked file
# rather than a compilation error.
#
# On Windows the go command links into %TEMP%\go-buildNNN\b001\exe\a.out.exe and
# then copies that file to the -o path. Real-time antivirus opens the freshly
# written .exe to scan it, and while it is held open the copy fails with "The
# process cannot access the file because it is being used by another process".
# That is a race, not a build error: the compiled packages are already cached,
# so the same command run a moment later almost always succeeds. Retrying here
# saves the user from a red line in the middle of a sixty-program build and from
# a _build/ folder that is silently missing a binary.
build_one() {
    local dir=$1 name=$2
    local attempt out
    for (( attempt = 1; attempt <= ATTEMPTS; attempt++ )); do
        if out=$( cd "$dir" && CGO_ENABLED=0 go build -o "$ROOT/_build/$name$EXE" . 2>&1 ); then
            return 0
        fi
        printf '%s\n' "$out"
        case "$out" in
            *"being used by another process"*|*"Access is denied"*|*"text file busy"*)
                saw_lock=1
                if (( attempt < ATTEMPTS )); then
                    echo "  .. the linked binary was locked by another process (usually antivirus); retrying $name ($attempt/$((ATTEMPTS - 1)))"
                    sleep 2
                    continue
                fi
                ;;
        esac
        # A real compilation error, or a lock that outlasted every retry.
        return 1
    done
    return 1
}

for dir in examples/*/ tests/*/; do
    [ -f "${dir}main.go" ] || continue
    name=${dir%/}; name=${name##*/}
    echo "Building $name..."
    if ! build_one "$dir" "$name"; then
        echo "  !! failed to build $name"
        failed+=("$name")
    fi
done

if (( ${#failed[@]} == 0 )); then
    echo "Done. Binaries are in $ROOT/_build/"
    exit 0
fi

# Say plainly what is missing. The old script ended with "Done." and a zero exit
# status even when programs had failed, so a build that was short two binaries
# looked exactly like one that was complete.
echo
echo "Done, but ${#failed[@]} of the programs did NOT build:"
for name in "${failed[@]}"; do
    echo "  - $name"
done
echo "The other binaries are in $ROOT/_build/"

if (( saw_lock )); then
    cat <<'HINT'

At least one of those failures was a locked file, not a compilation error —
another process was holding the freshly linked executable. On Windows that is
almost always real-time antivirus scanning it.

  * Simply run ./build-all.sh again: everything that succeeded is cached, so
    only the missing programs are rebuilt, and the retry usually works.
  * If it keeps happening, exclude the Go build directories from real-time
    scanning (Windows Security > Virus & threat protection > Manage settings >
    Exclusions), or point Go's temporary directory at a folder that is already
    excluded:

        export GOTMPDIR=/c/go-tmp    # in Git Bash; mkdir -p /c/go-tmp first

    The directories worth excluding are %LOCALAPPDATA%\Temp, the build cache
    reported by `go env GOCACHE`, and this repository.
HINT
fi

exit 1
