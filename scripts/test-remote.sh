#!/bin/bash
ssh 192.168.1.15 "cd /home/gabe/cards-playtest && /home/gabe/.local/bin/poetry run python -m darwindeck.cli.evolve --population-size 10 --generations 2 --verbose"
