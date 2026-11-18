# Common setup of the lab scripts. Sourced by the others, not executed.

set -euo pipefail

SCRIPTS_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
LAB_DIR=$(dirname "$SCRIPTS_DIR")
ROOT_DIR=$(dirname "$LAB_DIR")
RESULTS_DIR=${RESULTS_DIR:-$ROOT_DIR/results}

# Load lab/.env. A variable already set in the shell keeps its value.
while IFS='=' read -r key value; do
  [[ $key =~ ^[A-Z_]+$ ]] || continue
  [[ -v $key ]] || export "$key=$value"
done < "$LAB_DIR/.env"

# Files written to results/ by the generator container belong to the caller.
LAB_UID=$(id -u)
LAB_GID=$(id -g)
export LAB_UID LAB_GID

NODES=(n1 n2 n3)

# compose <etcd|redis> <docker compose arguments>
compose() {
  local system=$1
  shift
  docker compose -f "$LAB_DIR/$system.yml" "$@"
}

require_system() {
  case "${1:-}" in
    etcd | redis) ;;
    *)
      echo "usage: $(basename "$0") <etcd|redis> ${2:-}" >&2
      exit 2
      ;;
  esac
}

# Address of a node (n1, n2, n3) on each network.
cluster_ip() { echo "$LAB_NET_CLUSTER.1${1#n}"; }
client_ip() { echo "$LAB_NET_CLIENTS.1${1#n}"; }

# Client port of the running system.
client_port() {
  case $(running_system) in
    etcd) echo 2379 ;;
    redis) echo 6379 ;;
  esac
}

# Regular expression that matches an address of the cluster network.
cluster_regex() { echo "^${LAB_NET_CLUSTER//./\\.}\\."; }

# System of the cluster that is up (etcd or redis), read from the generator
# container.
running_system() {
  docker exec gerador printenv LAB_SYSTEM 2>/dev/null || {
    echo "no cluster is up (run up.sh first)" >&2
    return 1
  }
}

log() { echo "[$(date +%T)] $*" >&2; }
