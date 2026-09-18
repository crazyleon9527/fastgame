#!/usr/bin/env bash
# 从 MySQL 数据源一键生成 docs/database 全部业务表的 goctl model（含 Redis cache）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

DSN="${MYSQL_DSN:-fastgame:fastgame_pass@tcp(127.0.0.1:13306)/fastgame}"
OUT_DIR="internal/model/biz"
CACHE="${GOCTL_CACHE:-true}"
STYLE="${GOCTL_STYLE:-goZero}"

# docs/database MySQL 业务表（31 张，不含 ClickHouse）
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

mkdir -p "$OUT_DIR"

CMD=(goctl model mysql datasource -url="$DSN" -dir="$OUT_DIR" -style="$STYLE")
if [[ "$CACHE" == "true" ]]; then
  CMD+=(-cache -p "biz")
fi
for t in "${TABLES[@]}"; do
  CMD+=(-table "$t")
done

echo "== goctl model mysql datasource (${#TABLES[@]} tables, cache=$CACHE) =="
"${CMD[@]}"

echo "OK: models written to $OUT_DIR"
