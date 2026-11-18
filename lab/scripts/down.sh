#!/usr/bin/env bash
# Usage: down.sh
# Stops whichever cluster is up and deletes its data.
. "$(dirname "$0")/lib.sh"

for system in etcd redis; do
  compose "$system" down -v --remove-orphans 2>&1 | grep -v 'Warning: No resource found' >&2 || true
done
