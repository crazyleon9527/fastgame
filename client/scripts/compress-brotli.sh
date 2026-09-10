#!/usr/bin/env bash
# 对 Cocos Web Mobile 导出产物生成 .br 预压缩文件，供 Nginx brotli_static 直接回源
# 用法: ./scripts/compress-brotli.sh [build/web-mobile]
set -euo pipefail

CLIENT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="${1:-$CLIENT_DIR/build/web-mobile}"
QUALITY="${BROTLI_QUALITY:-6}"

if [[ ! -d "$BUILD_DIR" ]]; then
  echo "Build directory not found: $BUILD_DIR" >&2
  exit 1
fi

if ! command -v brotli >/dev/null 2>&1; then
  echo "brotli CLI not found. Install: brew install brotli  OR  apt install brotli" >&2
  exit 1
fi

count=0
while IFS= read -r -d '' file; do
  brotli -f -q "$QUALITY" -o "${file}.br" "$file"
  count=$((count + 1))
done < <(find "$BUILD_DIR" -type f \( -name '*.js' -o -name '*.wasm' -o -name '*.json' -o -name '*.css' -o -name '*.html' \) ! -name '*.br' -print0)

echo "Done. Brotli compressed $count file(s) under $BUILD_DIR (quality=$QUALITY)"
