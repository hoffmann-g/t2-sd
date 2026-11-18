#!/usr/bin/env bash
# Usage: events.sh <output dir> <label>
# Appends a line with the current time and the label to events.csv in the
# output directory. The analysis aligns these instants with the timeline.
. "$(dirname "$0")/lib.sh"
dir=$RESULTS_DIR/$1
mkdir -p "$dir"
[[ -f $dir/events.csv ]] || echo "unix_ms,label" > "$dir/events.csv"
echo "$(date +%s%3N),$2" >> "$dir/events.csv"
log "$2"
