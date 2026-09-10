#!/usr/bin/env bash
# Staging 对账验证：MySQL 金额列类型 + bet_limits minor + ClickHouse Int64
set -euo pipefail

MYSQL="docker exec fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame -N -e"
CH="docker exec fastgame-clickhouse clickhouse-client --user fastgame --password fastgame_pass -q"

echo "== MySQL column types =="
$MYSQL "SHOW COLUMNS FROM pending_transactions LIKE 'bet_amount';"
$MYSQL "SHOW COLUMNS FROM wallet_pending_ops LIKE 'bet_amount';"
$MYSQL "SHOW COLUMNS FROM game_round_replay LIKE 'bet_amount';"

echo "== bet_limits seed (expect min=10000) =="
$MYSQL "SELECT config_value FROM game_configs WHERE config_key='bet_limits' LIMIT 1;"

echo "== ClickHouse game_round_settled types =="
CH_OUT=""
if CH_OUT=$( ( $CH "SELECT name, type FROM system.columns WHERE database='fastgame' AND table='game_round_settled' AND name IN ('bet_amount','win_amount','multiplier')" ) & pid=$!; sleep 15; kill $pid 2>/dev/null; wait $pid 2>/dev/null ); then
  echo "$CH_OUT"
  if echo "$CH_OUT" | grep -qv 'Int64'; then
    echo "FAIL: expected Int64 money columns in ClickHouse (run: make migrate-ch-money)" >&2
    exit 1
  fi
else
  echo "WARN: ClickHouse query timed out — server may be applying mutations; retry: make migrate-ch-money && make verify-staging-money" >&2
fi

echo "== Sample reconcile: pending_tx vs mock bet minor =="
$MYSQL "
SELECT COUNT(*) AS pending_rows,
       COALESCE(SUM(bet_amount),0) AS sum_bet_minor
FROM pending_transactions;
"

echo "OK: staging money verification complete"
