#!/usr/bin/env bash
# Usage: run-load.sh <output dir> <loadgen arguments...>
# Runs the load generator inside the gerador container with the CPU monitor
# on, saving loadgen.txt, cpu.csv and the state of the nodes in the output
# directory (a path under results/). Used by the experiment scripts.
. "$(dirname "$0")/lib.sh"
dir=$1
shift
mkdir -p "$RESULTS_DIR/$dir"
"$SCRIPTS_DIR/cpu-monitor.sh" "$RESULTS_DIR/$dir/cpu.csv" &
monitor=$!
docker exec gerador loadgen "$@" -out "/results/$dir" | tee "$RESULTS_DIR/$dir/loadgen.txt"
kill "$monitor" 2>/dev/null || true
wait "$monitor" 2>/dev/null || true
"$SCRIPTS_DIR/server-stats.sh" "$RESULTS_DIR/$dir"
