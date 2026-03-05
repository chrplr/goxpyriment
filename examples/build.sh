#!/bin/bash

# Get the script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Loop through subdirectories that have a main.go file
for dir in */; do
    if [ -f "$dir/main.go" ]; then
        echo "Building ${dir%/}..."
        (cd "$dir" && go build .)
        if [ $? -ne 0 ]; then
            echo "Failed to build ${dir%/}"
        fi
    fi
done
