#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

GO_BIN="${GO_BIN:-/usr/local/go/bin/go}"
PYTHON_BIN="${PYTHON_BIN:-python3}"
FROM_DATE="${FROM_DATE:-2025-01-01}"
TO_DATE="${TO_DATE:-$(date +%F)}"
TOP_N="${TOP_N:-10}"
CASH="${CASH:-100000}"
FEE_BPS="${FEE_BPS:-10}"
SLIPPAGE_BPS="${SLIPPAGE_BPS:-5}"
MODEL_LABEL="${MODEL_LABEL:-label_10d}"
PROMOTION_METRIC="${PROMOTION_METRIC:-$(awk -F': ' '/promotion_metric:/{print $2}' configs/model.yaml | tail -n 1)}"
MIN_PROMOTION_EDGE="${MIN_PROMOTION_EDGE:-$(awk -F': ' '/min_promotion_edge:/{print $2}' configs/model.yaml | tail -n 1)}"
MIN_SHADOW_OBSERVATIONS="${MIN_SHADOW_OBSERVATIONS:-$(awk -F': ' '/min_shadow_observations:/{print $2}' configs/model.yaml | tail -n 1)}"
SHADOW_VERSION="${SHADOW_VERSION:-$(awk -F': ' '/shadow_version:/{print $2}' configs/model.yaml | tail -n 1)}"
SKIP_MODEL="${SKIP_MODEL:-0}"
SCAN_ONLY="${SCAN_ONLY:-0}"
ARCHIVE_ONLY="${ARCHIVE_ONLY:-0}"
SKIP_SHADOW="${SKIP_SHADOW:-0}"
SKIP_PROMOTION="${SKIP_PROMOTION:-0}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --from) FROM_DATE="$2"; shift 2 ;;
    --to) TO_DATE="$2"; shift 2 ;;
    --skip-model) SKIP_MODEL=1; shift ;;
    --skip-shadow) SKIP_SHADOW=1; shift ;;
    --skip-promotion) SKIP_PROMOTION=1; shift ;;
    --scan-only) SCAN_ONLY=1; shift ;;
    --archive-only) ARCHIVE_ONLY=1; shift ;;
    *) echo "unknown arg: $1" >&2; exit 1 ;;
  esac
done

if [[ "$ARCHIVE_ONLY" == "1" ]]; then
  echo "archive-only mode: regenerating dashboard from current reports"
  PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --scan-a-share --top "$TOP_N" >/dev/null
  exit 0
fi

echo "[1/7] scan A-share universe"
PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --scan-a-share --top "$TOP_N"

if [[ "$SCAN_ONLY" == "1" ]]; then
  echo "scan-only workflow complete"
  exit 0
fi

echo "[2/7] run portfolio backtest"
PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --portfolio-backtest --from "$FROM_DATE" --to "$TO_DATE" --cash "$CASH" --fee-bps "$FEE_BPS" --slippage-bps "$SLIPPAGE_BPS" --top 3

echo "[3/7] export training dataset"
PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --export-dataset --from "$FROM_DATE" --to "$TO_DATE"

echo "[4/7] refresh model pipeline"
if [[ "$SKIP_MODEL" != "1" ]]; then
  "$PYTHON_BIN" scripts/model_pipeline.py --from "$FROM_DATE" --to "$TO_DATE" --label "$MODEL_LABEL"
else
  echo "skip-model enabled"
fi

echo "[5/7] run shadow paper account"
if [[ "$SKIP_SHADOW" != "1" ]]; then
  PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --paper-shadow-run --once --shadow-version "$SHADOW_VERSION" --top 3 --cash "$CASH" --fee-bps "$FEE_BPS" --slippage-bps "$SLIPPAGE_BPS"
else
  echo "skip-shadow enabled"
fi

echo "[6/7] evaluate promotion"
if [[ "$SKIP_PROMOTION" != "1" ]]; then
  "$PYTHON_BIN" scripts/strategy_promote.py --candidate "$SHADOW_VERSION" --min-edge "$MIN_PROMOTION_EDGE" --min-observations "$MIN_SHADOW_OBSERVATIONS"
else
  echo "skip-promotion enabled"
fi

echo "[7/7] refresh active paper account"
PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --paper-run --once --top 3 --cash "$CASH" --fee-bps "$FEE_BPS" --slippage-bps "$SLIPPAGE_BPS"

echo "daily workflow complete"
echo "dashboard: $ROOT_DIR/reports/dashboard.html"
