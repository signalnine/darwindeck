#!/bin/bash
# Run evolution on the server with optimal parallelization

set -e

# Change to project directory
cd "$(dirname "$0")/.."

# Detect CPU count
CPU_COUNT=$(nproc)
echo "🖥️  Detected $CPU_COUNT CPU cores"

# Calculate optimal worker count
# Use all cores for Python-level parallelism
WORKERS=$CPU_COUNT
echo "🔧 Using $WORKERS parallel workers"

# Evolution parameters
POPULATION=${POPULATION:-500}  # Larger population for beefy server
GENERATIONS=${GENERATIONS:-100}
STYLE=${STYLE:-balanced}
OUTPUT_DIR=${OUTPUT_DIR:-"output/evolution-$(date +%Y%m%d-%H%M%S)"}

echo ""
echo "🧬 Evolution Configuration:"
echo "  Population size: $POPULATION"
echo "  Generations: $GENERATIONS"
echo "  Style: $STYLE"
echo "  Workers: $WORKERS"
echo "  Output directory: $OUTPUT_DIR"
echo ""

# Run evolution
echo "🚀 Starting evolution..."
export PYTHONUNBUFFERED=1
export EVOLUTION_WORKERS=$WORKERS

uv run python -m darwindeck.cli.evolve \
    --population-size $POPULATION \
    --generations $GENERATIONS \
    --style $STYLE \
    --output-dir "$OUTPUT_DIR" \
    --save-top-n 20 \
    --verbose

echo ""
echo "✅ Evolution complete! Results saved to $OUTPUT_DIR"
