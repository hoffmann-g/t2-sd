#!/usr/bin/env bash
# Usage: e3.sh <etcd|redis> [pairs] [repetitions]
#
# E3, stale reads. Runs the coordinator (PUT on n1, then GET on n3) in two
# conditions: with the original network and with the delay of E4 (50 ms per
# direction between nodes). Each repetition starts from a clean cluster.
#
# The pairs of a repetition are split among COORDINATORS (default 20)
# coordinator processes that run at the same time, each on its own key, so
# that a repetition with the delay finishes in minutes: with an RTT of 100 ms
# a pair on etcd takes about half a second. Within each coordinator the GET
# still only starts after its PUT returned.
#
# Results go to results/e3/<system>/<normal|delay>/rep<r>/c<k>/ for each
# coordinator: summary.json, timeline.csv, pairs.csv.gz and coordinator.txt.
# The repetition directory holds netem.txt and rtt.txt with the delay.
. "$(dirname "$0")/lib.sh"
require_system "${1:-}" '[pairs] [repetitions]'
system=$1
pairs=${2:-10000}
reps=${3:-3}
DELAY=${DELAY:-50ms}
COORDINATORS=${COORDINATORS:-20}
per=$(( (pairs + COORDINATORS - 1) / COORDINATORS ))

base=e3/$system
mkdir -p "$RESULTS_DIR/$base"
"$SCRIPTS_DIR/env-info.sh" > "$RESULTS_DIR/$base/environment.txt"
for cond in normal delay; do
  for rep in $(seq 1 "$reps"); do
    dir=$base/$cond/rep$rep
    if [[ -f $RESULTS_DIR/$dir/done ]]; then
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
    log "E3 $system $cond rep $rep: $pairs pairs, $COORDINATORS coordinators of $per"
    rm -rf "${RESULTS_DIR:?}/$dir"/c*
    for k in $(seq 1 "$COORDINATORS"); do
      mkdir -p "$RESULTS_DIR/$dir/c$k"
      docker exec gerador stale -pairs "$per" -key "stale$k" -write-node 1 -read-node 3 \
        -out "/results/$dir/c$k" > "$RESULTS_DIR/$dir/c$k/coordinator.txt" 2>&1 &
    done
    wait
    cat "$RESULTS_DIR/$dir"/c*/coordinator.txt | grep '^stale reads' | sort | uniq -c | sort -rn | head -3
    touch "$RESULTS_DIR/$dir/done"
  done
done
"$SCRIPTS_DIR/down.sh"
log "E3 $system: done"
