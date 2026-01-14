"""Tests for RuleExplainer."""

import pytest
from darwindeck.playtest.rules import RuleExplainer
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    PlayPhase, DrawPhase, BettingPhase, Location
)


def make_genome(
    name: str = "TestGame",
    phases: list = None,
    win_type: str = "empty_hand",
    starting_chips: int = 0,
) -> GameGenome:
    """Create test genome."""
    if phases is None:
        phases = [PlayPhase(target=Location.DISCARD)]

    return GameGenome(
        schema_version="1.0",
        genome_id=name,
        generation=1,
        setup=SetupRules(cards_per_player=5, starting_chips=starting_chips),
        turn_structure=TurnStructure(phases=phases),
        special_effects=[],
        win_conditions=[WinCondition(type=win_type)],
        scoring_rules=[],
        max_turns=100,
        min_turns=1,
        player_count=2,
    )


class TestRuleExplainer:
    """Tests for RuleExplainer."""

    def test_explains_game_name(self):
        """Shows game name in rules."""
        explainer = RuleExplainer()
        genome = make_genome(name="MyGame")

        output = explainer.explain_rules(genome)

        assert "MyGame" in output

    def test_explains_win_condition_empty_hand(self):
        """Explains empty hand win condition."""
        explainer = RuleExplainer()
        genome = make_genome(win_type="empty_hand")

        output = explainer.explain_rules(genome)

        assert "empty" in output.lower() or "hand" in output.lower()

    def test_explains_win_condition_capture_all(self):
        """Explains capture all win condition."""
        explainer = RuleExplainer()
        genome = make_genome(win_type="capture_all")

        output = explainer.explain_rules(genome)

        assert "capture" in output.lower() or "all" in output.lower()

    def test_explains_play_phase(self):
        """Explains play card phases."""
        explainer = RuleExplainer()
        genome = make_genome(phases=[PlayPhase(target=Location.DISCARD)])

        output = explainer.explain_rules(genome)

        assert "play" in output.lower() or "card" in output.lower()

    def test_explains_betting(self):
        """Shows betting info if chips > 0."""
        explainer = RuleExplainer()
        genome = make_genome(
            starting_chips=1000,
            phases=[BettingPhase(min_bet=10)]
        )

        output = explainer.explain_rules(genome)

        assert "bet" in output.lower() or "chip" in output.lower()

    def test_explains_phase_during_game(self):
        """Explains current phase."""
        explainer = RuleExplainer()
        genome = make_genome(phases=[
            DrawPhase(source=Location.DECK),
            PlayPhase(target=Location.DISCARD),
        ])

        # Explain phase 0 (draw)
        output0 = explainer.explain_phase(0, genome)
        assert "draw" in output0.lower()

        # Explain phase 1 (play)
        output1 = explainer.explain_phase(1, genome)
        assert "play" in output1.lower()
