#!/usr/bin/env bash
# Usage: particao.sh
# Isolates n1 from n2 and n3 by disconnecting it from the cluster network.
# n1 stays on the client network, so clients still reach it.
. "$(dirname "$0")/lib.sh"
docker network disconnect lab_cluster n1
echo "n1 isolated from n2 and n3 at $(date +%T)"
