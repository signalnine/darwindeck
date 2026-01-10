#!/bin/bash
# Deploy cards-playtest to remote server

set -e

SERVER="192.168.1.15"
REMOTE_DIR="/home/gabe/cards-playtest"
LOCAL_DIR="/home/gabe/cards-playtest"

echo "🚀 Deploying to $SERVER..."

# Create remote directory if it doesn't exist
ssh "$SERVER" "mkdir -p $REMOTE_DIR"

# Sync code (excluding build artifacts, cache, etc.)
echo "📦 Syncing code..."
rsync -avz --progress \
    --exclude '.git' \
    --exclude '__pycache__' \
    --exclude '*.pyc' \
    --exclude '.pytest_cache' \
    --exclude '.hypothesis' \
    --exclude 'output' \
    --exclude '.venv' \
    --exclude 'node_modules' \
    "$LOCAL_DIR/" "$SERVER:$REMOTE_DIR/"

# Sync the compiled Go simulator
echo "📦 Syncing compiled Go simulator..."
rsync -avz --progress \
    "$LOCAL_DIR/libcardsim.so" "$SERVER:$REMOTE_DIR/"

echo "✅ Deployment complete!"
echo ""
echo "Next steps:"
echo "  ssh $SERVER"
echo "  cd $REMOTE_DIR"
echo "  ./scripts/setup-server.sh"
echo "  ./scripts/run-evolution.sh"
