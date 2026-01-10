#!/bin/bash
# Setup environment on the server

set -e

echo "🔧 Setting up server environment..."

# Check Python version
echo "Checking Python..."
python3 --version

# Install poetry if not installed
if ! command -v poetry &> /dev/null; then
    echo "Installing Poetry..."
    curl -sSL https://install.python-poetry.org | python3 -
    export PATH="$HOME/.local/bin:$PATH"
fi

# Install dependencies
echo "Installing Python dependencies..."
poetry install --no-dev

# Verify Go simulator is present
if [ ! -f "libcardsim.so" ]; then
    echo "❌ Error: libcardsim.so not found!"
    echo "Run deploy-to-server.sh from your local machine first."
    exit 1
fi

echo "✅ Server setup complete!"
echo ""
echo "System info:"
echo "  CPUs: $(nproc)"
echo "  RAM: $(free -h | grep Mem | awk '{print $2}')"
echo "  Python: $(python3 --version)"
echo "  Poetry: $(poetry --version)"
