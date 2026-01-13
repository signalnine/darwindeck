#!/bin/bash
# Quick deploy and test
set -e
SERVER="192.168.1.15"
REMOTE_DIR="/home/gabe/cards-playtest"

echo "Pulling latest changes on server..."
ssh "$SERVER" "cd $REMOTE_DIR && git pull origin master"

echo "Running quick evolution test..."
ssh "$SERVER" "cd $REMOTE_DIR && /home/gabe/.local/bin/poetry run python -m darwindeck.cli.evolve --population-size 10 --generations 2 --verbose"
