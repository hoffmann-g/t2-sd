#!/usr/bin/env bash
# Usage: latencia_off.sh n1 n2 n3
# Removes the delay added by latencia_on.sh.
. "$(dirname "$0")/lib.sh"
for c in "$@"; do
  docker run --rm --net "container:$c" --cap-add NET_ADMIN -e RE="$(cluster_regex)" "$NETSHOOT_IMAGE" sh -c '
    IF=$(ip -o -4 addr show | awk -v re="$RE" "\$4 ~ re {print \$2}")
    tc qdisc del dev "$IF" root 2>/dev/null || true
    echo "$(hostname): netem removed from $IF"'
done
