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

echo "[1/4] scan A-share universe"
PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --scan-a-share --top "$TOP_N"

echo "[2/4] run portfolio backtest"
PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --portfolio-backtest --from "$FROM_DATE" --to "$TO_DATE" --cash "$CASH" --fee-bps "$FEE_BPS" --slippage-bps "$SLIPPAGE_BPS" --top 3

echo "[3/4] export training dataset"
PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --export-dataset --from "$FROM_DATE" --to "$TO_DATE"

echo "[4/4] refresh model pipeline"
"$PYTHON_BIN" scripts/model_pipeline.py --from "$FROM_DATE" --to "$TO_DATE" --label "$MODEL_LABEL"

echo "daily workflow complete"
echo "dashboard: $ROOT_DIR/reports/dashboard.html"
