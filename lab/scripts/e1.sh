#!/usr/bin/env bash
# Usage: e1.sh <etcd|redis> [repetitions] ["thread counts"]
#
# E1, saturation. Runs the load generator once per thread count N, with no
# faults, and repeats the whole series. Each run starts from a clean cluster,
# lasts DURATION (default 2 min) and discards the first WARMUP (default 30 s).
#
# Results go to results/e1/<system>/rep<r>/n<N>/:
#   summary.json   throughput, latency percentiles and errors of the run
#   timeline.csv   requests completed per second
#   cpu.csv        CPU time of each container, once per second
#   loadgen.txt    output of the generator
#   server-n*.txt  state of each node at the end of the run
#
# A run whose summary.json already exists is skipped, so an interrupted series
# continues where it stopped. To extend the curve beyond N = 512, run again
# with a longer list, for example: e1.sh etcd 3 "1024 2048".
. "$(dirname "$0")/lib.sh"
require_system "${1:-}" '[repetitions] ["thread counts"]'
system=$1
reps=${2:-3}
threads=${3:-1 2 4 8 16 32 64 128 256 512}
DURATION=${DURATION:-120s}
WARMUP=${WARMUP:-30s}

base=e1/$system
mkdir -p "$RESULTS_DIR/$base"
"$SCRIPTS_DIR/env-info.sh" > "$RESULTS_DIR/$base/environment.txt"

for rep in $(seq 1 "$reps"); do
  for n in $threads; do
    dir=$base/rep$rep/n$n
    if [[ -f $RESULTS_DIR/$dir/summary.json ]]; then
      log "E1 $system rep $rep N=$n: already done, skipping"
      continue
    fi
    log "E1 $system rep $rep N=$n: starting clean cluster"
    "$SCRIPTS_DIR/up.sh" "$system" 2>/dev/null
    mkdir -p "$RESULTS_DIR/$dir"

    "$SCRIPTS_DIR/cpu-monitor.sh" "$RESULTS_DIR/$dir/cpu.csv" &
    monitor=$!
    log "E1 $system rep $rep N=$n: load for $DURATION"
    docker exec gerador loadgen -threads "$n" -duration "$DURATION" -warmup "$WARMUP" \
      -out "/results/$dir" | tee "$RESULTS_DIR/$dir/loadgen.txt"
    kill "$monitor" 2>/dev/null || true
    wait "$monitor" 2>/dev/null || true

    "$SCRIPTS_DIR/server-stats.sh" "$RESULTS_DIR/$dir"
  done
done

"$SCRIPTS_DIR/down.sh"
log "E1 $system: done. Summary: go run ./cmd/analyze e1"
