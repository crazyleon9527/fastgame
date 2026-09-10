#!/usr/bin/env bash
# 生产源站防火墙 — 仅允许 Cloudflare 回源 IP 访问 80/443，封死其他入站
# 用法: sudo ./apply-ufw.sh [--dry-run]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
IPS_V4="$SCRIPT_DIR/ips-v4.txt"
IPS_V6="$SCRIPT_DIR/ips-v6.txt"
DRY_RUN=false

if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=true
fi

if [[ ! -f "$IPS_V4" ]]; then
  echo "Missing $IPS_V4 — run fetch-cloudflare-ips.sh first" >&2
  exit 1
fi

run() {
  if $DRY_RUN; then
    echo "[dry-run] $*"
  else
    "$@"
  fi
}

echo "==> Resetting UFW to deny incoming by default"
run ufw --force reset
run ufw default deny incoming
run ufw default allow outgoing

echo "==> Allowing SSH (22/tcp) — adjust if you use a non-standard port"
run ufw allow 22/tcp comment 'SSH admin'

echo "==> Allowing Cloudflare IPv4 to HTTP/HTTPS"
while read -r cidr; do
  [[ -z "$cidr" ]] && continue
  run ufw allow from "$cidr" to any port 80 proto tcp comment 'CF origin HTTP'
  run ufw allow from "$cidr" to any port 443 proto tcp comment 'CF origin HTTPS'
done < "$IPS_V4"

if [[ -f "$IPS_V6" ]]; then
  echo "==> Allowing Cloudflare IPv6 to HTTP/HTTPS"
  while read -r cidr; do
    [[ -z "$cidr" ]] && continue
    run ufw allow from "$cidr" to any port 80 proto tcp comment 'CF origin HTTP v6'
    run ufw allow from "$cidr" to any port 443 proto tcp comment 'CF origin HTTPS v6'
  done < "$IPS_V6"
fi

echo "==> Enabling UFW"
if $DRY_RUN; then
  echo "[dry-run] ufw enable"
else
  ufw --force enable
  ufw status verbose
fi

echo ""
echo "Done. Backend services (RGS/Admin :18888/:18889) must bind 127.0.0.1 only."
echo "Public traffic enters via Cloudflare -> Nginx gateway on 80/443."
