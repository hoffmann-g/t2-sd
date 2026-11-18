#!/usr/bin/env bash
# Usage: leader.sh
# Prints the name of the node that coordinates writes: the Raft leader of
# etcd or the Redis primary.
. "$(dirname "$0")/lib.sh"
case $(running_system) in
  etcd)
    # The IS LEADER column of the status table, followed by IS LEARNER.
    for node in "${NODES[@]}"; do
      if docker exec "$node" etcdctl --endpoints="http://$(client_ip "$node"):2379" endpoint status -w table 2>/dev/null |
        grep -qE '\| +true +\| +false +\|'; then
        echo "$node"
        exit 0
      fi
    done
    echo "no leader found" >&2
    exit 1
    ;;
  redis)
    for node in "${NODES[@]}"; do
      if docker exec "$node" redis-cli role 2>/dev/null | head -1 | grep -q master; then
        echo "$node"
        exit 0
      fi
    done
    echo "no primary found" >&2
    exit 1
    ;;
esac
