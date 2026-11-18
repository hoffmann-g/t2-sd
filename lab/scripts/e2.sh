#!/usr/bin/env bash
# Usage: e2.sh <etcd|redis> <N> [repetitions]
#
# E2, baseline. Runs the load generator with the optimal N of E1 for 3 min
# (the first 30 s discarded), with no faults, from a clean cluster, and
# repeats. Results go to results/e2/<system>/rep<r>/ (see e1.sh for the
# files). A repetition whose summary.json exists is skipped.
. "$(dirname "$0")/lib.sh"
require_system "${1:-}" '<N> [repetitions]'
system=$1
n=${2:?N (optimal thread count from E1) is required}
reps=${3:-3}
DURATION=${DURATION:-180s}
WARMUP=${WARMUP:-30s}

base=e2/$system
mkdir -p "$RESULTS_DIR/$base"
"$SCRIPTS_DIR/env-info.sh" > "$RESULTS_DIR/$base/environment.txt"
for rep in $(seq 1 "$reps"); do
  dir=$base/rep$rep
  if [[ -f $RESULTS_DIR/$dir/summary.json ]]; then
    log "E2 $system rep $rep: already done, skipping"
    continue
  fi
  log "E2 $system rep $rep N=$n: starting clean cluster"
  "$SCRIPTS_DIR/up.sh" "$system" 2>/dev/null
  log "E2 $system rep $rep: load for $DURATION"
  "$SCRIPTS_DIR/run-load.sh" "$dir" -threads "$n" -duration "$DURATION" -warmup "$WARMUP" -samples
done
"$SCRIPTS_DIR/down.sh"
log "E2 $system: done"
