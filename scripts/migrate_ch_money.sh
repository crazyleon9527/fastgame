#!/usr/bin/env bash
# ClickHouse money migration — idempotent; staging rebuild when MODIFY Decimal→Int64 fails
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CH="docker exec fastgame-clickhouse clickhouse-client --user fastgame --password fastgame_pass"
INIT="$ROOT/docker/clickhouse/init"

col_type() {
  $CH -q "SELECT type FROM system.columns WHERE database='fastgame' AND table='$1' AND name='$2' LIMIT 1" 2>/dev/null || true
}

rebuild_money_tables() {
  echo "Rebuilding ClickHouse money tables from init SQL (staging-safe)…"
  $CH --multiquery <<'EOF'
DROP VIEW IF EXISTS fastgame.mv_rtp_hourly;
DROP TABLE IF EXISTS fastgame.game_round_settled SYNC;
DROP TABLE IF EXISTS fastgame.game_event_bigwin SYNC;
DROP TABLE IF EXISTS fastgame.game_wallet_rollback SYNC;
EOF
  left=$($CH -q "SELECT count() FROM system.tables WHERE database='fastgame' AND name IN ('game_round_settled','game_event_bigwin','game_wallet_rollback')" 2>/dev/null || echo 3)
  if [[ "${left:-3}" != "0" ]]; then
    echo "FAIL: money tables still present after DROP SYNC (count=$left) — restart clickhouse or reset volume" >&2
    exit 1
  fi
  $CH --multiquery < "$INIT/01-init.sql"
  if [[ -f "$INIT/02-trace-migration.sql" ]]; then
    $CH --multiquery < "$INIT/02-trace-migration.sql"
  fi
}

bet_type=$(col_type game_round_settled bet_amount)
if [[ "$bet_type" == "Int64" ]]; then
  echo "ClickHouse game_round_settled.bet_amount already Int64 — ensuring RTP view"
else
  echo "Migrating ClickHouse money columns (current bet_amount type: ${bet_type:-unknown})"
  rows=$($CH -q "SELECT count() FROM fastgame.game_round_settled" 2>/dev/null || echo 0)
  if [[ "${rows:-0}" -le 10000 ]]; then
    echo "Small table (rows=$rows) — rebuild with Int64 schema"
    rebuild_money_tables
  else
    sync=0
    echo "Large table rows=$rows — async UPDATE + MODIFY"
    $CH --multiquery <<EOF
SET mutations_sync = $sync;

ALTER TABLE fastgame.game_round_settled
  UPDATE
    bet_amount = toInt64(toDecimal64(bet_amount, 4) * 10000),
    win_amount = toInt64(toDecimal64(win_amount, 4) * 10000),
    multiplier = toInt64(toDecimal64(multiplier, 4) * 10000),
    balance_after = toInt64(toDecimal64(balance_after, 4) * 10000)
  WHERE 1;

ALTER TABLE fastgame.game_event_bigwin
  UPDATE win_amount = toInt64(toDecimal64(win_amount, 4) * 10000)
  WHERE 1;

ALTER TABLE fastgame.game_wallet_rollback
  UPDATE amount = toInt64(toDecimal64(amount, 4) * 10000)
  WHERE 1;
EOF
    $CH --multiquery <<'EOF'
ALTER TABLE fastgame.game_round_settled
  MODIFY COLUMN bet_amount Int64,
  MODIFY COLUMN win_amount Int64,
  MODIFY COLUMN multiplier Int64,
  MODIFY COLUMN balance_after Int64;

ALTER TABLE fastgame.game_event_bigwin
  MODIFY COLUMN win_amount Int64;

ALTER TABLE fastgame.game_wallet_rollback
  MODIFY COLUMN amount Int64;
EOF
  fi
  final=$(col_type game_round_settled bet_amount)
  if [[ "$final" != "Int64" ]]; then
    echo "FAIL: bet_amount still ${final:-unknown} after migration" >&2
    exit 1
  fi
fi

$CH --multiquery <<'EOF'
DROP TABLE IF EXISTS fastgame.mv_rtp_hourly;

CREATE MATERIALIZED VIEW IF NOT EXISTS fastgame.mv_rtp_hourly
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (merchant_id, game_code, hour)
AS SELECT
    merchant_id,
    game_code,
    toStartOfHour(settled_at) AS hour,
    sum(bet_amount)           AS total_bet,
    sum(win_amount)           AS total_win,
    count()                   AS total_rounds
FROM fastgame.game_round_settled
GROUP BY merchant_id, game_code, hour;
EOF

echo "ClickHouse money migration complete"
