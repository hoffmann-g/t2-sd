#!/usr/bin/env bash
# Usage: server-stats.sh <output directory>
# Saves the state each node reports about itself, as server-n1.txt and so on.
# etcd: the Prometheus metrics about disk, Raft and network. Redis: INFO.
. "$(dirname "$0")/lib.sh"
out=$1
system=$(running_system)

for node in "${NODES[@]}"; do
  case $system in
    etcd)
      docker exec gerador wget -qO- -T 5 "http://$(client_ip "$node"):2379/metrics" 2>/dev/null |
        grep -E '^(etcd_disk_(wal_fsync|backend_commit)_duration_seconds|etcd_server_|etcd_network_peer_round_trip|etcd_mvcc_db_total_size|process_cpu_seconds_total)' \
          > "$out/server-$node.txt" || log "server-stats: no metrics from $node"
      ;;
    redis)
      docker exec "$node" redis-cli info > "$out/server-$node.txt" 2>/dev/null ||
        log "server-stats: no INFO from $node"
      ;;
  esac
done
