#!/usr/bin/env bash
# docker compose for the vnlocal stack, with the vnlocal machine's current IP.
#
# The vnlocal machine is a laptop that changes networks; its services follow
# its IP and are reached by name, lakiet-Surface-Laptop-3.local (mDNS; vnlocal
# HANDOFF-KET-NOI.md, "Việc bên dùng cần làm"). A container cannot resolve a
# .local name, so this resolves it on the host and hands the IP to every
# service that talks to the machine (extra_hosts in compose.yml): its LAN
# address when the name resolves and agy answers there, else its Tailscale
# address. Run it
# instead of `docker compose -f deploy/vnlocal/compose.yml`:
#
#   deploy/vnlocal/up.sh up -d --build
#   deploy/vnlocal/up.sh run --rm rag v-status
#
# After the machine changes networks, run `deploy/vnlocal/up.sh up -d` again:
# running containers keep the IP they started with.
set -euo pipefail

host="${RUDI_VNLOCAL_HOST:-lakiet-Surface-Laptop-3.local}"
# Tailscale fallback (vnlocal HANDOFF-KET-NOI.md, "Việc bên dùng cần làm"
# steps 3-4): fixed whatever WiFi the machine is on; when both machines share
# a LAN, Tailscale goes direct over it.
# The address is kept out of the repository (repo guard): VNLOCAL_TAILSCALE_IP
# in ~/.config/rudi/vnlocal.env, or RUDI_VNLOCAL_TAILSCALE.
tailscale_ip="${RUDI_VNLOCAL_TAILSCALE:-$(sed -n 's/^VNLOCAL_TAILSCALE_IP=//p' "$HOME/.config/rudi/vnlocal.env" 2>/dev/null | tail -1)}"
song() { curl -s -m 3 -o /dev/null -w '%{http_code}' "http://$1:20131/health" 2>/dev/null | grep -q '^200$'; }
# LAN first, by name (mDNS); if the name does not resolve here, by the LAN
# address the machine itself publishes in its handoff ("Địa chỉ hiện tại",
# rewritten and pushed by serve/doi-mang.sh on every network change); Tailscale
# only when neither answers (owner, 2026-09-29: prefer the LAN).
vnlocal_repo="${RUDI_VNLOCAL_REPO:-$(cd "$(dirname "$0")/../../.." && pwd)/vnlocal}"
ip_handoff() {
  [ -d "$vnlocal_repo/.git" ] || return 0
  git -C "$vnlocal_repo" fetch -q origin main 2>/dev/null || true
  git -C "$vnlocal_repo" show origin/main:HANDOFF-KET-NOI.md 2>/dev/null |
    sed -n 's/.*Địa chỉ hiện tại của máy vnlocal: `\([0-9.]*\)`.*/\1/p' | head -1
}
duong=lan
ip="$(getent ahostsv4 "$host" | awk 'NR==1 {print $1}' || true)"
if [ -z "$ip" ] || ! song "$ip"; then
  duong="lan (địa chỉ trong handoff)"
  ip="$(ip_handoff)"
fi
if [ -z "$ip" ] || ! song "$ip"; then
  duong=tailscale
  ip="$tailscale_ip"
  if [ -z "$ip" ] || ! song "$ip"; then
    echo "HỎNG: không vào được máy vnlocal bằng LAN ($host) lẫn Tailscale ($tailscale_ip)." >&2
    echo "  Máy đó đang tắt / mất mạng, hoặc máy này chưa ở trong tailnet (tailscale status)." >&2
    echo "  Máy vnlocal đang ở đâu: khối 'Địa chỉ hiện tại' đầu vnlocal HANDOFF-KET-NOI.md (git pull)." >&2
    exit 1
  fi
fi
# The sidecar's internal token, shared by core (MOBILE_RERANK_TOKEN) and
# ai-infer (AI_INFER_TOKEN); made once, mode 600, outside the repository.
tok="$HOME/.config/rudi/ai-infer.env"
if [ ! -f "$tok" ]; then
  t="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  (umask 077; printf 'AI_INFER_TOKEN=%s\nMOBILE_RERANK_TOKEN=%s\n' "$t" "$t" >"$tok")
  echo "--- sinh token sidecar vào $tok" >&2
fi
# The AI services (core on the Go engine, rag, rag-indexer) read their keys
# from the repo root .env of THIS tree (the x-ai-secrets anchor marks it
# optional, so compose alone would start them keyless and they crash-loop
# «GEMINI_API_KEY is not set»; seen 2026-09-29 deploying from a worktree).
# Refuse up front, naming the variables only, never their values.
root_env="$(cd "$(dirname "$0")/../.." && pwd)/.env"
thieu=""
for v in GEMINI_API_KEY AGY_PROXY_URL AGY_PROXY_KEY OPEN_ROUTER_API_KEY; do
  grep -qE "^$v=.+" "$root_env" 2>/dev/null || thieu="$thieu $v"
done
if [ -n "$thieu" ]; then
  echo "HỎNG: $root_env thiếu:$thieu" >&2
  echo "  Deploy từ worktree khác thì trỏ .env về file thật (symlink, đã gitignore) hoặc chạy từ cây có .env." >&2
  exit 1
fi
export RUDI_VNLOCAL_HOST="$host" RUDI_VNLOCAL_IP="$ip"
echo "--- $host = $ip (đường $duong)" >&2
cd "$(dirname "$0")/../.."
exec docker compose -f deploy/vnlocal/compose.yml "$@"
