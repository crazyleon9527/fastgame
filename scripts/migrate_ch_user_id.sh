#!/usr/bin/env bash
# ClickHouse user_id UInt64 -> String（小表 rebuild，大表 async UPDATE+MODIFY）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CH="docker exec fastgame-clickhouse clickhouse-client --user fastgame --password fastgame_pass"
INIT="$ROOT/docker/clickhouse/init"

col_type() {
  $CH -q "SELECT type FROM system.columns WHERE database='fastgame' AND table='$1' AND name='user_id' LIMIT 1" 2>/dev/null || true
}

typ=$(col_type game_round_settled)
if [[ "$typ" == "String" ]]; then
  echo "ClickHouse user_id already String"
  exit 0
fi

rows=$($CH -q "SELECT count() FROM fastgame.game_round_settled" 2>/dev/null || echo 0)
if [[ "${rows:-0}" -le 10000 ]]; then
  echo "Small table rows=$rows — rebuild CH tables with String user_id"
  $CH --multiquery <<'EOF'
DROP VIEW IF EXISTS fastgame.mv_rtp_hourly;
DROP TABLE IF EXISTS fastgame.game_round_settled SYNC;
DROP TABLE IF EXISTS fastgame.game_event_bigwin SYNC;
DROP TABLE IF EXISTS fastgame.game_wallet_rollback SYNC;
EOF
  $CH --multiquery < "$INIT/01-init.sql"
  $CH --multiquery < "$INIT/02-trace-migration.sql"
else
  echo "Large table — apply 04-user-id-string-migration.sql"
  $CH --multiquery < "$INIT/04-user-id-string-migration.sql"
fi

final=$(col_type game_round_settled)
if [[ "$final" != "String" ]]; then
  echo "FAIL: user_id still ${final:-unknown}" >&2
  exit 1
fi
echo "ClickHouse user_id migration complete"
