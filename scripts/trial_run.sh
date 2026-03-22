#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

GO_BIN="${GO_BIN:-/usr/local/go/bin/go}"
PYTHON_BIN="${PYTHON_BIN:-python3}"
FROM_DATE="${FROM_DATE:-2025-01-01}"
TO_DATE="${TO_DATE:-$(date +%F)}"
TOP_N="${TOP_N:-3}"
INITIAL_CASH="${INITIAL_CASH:-100000}"
FEE_BPS="${FEE_BPS:-10}"
SLIPPAGE_BPS="${SLIPPAGE_BPS:-5}"
TRIAL_COUNT="${TRIAL_COUNT:-100}"
TRIAL_PREFIX="${TRIAL_PREFIX:-trial-$(date +%Y%m%d-%H%M%S)}"
TRIAL_REPORT_TAG="${TRIAL_REPORT_TAG:-$TRIAL_PREFIX}"
INCLUDE_SHADOW_TRIAL="${INCLUDE_SHADOW_TRIAL:-1}"
SHADOW_VERSION="${SHADOW_VERSION:-}"

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
    --from) FROM_DATE="$2"; shift 2 ;;
    --to) TO_DATE="$2"; shift 2 ;;
    --top) TOP_N="$2"; shift 2 ;;
    --cash) INITIAL_CASH="$2"; shift 2 ;;
    --fee-bps) FEE_BPS="$2"; shift 2 ;;
    --slippage-bps) SLIPPAGE_BPS="$2"; shift 2 ;;
    --trial-count) TRIAL_COUNT="$2"; shift 2 ;;
    --trial-prefix) TRIAL_PREFIX="$2"; shift 2 ;;
    --trial-report-tag) TRIAL_REPORT_TAG="$2"; shift 2 ;;
    --shadow-version) SHADOW_VERSION="$2"; forwarded+=("$1" "$2"); shift 2 ;;
    --no-shadow-trial) INCLUDE_SHADOW_TRIAL=0; shift ;;
    *) forwarded+=("$1"); shift ;;
  esac
done

run_scheduler() {
  env PYTHON_BIN="$PYTHON_BIN" PATH="/usr/local/go/bin:$PATH" \
    "$GO_BIN" run ./cmd/scheduler "$@"
}

daily_args=(
  --workflow daily
  --python-bin "$PYTHON_BIN"
  --from "$FROM_DATE"
  --to "$TO_DATE"
  --top "$TOP_N"
  --cash "$INITIAL_CASH"
  --fee-bps "$FEE_BPS"
  --slippage-bps "$SLIPPAGE_BPS"
)
if ((${#forwarded[@]})); then
  daily_args+=("${forwarded[@]}")
fi
run_scheduler "${daily_args[@]}"

trial_args=(
  --paper-trial-run
  --paper-market a_share
  --top "$TOP_N"
  --cash "$INITIAL_CASH"
  --fee-bps "$FEE_BPS"
  --slippage-bps "$SLIPPAGE_BPS"
  --trial-count "$TRIAL_COUNT"
  --trial-prefix "$TRIAL_PREFIX"
  --trial-report-tag "$TRIAL_REPORT_TAG"
)
if [[ "$INCLUDE_SHADOW_TRIAL" != "0" ]]; then
  trial_args+=(--trial-include-shadow)
fi
if [[ -n "$SHADOW_VERSION" ]]; then
  trial_args+=(--shadow-version "$SHADOW_VERSION")
fi

run_scheduler "${trial_args[@]}"

echo "trial workflow complete"
echo "  dashboard: $ROOT_DIR/reports/dashboard.html"
echo "  latest experiment report: $ROOT_DIR/reports/paper_trials_latest.html"
echo "  latest winner: $ROOT_DIR/reports/paper_trial_winner_latest.txt"
