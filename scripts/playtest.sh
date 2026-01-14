#!/usr/bin/env bash
# Human playtesting CLI for evolved card games
#
# Usage:
#   ./scripts/playtest.sh                           # Interactive picker
#   ./scripts/playtest.sh genome.json               # Specific genome
#   ./scripts/playtest.sh genome.json -d greedy     # With difficulty

set -euo pipefail

cd "$(dirname "$0")/.."

uv run python -m darwindeck.cli.playtest "$@"
