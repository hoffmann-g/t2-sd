#!/usr/bin/env bash
# Usage: cpu-monitor.sh <output.csv>
# Records, once per second, the CPU time each container (n1, n2, n3, gerador)
# has consumed since it started, read from its cgroup. Runs until killed.
# The difference between two rows of a container, divided by the time between
# them, is the number of CPUs it kept busy in that interval.
. "$(dirname "$0")/lib.sh"
out=$1

declare -A stat
for c in "${NODES[@]}" gerador; do
  id=$(docker inspect -f '{{.Id}}' "$c")
  for p in "/sys/fs/cgroup/system.slice/docker-$id.scope/cpu.stat" "/sys/fs/cgroup/docker/$id/cpu.stat"; do
    [[ -r $p ]] && stat[$c]=$p && break
  done
  if [[ -z ${stat[$c]:-} ]]; then
    log "cpu-monitor: cgroup of $c not found; container CPU will not be recorded"
    exit 0
  fi
done

echo "unix_ms,container,cpu_usec" > "$out"
trap 'exit 0' TERM INT
while :; do
  now=$(date +%s%3N)
  for c in "${NODES[@]}" gerador; do
    # First line of cpu.stat: "usage_usec <microseconds>". The file is
    # missing while the container is stopped.
    if read -r _ usec 2>/dev/null < "${stat[$c]}"; then
      echo "$now,$c,$usec"
    fi
  done >> "$out"
  sleep 1
done
