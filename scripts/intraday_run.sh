#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

GO_BIN="${GO_BIN:-/usr/local/go/bin/go}"
PYTHON_BIN="${PYTHON_BIN:-python3}"
TOP_N="${TOP_N:-3}"
CASH="${CASH:-100000}"
FEE_BPS="${FEE_BPS:-10}"
SLIPPAGE_BPS="${SLIPPAGE_BPS:-5}"
SHADOW_VERSION="${SHADOW_VERSION:-$(awk -F': ' '/shadow_version:/{print $2}' configs/model.yaml | tail -n 1)}"
RUN_SHADOW="${RUN_SHADOW:-1}"
FORCE_RUN="${FORCE_RUN:-0}"

is_market_open() {
  local weekday hour minute hm
  weekday="$(TZ=Asia/Shanghai date +%u)"
  hour="$(TZ=Asia/Shanghai date +%H)"
  minute="$(TZ=Asia/Shanghai date +%M)"
  hm=$((10#$hour * 100 + 10#$minute))

  if [[ "$weekday" -gt 5 ]]; then
    return 1
  fi
  if (( hm >= 930 && hm < 1130 )); then
    return 0
  fi
  if (( hm >= 1300 && hm < 1500 )); then
    return 0
  fi
  return 1
}

echo "intraday cycle started at $(date '+%F %T')"
if [[ "$FORCE_RUN" != "1" ]] && ! is_market_open; then
  echo "market session closed; skipping paper execution"
  "$PYTHON_BIN" scripts/health_monitor.py --source intraday
  exit 0
fi

PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --paper-run --once --paper-market a_share --paper-mode live --top "$TOP_N" --cash "$CASH" --fee-bps "$FEE_BPS" --slippage-bps "$SLIPPAGE_BPS"

if [[ "$RUN_SHADOW" == "1" && -n "${SHADOW_VERSION}" ]]; then
  PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --paper-shadow-run --once --paper-market a_share --paper-mode live --shadow-version "$SHADOW_VERSION" --top "$TOP_N" --cash "$CASH" --fee-bps "$FEE_BPS" --slippage-bps "$SLIPPAGE_BPS"
fi

"$PYTHON_BIN" scripts/health_monitor.py --source intraday
"$PYTHON_BIN" scripts/evolution_report.py --hours 24
"$PYTHON_BIN" scripts/evolution_report.py --preset overnight
PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --dashboard-only >/dev/null
echo "intraday cycle complete"
