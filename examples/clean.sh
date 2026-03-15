#!/bin/bash

# Get the script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Loop through subdirectories that have a main.go file
for dir in */; do
    dir_name=${dir%/}
    if [ -f "$dir/main.go" ]; then
        if [ -f "$dir/$dir_name" ]; then
            echo "Cleaning $dir_name..."
            rm "$dir/$dir_name"
        fi
        # Also handle Windows binaries if they exist
        if [ -f "$dir/$dir_name.exe" ]; then
            echo "Cleaning $dir_name.exe..."
            rm "$dir/$dir_name.exe"
        fi
    fi
done
