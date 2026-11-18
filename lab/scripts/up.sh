#!/usr/bin/env bash
# Usage: up.sh <etcd|redis>
# Starts a clean cluster of the given system and the generator container, and
# waits until the cluster is ready. Any cluster already up is removed first,
# with its data, so every run starts from the same state.
. "$(dirname "$0")/lib.sh"
require_system "${1:-}"
system=$1

mkdir -p "$RESULTS_DIR"
"$SCRIPTS_DIR/down.sh"
compose "$system" up -d --build --quiet-pull >&2

ready_etcd() {
  docker exec n1 etcdctl --endpoints="http://$(client_ip n1):2379" endpoint health --cluster >/dev/null 2>&1
}

ready_redis() {
  docker exec n1 redis-cli info replication 2>/dev/null | grep -q '^connected_slaves:2' &&
    docker exec n2 redis-cli info replication 2>/dev/null | grep -q '^master_link_status:up' &&
    docker exec n3 redis-cli info replication 2>/dev/null | grep -q '^master_link_status:up'
}

for _ in $(seq 1 120); do
  if "ready_$system"; then
    log "$system cluster ready"
    exit 0
  fi
  sleep 0.5
done
echo "$system cluster did not become ready in 60 s" >&2
exit 1
