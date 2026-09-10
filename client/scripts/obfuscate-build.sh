#!/usr/bin/env bash
# 对 Cocos Creator 导出的游戏业务 JS 进行高强度混淆
# 用法: ./scripts/obfuscate-build.sh [build/web-mobile]
set -euo pipefail

CLIENT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="${1:-$CLIENT_DIR/build/web-mobile}"
OBF_CONFIG="$CLIENT_DIR/obfuscator.config.json"
OBF_BIN="$CLIENT_DIR/node_modules/.bin/javascript-obfuscator"

if [[ ! -d "$BUILD_DIR" ]]; then
  echo "Build directory not found: $BUILD_DIR" >&2
  echo "Export from Cocos Creator first (Project -> Build -> Web Mobile)." >&2
  exit 1
fi

if [[ ! -x "$OBF_BIN" ]]; then
  echo "Installing javascript-obfuscator..."
  (cd "$CLIENT_DIR" && npm install --no-save javascript-obfuscator@^4.1.1)
fi

# 仅混淆游戏业务 bundle，跳过 Cocos 引擎与第三方 runtime
TARGETS=(
  "$BUILD_DIR/assets/main/index.js"
  "$BUILD_DIR/assets/internal/index.js"
  "$BUILD_DIR/src/chunks"
)

count=0
obfuscate_file() {
  local file="$1"
  local tmp="${file}.obf.js"
  echo "  obfuscating: $file"
  "$OBF_BIN" "$file" --output "$tmp" --config "$OBF_CONFIG"
  mv "$tmp" "$file"
  count=$((count + 1))
}

for target in "${TARGETS[@]}"; do
  if [[ -f "$target" ]]; then
    obfuscate_file "$target"
  elif [[ -d "$target" ]]; then
    while IFS= read -r -d '' js; do
      obfuscate_file "$js"
    done < <(find "$target" -name '*.js' -type f -print0)
  fi
done

if [[ "$count" -eq 0 ]]; then
  echo "No JS files matched. Expected Cocos build output under: $BUILD_DIR" >&2
  exit 1
fi

echo "Done. Obfuscated $count file(s) in $BUILD_DIR"
