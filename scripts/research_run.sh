#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

GO_BIN="${GO_BIN:-/usr/local/go/bin/go}"

exec env PATH="/usr/local/go/bin:$PATH" "$GO_BIN" run ./cmd/scheduler --workflow research "$@"
