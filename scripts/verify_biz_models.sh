#!/usr/bin/env bash
# 校验 docs/database 31 张 MySQL 表是否均已生成 goctl model + cache 层
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIZ="$ROOT/internal/model/biz"

TABLES=(
  merchant_accounts merchant_credentials merchant_network_configs merchant_wallet_endpoints
  merchant_wallet_dlq merchant_contracts merchant_financial_periods merchant_reconciliation_diffs
  merchant_risk_policies merchant_promo_configs merchant_users merchant_audit_logs
  system_exchange_rates player_game_sessions game_math_models
  games merchant_games game_sessions game_transactions game_rounds
  promo_campaigns promo_tournament_rules promo_mission_definitions promo_user_progress promo_reward_grants
  i18n_languages i18n_modules i18n_messages i18n_merchant_overrides i18n_release_versions
  rpt_export_tasks
)

# goctl goZero style: merchant_accounts -> merchantAccountsModel.go
to_camel_file() {
  python3 - "$1" <<'PY'
import re, sys
name = sys.argv[1]
parts = name.split('_')
camel = parts[0] + ''.join(p.capitalize() for p in parts[1:])
print(f"{camel}Model")
PY
}

fail=0
for t in "${TABLES[@]}"; do
  base="$(to_camel_file "$t")"
  gen="$BIZ/${base}_gen.go"
  ext="$BIZ/${base}.go"
  if [[ ! -f "$gen" ]]; then
    echo "FAIL missing: $gen (table=$t)"
    fail=1
    continue
  fi
  if [[ ! -f "$ext" ]]; then
    echo "FAIL missing: $ext (table=$t)"
    fail=1
    continue
  fi
  if ! grep -q 'CachedConn' "$gen" 2>/dev/null; then
    echo "FAIL no cache layer in $gen (table=$t)"
    fail=1
    continue
  fi
  echo "OK  $t -> ${base}"
done

if [[ "$fail" -ne 0 ]]; then
  echo "verification failed"
  exit 1
fi
echo "All ${#TABLES[@]} tables verified"
