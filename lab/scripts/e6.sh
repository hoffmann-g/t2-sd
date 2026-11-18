#!/usr/bin/env bash
# Usage: e6.sh <etcd|redis> <N> [repetitions]
#
# E6, node failure. With the load of E2 running (clients know every node and
# rotate over them), one node is killed and later restarted. Two variants:
# (a) the coordinator of writes (the etcd leader or the Redis primary) and
# (b) a follower or replica. Timeline of each run:
#
#   0 s      load starts (30 s of warm-up)
#   60 s     docker kill <node>
#   120 s    docker start <node>
#   180 s    load ends
#
# Results go to results/e6/<system>/<leader|follower>/rep<r>/, with
# events.csv (instants of the kill and the restart, and the node) and
# samples.csv.gz, from which the analysis measures the failover time.
. "$(dirname "$0")/lib.sh"
require_system "${1:-}" '<N> [repetitions]'
system=$1
n=${2:?N (optimal thread count from E1) is required}
reps=${3:-3}
# Phases, in seconds: before the kill (warm-up included), down, after the
# restart. The defaults give the timeline above.
PRE=${PRE:-60}
DOWN=${DOWN:-60}
POST=${POST:-60}
WARMUP=${WARMUP:-30s}
duration=$((PRE + DOWN + POST))s

base=e6/$system
mkdir -p "$RESULTS_DIR/$base"
"$SCRIPTS_DIR/env-info.sh" > "$RESULTS_DIR/$base/environment.txt"
for variant in leader follower; do
  for rep in $(seq 1 "$reps"); do
    dir=$base/$variant/rep$rep
    if [[ -f $RESULTS_DIR/$dir/summary.json ]]; then
      log "E6 $system $variant rep $rep: already done, skipping"
      continue
    fi
    log "E6 $system $variant rep $rep N=$n: starting clean cluster"
    "$SCRIPTS_DIR/up.sh" "$system" 2>/dev/null
    rm -rf "${RESULTS_DIR:?}/$dir"
    mkdir -p "$RESULTS_DIR/$dir"

    "$SCRIPTS_DIR/events.sh" "$dir" "load start"
    "$SCRIPTS_DIR/run-load.sh" "$dir" -threads "$n" -duration "$duration" -warmup "$WARMUP" -samples &
    load=$!
    t_load=$(date +%s)
    sleep $((PRE - 5))
    leader=$("$SCRIPTS_DIR/leader.sh")
    if [[ $variant == leader ]]; then
      victim=$leader
    else
      for node in "${NODES[@]}"; do [[ $node != "$leader" ]] && victim=$node && break; done
    fi
    echo "$leader" > "$RESULTS_DIR/$dir/leader-before.txt"
    sleep $((t_load + PRE - $(date +%s)))
    docker kill "$victim" > /dev/null
    "$SCRIPTS_DIR/events.sh" "$dir" "kill $victim"
    sleep "$DOWN"
    docker start "$victim" > /dev/null
    "$SCRIPTS_DIR/events.sh" "$dir" "start $victim"
    wait "$load"
    "$SCRIPTS_DIR/events.sh" "$dir" "load end"
    "$SCRIPTS_DIR/leader.sh" > "$RESULTS_DIR/$dir/leader-after.txt" 2>/dev/null || true
  done
done
"$SCRIPTS_DIR/down.sh"
log "E6 $system: done"
