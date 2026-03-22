#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AGENT_DIR="$HOME/Library/LaunchAgents"
STALE_OPENCLAW_PLIST="$AGENT_DIR/com.yangrenqing.openclaw.quant-mvp.night-shift.plist"
mkdir -p "$AGENT_DIR"

cp "$ROOT_DIR/launchd/com.yangrenqing.quant-mvp.daily.plist" "$AGENT_DIR/"
cp "$ROOT_DIR/launchd/com.yangrenqing.quant-mvp.weekly.plist" "$AGENT_DIR/"
cp "$ROOT_DIR/launchd/com.yangrenqing.quant-mvp.intraday.plist" "$AGENT_DIR/"
cp "$ROOT_DIR/launchd/com.yangrenqing.quant-mvp.night-shift.plist" "$AGENT_DIR/"

if [[ -f "$STALE_OPENCLAW_PLIST" ]]; then
  launchctl bootout "gui/$(id -u)" "$STALE_OPENCLAW_PLIST" >/dev/null 2>&1 || true
  rm -f "$STALE_OPENCLAW_PLIST"
  echo "removed stale launchd agent: com.yangrenqing.openclaw.quant-mvp.night-shift"
fi

launchctl bootout "gui/$(id -u)" "$AGENT_DIR/com.yangrenqing.quant-mvp.daily.plist" >/dev/null 2>&1 || true
launchctl bootout "gui/$(id -u)" "$AGENT_DIR/com.yangrenqing.quant-mvp.weekly.plist" >/dev/null 2>&1 || true
launchctl bootout "gui/$(id -u)" "$AGENT_DIR/com.yangrenqing.quant-mvp.intraday.plist" >/dev/null 2>&1 || true
launchctl bootout "gui/$(id -u)" "$AGENT_DIR/com.yangrenqing.quant-mvp.night-shift.plist" >/dev/null 2>&1 || true

launchctl bootstrap "gui/$(id -u)" "$AGENT_DIR/com.yangrenqing.quant-mvp.daily.plist"
launchctl bootstrap "gui/$(id -u)" "$AGENT_DIR/com.yangrenqing.quant-mvp.weekly.plist"
launchctl bootstrap "gui/$(id -u)" "$AGENT_DIR/com.yangrenqing.quant-mvp.intraday.plist"
launchctl bootstrap "gui/$(id -u)" "$AGENT_DIR/com.yangrenqing.quant-mvp.night-shift.plist"

launchctl enable "gui/$(id -u)/com.yangrenqing.quant-mvp.daily"
launchctl enable "gui/$(id -u)/com.yangrenqing.quant-mvp.weekly"
launchctl enable "gui/$(id -u)/com.yangrenqing.quant-mvp.intraday"
launchctl enable "gui/$(id -u)/com.yangrenqing.quant-mvp.night-shift"

echo "installed launchd agents:"
echo "  com.yangrenqing.quant-mvp.daily"
echo "  com.yangrenqing.quant-mvp.weekly"
echo "  com.yangrenqing.quant-mvp.intraday"
echo "  com.yangrenqing.quant-mvp.night-shift"
