#!/usr/bin/env bash
# Usage: env-info.sh
# Prints the hardware, the software versions and the lab configuration of
# this host, to be stored next to the results of an experiment.
. "$(dirname "$0")/lib.sh"

sysfs() { cat "$1" 2>/dev/null || echo "?"; }

echo "date:      $(date -Is)"
echo "kernel:    $(uname -srm)"
echo "os:        $(. /etc/os-release 2>/dev/null && echo "${PRETTY_NAME:-?}")"
echo
lscpu | grep -E '^(Model name|CPU\(s\)|Thread\(s\) per core|Core\(s\) per socket|Socket\(s\)|CPU max MHz)'
echo "governor:  $(sysfs /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor)"
echo "epp:       $(sysfs /sys/devices/system/cpu/cpu0/cpufreq/energy_performance_preference)"
for ac in /sys/class/power_supply/A*/online; do
  [[ -r $ac ]] && echo "ac online: $(cat "$ac") ($(basename "$(dirname "$ac")"))"
done
echo
free -h | sed -n 1,3p
echo
root=$(docker info -f '{{.DockerRootDir}}')
echo "docker data: $root"
df -hT "$root" | tail -1
lsblk -d -o NAME,MODEL,ROTA,TRAN,SIZE 2>/dev/null | grep -v zram || true
echo
docker version --format 'docker engine {{.Server.Version}}'
docker compose version
docker info -f 'storage driver {{.Driver}}, cgroup v{{.CgroupVersion}} ({{.CgroupDriver}})'
echo
echo "images:"
for img in "$ETCD_IMAGE" "$REDIS_IMAGE" "$NETSHOOT_IMAGE" lab-gerador; do
  docker image inspect -f "  $img {{.Id}}" "$img" 2>/dev/null || echo "  $img (not pulled)"
done
echo
echo "client libraries (go.mod):"
grep -E 'go-redis|etcd/client/v3 |grpc ' "$ROOT_DIR/go.mod" | sed 's/^\s*/  /'
echo
echo "lab/.env:"
grep -vE '^\s*(#|$)' "$LAB_DIR/.env" | sed 's/^/  /'
echo
echo "other running containers:"
docker ps --format '{{.Names}} ({{.Image}})' | grep -vE '^(n[123]|gerador) ' | sed 's/^/  /' | grep . || echo "  none"
