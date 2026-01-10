#!/bin/bash
# Run evolution on the server with optimal parallelization

set -e

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
OUTPUT_DIR=${OUTPUT_DIR:-"output/evolution-$(date +%Y%m%d-%H%M%S)"}

echo ""
echo "🧬 Evolution Configuration:"
echo "  Population size: $POPULATION"
echo "  Generations: $GENERATIONS"
echo "  Workers: $WORKERS"
echo "  Output directory: $OUTPUT_DIR"
echo ""

# Run evolution
echo "🚀 Starting evolution..."
export PYTHONUNBUFFERED=1
export EVOLUTION_WORKERS=$WORKERS

poetry run python -m cards_evolve.cli.evolve \
    --population-size $POPULATION \
    --generations $GENERATIONS \
    --output-dir "$OUTPUT_DIR" \
    --save-top-n 20 \
    --verbose

echo ""
echo "✅ Evolution complete! Results saved to $OUTPUT_DIR"
