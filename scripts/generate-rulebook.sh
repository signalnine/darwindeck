#!/usr/bin/env bash
# Generate rulebooks from evolved game genomes
#
# Usage:
#   ./scripts/generate-rulebook.sh genome.json                    # Single file with LLM
#   ./scripts/generate-rulebook.sh genome.json --basic            # Single file, no LLM
#   ./scripts/generate-rulebook.sh ./evolution-run/ --top 5       # Directory, top 5
#   ./scripts/generate-rulebook.sh genome.json -o /tmp/rules.md   # Custom output

set -euo pipefail

cd "$(dirname "$0")/.."

uv run python -m darwindeck.cli.rulebook "$@"
