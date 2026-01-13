#!/bin/bash
cd /home/gabe/cards-playtest
/home/gabe/.local/bin/poetry run python -m darwindeck.cli.evolve --population-size 100 --generations 5 --verbose
