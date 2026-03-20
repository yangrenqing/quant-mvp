#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

GO_BIN="${GO_BIN:-/usr/local/go/bin/go}"
PYTHON_BIN="${PYTHON_BIN:-python3}"

forwarded=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-model) export SKIP_MODEL=1; shift ;;
    --skip-shadow) export SKIP_SHADOW=1; shift ;;
    --skip-promotion) export SKIP_PROMOTION=1; shift ;;
    --skip-health) export SKIP_HEALTH=1; shift ;;
    --skip-factor) export SKIP_FACTOR=1; shift ;;
    --skip-evolution) export SKIP_EVOLUTION=1; shift ;;
    --scan-only) export SCAN_ONLY=1; shift ;;
    --archive-only) export ARCHIVE_ONLY=1; shift ;;
    --from|--to) forwarded+=("$1" "$2"); shift 2 ;;
    *) forwarded+=("$1"); shift ;;
  esac
done

if ((${#forwarded[@]})); then
  exec env PYTHON_BIN="$PYTHON_BIN" PATH="/usr/local/go/bin:$PATH" \
    "$GO_BIN" run ./cmd/scheduler --workflow daily "${forwarded[@]}"
fi

exec env PYTHON_BIN="$PYTHON_BIN" PATH="/usr/local/go/bin:$PATH" \
  "$GO_BIN" run ./cmd/scheduler --workflow daily
