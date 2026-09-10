#!/usr/bin/env bash
# iptables 版源站防火墙 — 仅允许 Cloudflare 回源 IP 访问 80/443
# 用法: sudo ./apply-iptables.sh [--dry-run]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
IPS_V4="$SCRIPT_DIR/ips-v4.txt"
IPS_V6="$SCRIPT_DIR/ips-v6.txt"
DRY_RUN=false
CHAIN="FASTGAME_CF"

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

echo "==> Building iptables chain: $CHAIN"
run iptables -N "$CHAIN" 2>/dev/null || run iptables -F "$CHAIN"

while read -r cidr; do
  [[ -z "$cidr" ]] && continue
  run iptables -A "$CHAIN" -s "$cidr" -p tcp -m multiport --dports 80,443 -j ACCEPT
done < "$IPS_V4"

run iptables -A "$CHAIN" -p tcp -m multiport --dports 80,443 -j DROP

if ! iptables -C INPUT -p tcp -m multiport --dports 80,443 -j "$CHAIN" 2>/dev/null; then
  run iptables -I INPUT 1 -p tcp -m multiport --dports 80,443 -j "$CHAIN"
fi

if [[ -f "$IPS_V6" ]] && command -v ip6tables >/dev/null; then
  echo "==> Building ip6tables chain: ${CHAIN}_V6"
  run ip6tables -N "${CHAIN}_V6" 2>/dev/null || run ip6tables -F "${CHAIN}_V6"
  while read -r cidr; do
    [[ -z "$cidr" ]] && continue
    run ip6tables -A "${CHAIN}_V6" -s "$cidr" -p tcp -m multiport --dports 80,443 -j ACCEPT
  done < "$IPS_V6"
  run ip6tables -A "${CHAIN}_V6" -p tcp -m multiport --dports 80,443 -j DROP
  if ! ip6tables -C INPUT -p tcp -m multiport --dports 80,443 -j "${CHAIN}_V6" 2>/dev/null; then
    run ip6tables -I INPUT 1 -p tcp -m multiport --dports 80,443 -j "${CHAIN}_V6"
  fi
fi

echo ""
echo "Done. Save rules with: iptables-save / ip6tables-save (or netfilter-persistent)."
echo "Backend bind 127.0.0.1 — never expose RGS/Admin ports publicly."
