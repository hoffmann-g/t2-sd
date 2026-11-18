#!/usr/bin/env bash
# Usage: latencia_on.sh 50ms n1 n2 n3
# Adds a fixed delay to the packets each listed node sends on the cluster
# network. The client network is not affected. A delay of 50ms on every node
# gives a round-trip time of 100ms between nodes.
#
# The tc command runs in an auxiliary container that shares the network
# namespace of the node, because the images of the systems do not ship it.
. "$(dirname "$0")/lib.sh"
delay=$1
shift
for c in "$@"; do
  docker run --rm --net "container:$c" --cap-add NET_ADMIN -e D="$delay" -e RE="$(cluster_regex)" "$NETSHOOT_IMAGE" sh -c '
    IF=$(ip -o -4 addr show | awk -v re="$RE" "\$4 ~ re {print \$2}")
    tc qdisc replace dev "$IF" root netem delay "$D"
    echo "$(hostname): netem $D on $IF"'
done
