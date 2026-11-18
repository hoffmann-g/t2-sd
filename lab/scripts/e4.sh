#!/usr/bin/env bash
# Usage: e4.sh <etcd|redis> <N> [repetitions]
#
# E4, network with high latency. Like E2, with a fixed delay of 50 ms per
# direction between the nodes (RTT of 100 ms), applied with tc netem on the
# cluster network only. The RTT is measured before each run and saved in
# rtt.txt. Results go to results/e4/<system>/rep<r>/.
. "$(dirname "$0")/lib.sh"
require_system "${1:-}" '<N> [repetitions]'
system=$1
n=${2:?N (optimal thread count from E1) is required}
reps=${3:-3}
DURATION=${DURATION:-180s}
WARMUP=${WARMUP:-30s}
DELAY=${DELAY:-50ms}

base=e4/$system
mkdir -p "$RESULTS_DIR/$base"
"$SCRIPTS_DIR/env-info.sh" > "$RESULTS_DIR/$base/environment.txt"
for rep in $(seq 1 "$reps"); do
  dir=$base/rep$rep
  if [[ -f $RESULTS_DIR/$dir/summary.json ]]; then
    log "E4 $system rep $rep: already done, skipping"
    continue
  fi
  log "E4 $system rep $rep N=$n: starting clean cluster"
  "$SCRIPTS_DIR/up.sh" "$system" 2>/dev/null
  mkdir -p "$RESULTS_DIR/$dir"
  "$SCRIPTS_DIR/latencia_on.sh" "$DELAY" "${NODES[@]}" > "$RESULTS_DIR/$dir/netem.txt"
  "$SCRIPTS_DIR/rtt.sh" n1 n2 > "$RESULTS_DIR/$dir/rtt.txt"
  tail -1 "$RESULTS_DIR/$dir/rtt.txt"
  log "E4 $system rep $rep: load for $DURATION with $DELAY per direction"
  "$SCRIPTS_DIR/run-load.sh" "$dir" -threads "$n" -duration "$DURATION" -warmup "$WARMUP" -samples
done
"$SCRIPTS_DIR/down.sh"
log "E4 $system: done"
