#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

GO_BIN="${GO_BIN:-/usr/local/go/bin/go}"
PYTHON_BIN="${PYTHON_BIN:-python3}"
FROM_DATE="${FROM_DATE:-2025-01-01}"
TO_DATE="${TO_DATE:-$(date +%F)}"
MODEL_LABEL="${MODEL_LABEL:-label_10d}"
NIGHT_START_HOUR="${NIGHT_START_HOUR:-22}"
NIGHT_END_HOUR="${NIGHT_END_HOUR:-8}"
LOCK_DIR="${ROOT_DIR}/.cache/night-shift.lock"

hour_now="$(date +%H)"
hour_now="${hour_now#0}"
if [[ -z "$hour_now" ]]; then
  hour_now=0
fi

in_window=0
if (( NIGHT_START_HOUR == NIGHT_END_HOUR )); then
  in_window=1
elif (( NIGHT_START_HOUR < NIGHT_END_HOUR )); then
  if (( hour_now >= NIGHT_START_HOUR && hour_now < NIGHT_END_HOUR )); then
    in_window=1
  fi
else
  if (( hour_now >= NIGHT_START_HOUR || hour_now < NIGHT_END_HOUR )); then
    in_window=1
  fi
fi

if (( in_window == 0 )) && [[ "${FORCE_RUN:-0}" != "1" ]]; then
  echo "night shift skipped outside window ${NIGHT_START_HOUR}:00-${NIGHT_END_HOUR}:00"
  exit 0
fi

mkdir -p "${ROOT_DIR}/.cache"
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  if [[ -f "$LOCK_DIR/pid" ]]; then
    lock_pid="$(cat "$LOCK_DIR/pid" 2>/dev/null || true)"
    if [[ -n "$lock_pid" ]] && kill -0 "$lock_pid" 2>/dev/null; then
      echo "night shift already running with pid $lock_pid"
      exit 0
    fi
  fi
  rm -rf "$LOCK_DIR"
  mkdir "$LOCK_DIR"
fi
trap 'rm -rf "$LOCK_DIR"' EXIT
echo "$$" > "$LOCK_DIR/pid"

run_go() {
  env PYTHON_BIN="$PYTHON_BIN" PATH="/usr/local/go/bin:$PATH" \
    "$GO_BIN" run ./cmd/scheduler "$@"
}

run_py() {
  "$PYTHON_BIN" "$@"
}

echo "night shift started at $(date --iso-8601=seconds 2>/dev/null || date +%Y-%m-%dT%H:%M:%S%z)"

run_go --workflow research
run_go --export-dataset --from "$FROM_DATE" --to "$TO_DATE"
run_py scripts/model_pipeline.py --from "$FROM_DATE" --to "$TO_DATE" --label "$MODEL_LABEL"
run_py scripts/factor_research.py --dataset reports/training_dataset.csv --label "$MODEL_LABEL"
run_py scripts/factor_diagnostics.py --dataset reports/training_dataset.csv
run_py scripts/model_comparison.py
run_py scripts/health_monitor.py --source night-shift
run_py scripts/evolution_report.py --preset overnight
run_py scripts/runtime_report.py
run_py scripts/strategy_compare.py
run_py scripts/strategy_quality.py
run_py scripts/research_summary.py
run_go --dashboard-only

echo "night shift finished at $(date --iso-8601=seconds 2>/dev/null || date +%Y-%m-%dT%H:%M:%S%z)"
