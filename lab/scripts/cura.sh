#!/usr/bin/env bash
# Usage: cura.sh
# Reconnects n1 to the cluster network, with the same address.
. "$(dirname "$0")/lib.sh"
docker network connect --ip "$(cluster_ip n1)" lab_cluster n1
echo "partition healed at $(date +%T)"
