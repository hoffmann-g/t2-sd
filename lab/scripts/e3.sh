#!/usr/bin/env bash
# Usage: e3.sh <etcd|redis> [pairs] [repetitions]
#
# E3, stale reads. Runs the coordinator (PUT on n1, then GET on n3) in two
# conditions: with the original network and with the delay of E4 (50 ms per
# direction between nodes). Each repetition starts from a clean cluster.
# Results go to results/e3/<system>/<normal|delay>/rep<r>/: summary.json,
# timeline.csv, pairs.csv.gz, coordinator.txt and, with delay, rtt.txt.
. "$(dirname "$0")/lib.sh"
require_system "${1:-}" '[pairs] [repetitions]'
system=$1
pairs=${2:-10000}
reps=${3:-3}
DELAY=${DELAY:-50ms}

base=e3/$system
mkdir -p "$RESULTS_DIR/$base"
"$SCRIPTS_DIR/env-info.sh" > "$RESULTS_DIR/$base/environment.txt"
for cond in normal delay; do
  for rep in $(seq 1 "$reps"); do
    dir=$base/$cond/rep$rep
    if [[ -f $RESULTS_DIR/$dir/summary.json ]]; then
      log "E3 $system $cond rep $rep: already done, skipping"
      continue
    fi
    log "E3 $system $cond rep $rep: starting clean cluster"
    "$SCRIPTS_DIR/up.sh" "$system" 2>/dev/null
    mkdir -p "$RESULTS_DIR/$dir"
    if [[ $cond == delay ]]; then
      "$SCRIPTS_DIR/latencia_on.sh" "$DELAY" "${NODES[@]}" > "$RESULTS_DIR/$dir/netem.txt"
      "$SCRIPTS_DIR/rtt.sh" n1 n2 > "$RESULTS_DIR/$dir/rtt.txt"
      tail -1 "$RESULTS_DIR/$dir/rtt.txt"
    fi
    log "E3 $system $cond rep $rep: $pairs pairs"
    docker exec gerador stale -pairs "$pairs" -write-node 1 -read-node 3 -out "/results/$dir" |
      tee "$RESULTS_DIR/$dir/coordinator.txt"
  done
done
"$SCRIPTS_DIR/down.sh"
log "E3 $system: done"
