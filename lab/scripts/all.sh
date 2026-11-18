#!/usr/bin/env bash
# Usage: all.sh [repetitions]
#
# Runs the whole series on both systems: E1 first, then the analysis of E1
# to find the optimal N of each system, then E2 to E6 with that N. Every
# script skips the runs whose output already exists, so this can be
# interrupted and started again. The log goes to results/all.log.
. "$(dirname "$0")/lib.sh"
reps=${1:-3}
exec > >(tee -a "$RESULTS_DIR/all.log") 2>&1
cd "$ROOT_DIR"

log "series started (repetitions: $reps)"
for system in etcd redis; do
  "$SCRIPTS_DIR/e1.sh" "$system" "$reps"
done

# Optimal N of each system, from the analysis of E1.
go run ./cmd/analyze e1 > "$RESULTS_DIR/e1/e1.md"
declare -A optimal
while IFS=, read -r system threads _ _ _ _ _ _ _ _ _ _ _ _ opt _; do
  [[ $opt == true ]] && optimal[$system]=$threads
done < <(tail -n +2 "$RESULTS_DIR/e1/e1.csv")
for system in etcd redis; do
  [[ -n ${optimal[$system]:-} ]] || { log "no optimal N for $system in e1.csv"; exit 1; }
  log "optimal N of $system: ${optimal[$system]}"
done

for system in etcd redis; do
  n=${optimal[$system]}
  "$SCRIPTS_DIR/e2.sh" "$system" "$n" "$reps"
  "$SCRIPTS_DIR/e3.sh" "$system" 10000 "$reps"
  "$SCRIPTS_DIR/e4.sh" "$system" "$n" "$reps"
  "$SCRIPTS_DIR/e5.sh" "$system" "$n" "$reps"
  "$SCRIPTS_DIR/e6.sh" "$system" "$n" "$reps"
done

go run ./cmd/analyze all > "$RESULTS_DIR/analysis.md"
log "series done; tables in results/analysis.md"
