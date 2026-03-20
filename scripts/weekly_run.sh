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
MIN_PROMOTION_EDGE="${MIN_PROMOTION_EDGE:-$(awk -F': ' '/min_promotion_edge:/{print $2}' configs/model.yaml | tail -n 1)}"
MIN_SHADOW_OBSERVATIONS="${MIN_SHADOW_OBSERVATIONS:-$(awk -F': ' '/min_shadow_observations:/{print $2}' configs/model.yaml | tail -n 1)}"
SHADOW_VERSION="${SHADOW_VERSION:-$(awk -F': ' '/shadow_version:/{print $2}' configs/model.yaml | tail -n 1)}"
AUTO_ROLLBACK_EDGE="${AUTO_ROLLBACK_EDGE:-0.0}"
SKIP_MODEL="${SKIP_MODEL:-0}"
SKIP_ROLLBACK="${SKIP_ROLLBACK:-0}"
SKIP_HEALTH="${SKIP_HEALTH:-0}"
SKIP_EVOLUTION="${SKIP_EVOLUTION:-0}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --from) FROM_DATE="$2"; shift 2 ;;
    --to) TO_DATE="$2"; shift 2 ;;
    --skip-model) SKIP_MODEL=1; shift ;;
    --skip-rollback) SKIP_ROLLBACK=1; shift ;;
    --skip-health) SKIP_HEALTH=1; shift ;;
    --skip-evolution) SKIP_EVOLUTION=1; shift ;;
    *) echo "unknown arg: $1" >&2; exit 1 ;;
  esac
done

echo "[1/9] prepare research workspace"
bash scripts/research_run.sh >/dev/null

echo "[2/9] refresh dataset"
PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --export-dataset --from "$FROM_DATE" --to "$TO_DATE"

echo "[3/9] run model pipeline"
if [[ "$SKIP_MODEL" != "1" ]]; then
  "$PYTHON_BIN" scripts/model_pipeline.py --from "$FROM_DATE" --to "$TO_DATE" --label "$MODEL_LABEL"
else
  echo "skip-model enabled"
fi

echo "[4/9] refresh factor research"
"$PYTHON_BIN" scripts/factor_research.py --dataset reports/training_dataset.csv --label "$MODEL_LABEL"
"$PYTHON_BIN" scripts/factor_diagnostics.py --dataset reports/training_dataset.csv
"$PYTHON_BIN" scripts/model_comparison.py
"$PYTHON_BIN" scripts/strategy_quality.py
"$PYTHON_BIN" scripts/research_summary.py

echo "[5/9] refresh shadow account"
PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --paper-shadow-run --once --shadow-version "$SHADOW_VERSION" --top 3 --cash "$CASH" --fee-bps "$FEE_BPS" --slippage-bps "$SLIPPAGE_BPS"

echo "[6/9] evaluate promotion"
"$PYTHON_BIN" scripts/strategy_promote.py --candidate "$SHADOW_VERSION" --min-edge "$MIN_PROMOTION_EDGE" --min-observations "$MIN_SHADOW_OBSERVATIONS"

echo "[7/9] evaluate rollback"
if [[ "$SKIP_ROLLBACK" != "1" ]]; then
  "$PYTHON_BIN" scripts/strategy_auto_rollback.py --min-edge "$AUTO_ROLLBACK_EDGE"
else
  echo "skip-rollback enabled"
fi

echo "[8/9] refresh health monitor"
if [[ "$SKIP_HEALTH" != "1" ]]; then
  "$PYTHON_BIN" scripts/health_monitor.py
else
  echo "skip-health enabled"
fi

echo "[9/9] refresh evolution report"
if [[ "$SKIP_EVOLUTION" != "1" ]]; then
  "$PYTHON_BIN" scripts/evolution_report.py --hours 168
  "$PYTHON_BIN" scripts/evolution_report.py --preset overnight
else
  echo "skip-evolution enabled"
fi

PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --dashboard-only >/dev/null

echo "weekly workflow complete"
