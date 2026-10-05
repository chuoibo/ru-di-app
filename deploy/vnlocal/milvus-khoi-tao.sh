#!/usr/bin/env bash
# Prepare the Milvus secrets of the vnlocal stack, once per machine, outside
# the repository (ADR-0049 §2.6):
#   ~/.config/rudi/milvus/user.yaml      server config: auth on, the root
#                                         password, embedded etcd, MinIO
#                                         storage, woodpecker, GPU memory pool
#   ~/.config/rudi/milvus/embedEtcd.yaml  embedded etcd
#   ~/.config/rudi/milvus/minio.env       the milvus-minio root credentials
#   ~/.config/rudi/milvus.env             MOBILE_MILVUS_USER / _PASSWORD for core
# Re-running keeps existing passwords: Milvus only reads defaultRootPassword
# when it initialises its data, so a new one would not match the volume.
#
# Storage is MinIO, not local (05/10): with woodpecker on local storage
# Milvus v3.0.2 never compacts or truncates the WAL ("Local storage detected,
# skipping compaction (not yet implemented)"). Empty segments piled up
# (234k on one channel), every WAL read listed them all from etcd, and the
# server crash-looped. scripts/go_milvus_tier.sh keeps local: its data lives
# for one run.
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

minio_envf="$dir/minio.env"
if [ -f "$minio_envf" ]; then
  minio_pw="$(sed -n 's/^MINIO_ROOT_PASSWORD=//p' "$minio_envf")"
  [ -n "$minio_pw" ] || { echo "HỎNG: $minio_envf không có MINIO_ROOT_PASSWORD" >&2; exit 1; }
  echo "giữ mật khẩu MinIO có sẵn trong $minio_envf"
else
  minio_pw="$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  # MC_HOST_local lets the healthcheck's `mc ready local` reach the server.
  (umask 077; printf 'MINIO_ROOT_USER=rudi-milvus\nMINIO_ROOT_PASSWORD=%s\nMC_HOST_local=http://rudi-milvus:%s@127.0.0.1:9000\n' \
    "$minio_pw" "$minio_pw" >"$minio_envf")
  echo "sinh mật khẩu MinIO mới vào $minio_envf"
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
  storageType: remote
  security:
    authorizationEnabled: true
    defaultRootPassword: "$pw"
minio:
  address: milvus-minio
  port: 9000
  accessKeyID: rudi-milvus
  secretAccessKey: "$minio_pw"
  useSSL: false
  bucketName: milvus
mq:
  type: woodpecker
woodpecker:
  storage:
    type: minio
gpu:
  initMemSize: $gpu_init
  maxMemSize: $gpu_max
YAML
)
# etcd's heartbeat-interval / election-timeout are milliseconds: leave them
# at the defaults (100 / 1000). A hand edit to 2 / 10 on 02/10 made etcd
# 50-100x tighter, not looser.
cat >"$dir/embedEtcd.yaml" <<YAML
listen-client-urls: http://0.0.0.0:2379
advertise-client-urls: http://0.0.0.0:2379
quota-backend-bytes: $((4 << 30))
auto-compaction-mode: revision
auto-compaction-retention: '1000'
YAML
# The server runs as root in the container and reads the bind mount.
chmod 644 "$dir/user.yaml" "$dir/embedEtcd.yaml"
echo "xong: $dir/user.yaml, $dir/embedEtcd.yaml, $minio_envf"
