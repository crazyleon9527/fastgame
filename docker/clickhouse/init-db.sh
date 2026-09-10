#!/bin/bash
set -euo pipefail

HOST="${CLICKHOUSE_HOST:-clickhouse}"
USER="${CLICKHOUSE_USER:-fastgame}"
PASSWORD="${CLICKHOUSE_PASSWORD:-fastgame_pass}"
MAX_RETRIES=30

wait_for_clickhouse() {
  echo "Waiting for ClickHouse at ${HOST}..."
  for i in $(seq 1 "$MAX_RETRIES"); do
    if clickhouse-client --host "$HOST" --user "$USER" --password "$PASSWORD" --query "SELECT 1" >/dev/null 2>&1; then
      echo "ClickHouse is ready."
      return 0
    fi
    echo "  attempt ${i}/${MAX_RETRIES}..."
    sleep 2
  done
  echo "ClickHouse not ready after ${MAX_RETRIES} attempts."
  exit 1
}

wait_for_clickhouse

echo "Applying ClickHouse schema..."
clickhouse-client --host "$HOST" --user "$USER" --password "$PASSWORD" --multiquery < /scripts/01-init.sql
if [ -f /scripts/02-trace-migration.sql ]; then
  clickhouse-client --host "$HOST" --user "$USER" --password "$PASSWORD" --multiquery < /scripts/02-trace-migration.sql
fi

echo "ClickHouse tables:"
clickhouse-client --host "$HOST" --user "$USER" --password "$PASSWORD" --query "SHOW TABLES FROM fastgame"
