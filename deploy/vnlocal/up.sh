#!/usr/bin/env bash
# docker compose for the vnlocal stack, with the vnlocal machine's current IP.
#
# The vnlocal machine is a laptop that changes networks; its services follow
# its IP and are reached by name, lakiet-Surface-Laptop-3.local (mDNS; vnlocal
# HANDOFF-KET-NOI.md, "Việc bên dùng cần làm"). A container cannot resolve a
# .local name, so this resolves it on the host and hands the IP to every
# service that talks to the machine (extra_hosts in compose.yml). Run it
# instead of `docker compose -f deploy/vnlocal/compose.yml`:
#
#   deploy/vnlocal/up.sh up -d --build
#   deploy/vnlocal/up.sh run --rm rag v-status
#
# After the machine changes networks, run `deploy/vnlocal/up.sh up -d` again:
# running containers keep the IP they started with.
set -euo pipefail

host="${RUDI_VNLOCAL_HOST:-lakiet-Surface-Laptop-3.local}"
ip="$(getent ahostsv4 "$host" | awk 'NR==1 {print $1}')"
if [ -z "$ip" ]; then
  echo "HỎNG: không phân giải được $host." >&2
  echo "  Máy này phải cùng WiFi/LAN với máy vnlocal, và có avahi-daemon + libnss-mdns." >&2
  echo "  Máy vnlocal đang ở mạng nào: khối 'Địa chỉ hiện tại' đầu vnlocal HANDOFF-KET-NOI.md (git pull)." >&2
  exit 1
fi
export RUDI_VNLOCAL_HOST="$host" RUDI_VNLOCAL_IP="$ip"
echo "--- $host = $ip" >&2
cd "$(dirname "$0")/../.."
exec docker compose -f deploy/vnlocal/compose.yml "$@"
