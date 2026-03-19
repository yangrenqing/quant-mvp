#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

mkdir -p research/papers research/factors research/experiments

echo "research workspace ready:"
echo "  $ROOT_DIR/research/papers"
echo "  $ROOT_DIR/research/factors"
echo "  $ROOT_DIR/research/experiments"
echo
echo "Suggested next step:"
echo "  put paper notes in research/papers and factor ideas in research/factors"
