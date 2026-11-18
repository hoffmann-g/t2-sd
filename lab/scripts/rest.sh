#!/usr/bin/env bash
# Usage: rest.sh <N etcd> <N redis> [repetitions]
# E2 to E6 on both systems with the given N, then the E1 extension of etcd
# (N = 1024 and 2048) and the analysis. Log appended to results/all.log.
. "$(dirname "$0")/lib.sh"
n_etcd=${1:?}; n_redis=${2:?}; reps=${3:-3}
exec >> "$RESULTS_DIR/all.log" 2>&1
cd "$ROOT_DIR"
log "E2 to E6 with N=$n_etcd (etcd) and N=$n_redis (redis)"
for system in etcd redis; do
  n=$n_etcd; [[ $system == redis ]] && n=$n_redis
  "$SCRIPTS_DIR/e2.sh" "$system" "$n" "$reps"
  "$SCRIPTS_DIR/e3.sh" "$system" 10000 "$reps"
  "$SCRIPTS_DIR/e4.sh" "$system" "$n" "$reps"
  "$SCRIPTS_DIR/e5.sh" "$system" "$n" "$reps"
  "$SCRIPTS_DIR/e6.sh" "$system" "$n" "$reps"
done
"$SCRIPTS_DIR/e1.sh" etcd "$reps" "1024 2048"
go run ./cmd/analyze all > "$RESULTS_DIR/analysis.md"
log "series done; tables in results/analysis.md"
