"""Tests for genome interpreter."""

import pytest
from cards_evolve.simulation.interpreter import GenomeInterpreter, GameLogic
from cards_evolve.genome.examples import create_war_genome
from cards_evolve.simulation.state import GameState


def test_interpreter_creates_game_logic() -> None:
    """Test interpreter converts genome to GameLogic."""
    genome = create_war_genome()
    interpreter = GenomeInterpreter()

    logic = interpreter.to_executable(genome)

    assert isinstance(logic, GameLogic)


def test_game_logic_creates_initial_state() -> None:
    """Test GameLogic can create initial game state."""
    genome = create_war_genome()
    interpreter = GenomeInterpreter()
    logic = interpreter.to_executable(genome)

    state = logic.create_initial_state(seed=42)

    assert isinstance(state, GameState)
    assert len(state.players) == 2
    assert len(state.players[0].hand) == 26
    assert len(state.players[1].hand) == 26


def test_game_logic_deterministic() -> None:
    """Test same seed produces same initial state."""
    genome = create_war_genome()
    interpreter = GenomeInterpreter()
    logic = interpreter.to_executable(genome)

    state1 = logic.create_initial_state(seed=42)
    state2 = logic.create_initial_state(seed=42)

    assert state1.players[0].hand == state2.players[0].hand
    assert state1.players[1].hand == state2.players[1].hand
