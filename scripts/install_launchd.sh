#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AGENT_DIR="$HOME/Library/LaunchAgents"
mkdir -p "$AGENT_DIR"

cp "$ROOT_DIR/launchd/com.yangrenqing.quant-mvp.daily.plist" "$AGENT_DIR/"
cp "$ROOT_DIR/launchd/com.yangrenqing.quant-mvp.weekly.plist" "$AGENT_DIR/"

launchctl bootout "gui/$(id -u)" "$AGENT_DIR/com.yangrenqing.quant-mvp.daily.plist" >/dev/null 2>&1 || true
launchctl bootout "gui/$(id -u)" "$AGENT_DIR/com.yangrenqing.quant-mvp.weekly.plist" >/dev/null 2>&1 || true

launchctl bootstrap "gui/$(id -u)" "$AGENT_DIR/com.yangrenqing.quant-mvp.daily.plist"
launchctl bootstrap "gui/$(id -u)" "$AGENT_DIR/com.yangrenqing.quant-mvp.weekly.plist"

launchctl enable "gui/$(id -u)/com.yangrenqing.quant-mvp.daily"
launchctl enable "gui/$(id -u)/com.yangrenqing.quant-mvp.weekly"

echo "installed launchd agents:"
echo "  com.yangrenqing.quant-mvp.daily"
echo "  com.yangrenqing.quant-mvp.weekly"
