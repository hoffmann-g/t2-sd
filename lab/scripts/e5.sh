#!/usr/bin/env bash
# Usage: e5.sh <etcd|redis> <N> [repetitions]
#
# E5, network partition. With the load of E2 running and each client pinned
# to one node (so that both sides of the partition are observed), n1 is
# isolated from n2 and n3 for 60 s and then reconnected. Timeline of the
# run, with the warm-up of 30 s:
#
#   0 s      load starts
#   60 s     partition (particao.sh)
#   120 s    heal (cura.sh)
#   180 s    load ends
#
# After the run, check reads every key from every node and compares the
# values with the confirmed writes of the run (samples.csv.gz), reporting
# divergence between nodes and confirmed writes that were lost.
# Results go to results/e5/<system>/rep<r>/, with events.csv (instants of
# the partition and the heal) and check.txt.
. "$(dirname "$0")/lib.sh"
require_system "${1:-}" '<N> [repetitions]'
system=$1
n=${2:?N (optimal thread count from E1) is required}
reps=${3:-3}
# Phases, in seconds: before the partition (warm-up included), partitioned,
# after the heal. The defaults give the timeline above.
PRE=${PRE:-60}
PART=${PART:-60}
POST=${POST:-60}
WARMUP=${WARMUP:-30s}
duration=$((PRE + PART + POST))s

base=e5/$system
mkdir -p "$RESULTS_DIR/$base"
"$SCRIPTS_DIR/env-info.sh" > "$RESULTS_DIR/$base/environment.txt"
for rep in $(seq 1 "$reps"); do
  dir=$base/rep$rep
  if [[ -f $RESULTS_DIR/$dir/check.txt ]]; then
    log "E5 $system rep $rep: already done, skipping"
    continue
  fi
  log "E5 $system rep $rep N=$n: starting clean cluster"
  "$SCRIPTS_DIR/up.sh" "$system" 2>/dev/null
  rm -rf "${RESULTS_DIR:?}/$dir"
  mkdir -p "$RESULTS_DIR/$dir"
  "$SCRIPTS_DIR/leader.sh" > "$RESULTS_DIR/$dir/leader-before.txt" || true
  log "E5 $system rep $rep: coordinator before the partition: $(cat "$RESULTS_DIR/$dir/leader-before.txt")"

  "$SCRIPTS_DIR/events.sh" "$dir" "load start"
  "$SCRIPTS_DIR/run-load.sh" "$dir" -threads "$n" -duration "$duration" -warmup "$WARMUP" -pinned -samples &
  load=$!
  sleep "$PRE"
  t_part=$(date +%s)
  "$SCRIPTS_DIR/particao.sh" > "$RESULTS_DIR/$dir/particao.txt"
  "$SCRIPTS_DIR/events.sh" "$dir" "partition"
  # Mandatory verification: n2 reaches n3, n1 does not reach n2, the
  # generator reaches n1.
  # The second ping is expected to fail; none of them stops the script.
  {
    echo "== n2 -> n3 (must work)"; "$SCRIPTS_DIR/rtt.sh" n2 n3 2>&1 | tail -2 || true
    echo "== n1 -> n2 (must fail)"; "$SCRIPTS_DIR/rtt.sh" n1 n2 2>&1 | tail -2 || true
    echo "== gerador -> n1 (must work)"
    if docker exec gerador nc -zw2 "$(client_ip n1)" "$(client_port)"; then echo "tcp connection ok"; else echo "FAILED"; fi
  } > "$RESULTS_DIR/$dir/partition-check.txt"
  "$SCRIPTS_DIR/leader.sh" > "$RESULTS_DIR/$dir/leader-during.txt" 2>/dev/null || echo "none" > "$RESULTS_DIR/$dir/leader-during.txt"
  sleep $((t_part + PART - $(date +%s)))
  "$SCRIPTS_DIR/cura.sh" > "$RESULTS_DIR/$dir/cura.txt"
  "$SCRIPTS_DIR/events.sh" "$dir" "heal"
  wait "$load"
  "$SCRIPTS_DIR/events.sh" "$dir" "load end"
  "$SCRIPTS_DIR/leader.sh" > "$RESULTS_DIR/$dir/leader-after.txt" 2>/dev/null || true

  log "E5 $system rep $rep: checking convergence"
  sleep 5
  docker exec gerador check -samples "/results/$dir/samples.csv.gz" | tee "$RESULTS_DIR/$dir/check.txt"
done
"$SCRIPTS_DIR/down.sh"
log "E5 $system: done"
