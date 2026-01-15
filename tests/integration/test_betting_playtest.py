"""Integration tests for betting in playtest."""

import pytest
from darwindeck.playtest.session import PlaytestSession, SessionConfig
from darwindeck.simulation.state import PlayerState, GameState
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase, PlayPhase, Location
)


class TestBettingPlaytest:
    """Test betting games can be playtested."""

    def _make_betting_genome(self) -> GameGenome:
        """Create a simple betting game genome."""
        return GameGenome(
            schema_version="1.0",
            genome_id="test_betting",
            generation=0,
            setup=SetupRules(cards_per_player=2, starting_chips=500),
            turn_structure=TurnStructure(
                phases=(BettingPhase(min_bet=10, max_raises=3),),
            ),
            special_effects=[],
            win_conditions=[WinCondition(type="high_score")],
            scoring_rules=[],
            player_count=2,
        )

    def test_betting_moves_generated_for_betting_phase(self):
        """Session should generate betting moves for BettingPhase."""
        genome = self._make_betting_genome()
        config = SessionConfig(seed=12345)
        session = PlaytestSession(genome, config)
        session.state = session._initialize_state()

        # Import here to get the updated function
        from darwindeck.simulation.movegen import generate_legal_moves, BettingMove

        moves = generate_legal_moves(session.state, genome)

        # Should have betting moves, not empty
        assert len(moves) > 0
        assert all(isinstance(m, BettingMove) for m in moves)
