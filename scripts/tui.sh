#!/usr/bin/env bash
# Launch playtest with Rich TUI display
#
# Usage:
#   ./scripts/tui.sh                     # Interactive genome picker
#   ./scripts/tui.sh path/to/genome.json # Specific genome
#   ./scripts/tui.sh --latest            # Most recent evolution output
#
# Options are passed through to playtest command:
#   ./scripts/tui.sh genome.json --difficulty mcts --seed 42

set -e

cd "$(dirname "$0")/.."

# Handle --latest flag
if [[ "$1" == "--latest" ]]; then
    shift
    LATEST_DIR=$(ls -td output/2026-*/ 2>/dev/null | head -1)
    if [[ -z "$LATEST_DIR" ]]; then
        echo "No evolution output found in output/"
        exit 1
    fi
    GENOME=$(ls "$LATEST_DIR"rank01_*.json 2>/dev/null | head -1)
    if [[ -z "$GENOME" ]]; then
        echo "No rank01 genome found in $LATEST_DIR"
        exit 1
    fi
    echo "Using: $GENOME"
    exec uv run python -m darwindeck.cli.playtest "$GENOME" "$@"
fi

# Pass all args through
exec uv run python -m darwindeck.cli.playtest "$@"
