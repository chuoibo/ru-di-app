#!/usr/bin/env bash
# Prepare the Milvus secrets of the vnlocal stack, once per machine, outside
# the repository (ADR-0049 §2.6):
#   ~/.config/rudi/milvus/user.yaml      server config: auth on, the root
#                                         password, embedded etcd, local
#                                         storage, woodpecker, GPU memory pool
#   ~/.config/rudi/milvus/embedEtcd.yaml  embedded etcd
#   ~/.config/rudi/milvus.env             MOBILE_MILVUS_USER / _PASSWORD for core
# Same server layout as scripts/go_milvus_tier.sh, which the Milvus tier has
# proven. Re-running keeps an existing password: Milvus only reads
# defaultRootPassword when it initialises its data, so a new one would not
# match the volume.
set -euo pipefail

dir="$HOME/.config/rudi/milvus"
envf="$HOME/.config/rudi/milvus.env"
mkdir -p "$dir"
chmod 700 "$HOME/.config/rudi" "$dir"

if [ -f "$envf" ]; then
  pw="$(sed -n 's/^MOBILE_MILVUS_PASSWORD=//p' "$envf")"
  [ -n "$pw" ] || { echo "HỎNG: $envf không có MOBILE_MILVUS_PASSWORD" >&2; exit 1; }
  echo "giữ mật khẩu Milvus có sẵn trong $envf"
else
  pw="$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  (umask 077; printf 'MOBILE_MILVUS_USER=root\nMOBILE_MILVUS_PASSWORD=%s\n' "$pw" >"$envf")
  echo "sinh mật khẩu Milvus mới vào $envf"
fi

# GPU pool in MB: the RTX 4070 has 16 GB and the desktop keeps ~1 GB.
gpu_init="${RUDI_MILVUS_GPU_INIT_MB:-1024}"
gpu_max="${RUDI_MILVUS_GPU_MAX_MB:-6144}"

(umask 077; cat >"$dir/user.yaml" <<YAML
etcd:
  use:
    embed: true
  data:
    dir: /var/lib/milvus/etcd
  config:
    path: /milvus/configs/embedEtcd.yaml
common:
  storageType: local
  security:
    authorizationEnabled: true
    defaultRootPassword: "$pw"
mq:
  type: woodpecker
woodpecker:
  storage:
    type: local
gpu:
  initMemSize: $gpu_init
  maxMemSize: $gpu_max
YAML
)
cat >"$dir/embedEtcd.yaml" <<YAML
listen-client-urls: http://0.0.0.0:2379
advertise-client-urls: http://0.0.0.0:2379
quota-backend-bytes: $((4 << 30))
auto-compaction-mode: revision
auto-compaction-retention: '1000'
YAML
# The server runs as root in the container and reads the bind mount.
chmod 644 "$dir/user.yaml" "$dir/embedEtcd.yaml"
echo "xong: $dir/user.yaml, $dir/embedEtcd.yaml"
