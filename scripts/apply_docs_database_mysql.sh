#!/usr/bin/env bash
# 将 docs/database 中 MySQL 业务 DDL 按依赖顺序导入 fastgame 库
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MYSQL="${MYSQL_CMD:-docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame}"

run_sql() {
  local f="$1"
  echo "== applying $(basename "$f") =="
  $MYSQL < "$f"
}

run_sql "$ROOT/docs/database/merchant.sql"
run_sql "$ROOT/docs/database/math.sql"
run_sql "$ROOT/docs/database/game.sql"
run_sql "$ROOT/docs/database/promo.sql"
run_sql "$ROOT/docs/database/lang.sql"
run_sql "$ROOT/docs/database/report_mysql.sql"

echo "OK: docs/database MySQL schema applied"
