#!/usr/bin/env bash
# Usage: rtt.sh n1 n2
# Measures the round-trip time from the first node to the second, on the
# cluster network. The second argument may also be an IP address.
. "$(dirname "$0")/lib.sh"
target=$2
[[ $target =~ ^n[0-9]$ ]] && target=$(cluster_ip "$target")
docker run --rm --net "container:$1" "$NETSHOOT_IMAGE" ping -c 5 -W 2 "$target"
