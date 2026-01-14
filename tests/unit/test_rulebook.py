"""Tests for rulebook generation."""

import pytest
from darwindeck.evolution.rulebook import (
    RulebookSections, GenomeValidator, GenomeExtractor, ValidationResult,
    RulebookGenerator, RulebookEnhancer
)
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    PlayPhase, DrawPhase, BettingPhase, DiscardPhase, TrickPhase, ClaimPhase,
    Location, Suit, SpecialEffect, EffectType, TargetSelector, Rank
)


class TestRulebookSections:
    """Tests for the RulebookSections dataclass."""

    def test_rulebook_sections_creation(self):
        """RulebookSections can be created with all fields."""
        sections = RulebookSections(
            game_name="TestGame",
            player_count=2,
            overview="A test game.",
            components=["Standard 52-card deck"],
            setup_steps=["Shuffle the deck", "Deal 5 cards to each player"],
            objective="First to empty hand wins",
            phases=[("Draw", "Draw 1 card from the deck")],
            special_rules=[],
            edge_cases=["Reshuffle discard when deck empty"],
            quick_reference="Draw -> Play -> Win"
        )
        assert sections.game_name == "TestGame"
        assert len(sections.setup_steps) == 2
        assert len(sections.phases) == 1

    def test_rulebook_sections_defaults(self):
        """RulebookSections has sensible defaults."""
        sections = RulebookSections(
            game_name="Minimal",
            player_count=2,
            objective="Win the game"
        )
        assert sections.overview is None
        assert sections.components == []
        assert sections.edge_cases == []


class TestGenomeValidator:
    """Tests for pre-extraction genome validation."""

    def _make_genome(self, cards_per_player=5, player_count=2, starting_chips=0,
                     phases=None, win_conditions=None):
        """Helper to create test genomes."""
        if phases is None:
            phases = [DrawPhase(source=Location.DECK, count=1)]
        if win_conditions is None:
            win_conditions = [WinCondition(type="empty_hand")]
        return GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=cards_per_player, starting_chips=starting_chips),
            turn_structure=TurnStructure(phases=phases),
            special_effects=[],
            win_conditions=win_conditions,
            player_count=player_count,
            scoring_rules=[],
        )

    def test_valid_genome_passes(self):
        """A valid genome passes validation."""
        genome = self._make_genome()
        result = GenomeValidator().validate(genome)
        assert result.valid is True
        assert result.errors == []

    def test_too_many_cards_fails(self):
        """Dealing more cards than deck has fails."""
        genome = self._make_genome(cards_per_player=30, player_count=2)  # 60 > 52
        result = GenomeValidator().validate(genome)
        assert result.valid is False
        assert any("cards" in e.lower() for e in result.errors)

    def test_betting_without_chips_fails(self):
        """BettingPhase with 0 starting chips fails."""
        genome = self._make_genome(
            starting_chips=0,
            phases=[BettingPhase(min_bet=10)]
        )
        result = GenomeValidator().validate(genome)
        assert result.valid is False
        assert any("chip" in e.lower() for e in result.errors)

    def test_no_phases_fails(self):
        """Empty turn structure fails."""
        genome = self._make_genome(phases=[])
        result = GenomeValidator().validate(genome)
        assert result.valid is False
        assert any("phase" in e.lower() for e in result.errors)

    def test_no_win_conditions_fails(self):
        """No win conditions fails."""
        genome = self._make_genome(win_conditions=[])
        result = GenomeValidator().validate(genome)
        assert result.valid is False
        assert any("win" in e.lower() for e in result.errors)


class TestGenomeExtractor:
    """Tests for deterministic rule extraction."""

    def _make_genome(self, cards_per_player=5, starting_chips=0,
                     initial_discard_count=0, win_conditions=None, phases=None):
        """Helper to create test genomes."""
        if win_conditions is None:
            win_conditions = [WinCondition(type="empty_hand")]
        if phases is None:
            phases = [DrawPhase(source=Location.DECK, count=1)]
        return GameGenome(
            schema_version="1.0",
            genome_id="TestGame",
            generation=1,
            setup=SetupRules(
                cards_per_player=cards_per_player,
                starting_chips=starting_chips,
                initial_discard_count=initial_discard_count
            ),
            turn_structure=TurnStructure(phases=phases),
            special_effects=[],
            win_conditions=win_conditions,
            player_count=2,
            scoring_rules=[],
        )

    def test_extract_basic_setup(self):
        """Extracts basic setup steps."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(cards_per_player=7)
        sections = GenomeExtractor().extract(genome)

        assert "Shuffle the deck" in sections.setup_steps
        assert any("7 cards" in step for step in sections.setup_steps)

    def test_extract_setup_with_chips(self):
        """Includes chips in setup when starting_chips > 0."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(starting_chips=1000)
        sections = GenomeExtractor().extract(genome)

        assert any("1000" in step and "chip" in step.lower() for step in sections.setup_steps)

    def test_extract_setup_with_discard(self):
        """Includes initial discard when present."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(initial_discard_count=1)
        sections = GenomeExtractor().extract(genome)

        assert any("discard" in step.lower() for step in sections.setup_steps)

    def test_extract_empty_hand_objective(self):
        """Extracts empty_hand win condition."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(win_conditions=[WinCondition(type="empty_hand")])
        sections = GenomeExtractor().extract(genome)

        assert "empty" in sections.objective.lower()

    def test_extract_high_score_objective(self):
        """Extracts high_score win condition."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(win_conditions=[WinCondition(type="high_score")])
        sections = GenomeExtractor().extract(genome)

        assert "score" in sections.objective.lower() or "points" in sections.objective.lower()

    def test_extract_multiple_win_conditions(self):
        """Handles multiple win conditions."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(win_conditions=[
            WinCondition(type="empty_hand"),
            WinCondition(type="capture_all")
        ])
        sections = GenomeExtractor().extract(genome)

        assert "empty" in sections.objective.lower() or "capture" in sections.objective.lower()

    def test_extract_draw_phase(self):
        """Extracts DrawPhase correctly."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(phases=[
            DrawPhase(source=Location.DECK, count=2)
        ])
        sections = GenomeExtractor().extract(genome)

        assert len(sections.phases) == 1
        name, desc = sections.phases[0]
        assert "draw" in name.lower()
        assert "2" in desc

    def test_extract_play_phase(self):
        """Extracts PlayPhase correctly."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(phases=[
            PlayPhase(target=Location.DISCARD, min_cards=1, max_cards=1)
        ])
        sections = GenomeExtractor().extract(genome)

        assert len(sections.phases) == 1
        name, desc = sections.phases[0]
        assert "play" in name.lower()

    def test_extract_betting_phase(self):
        """Extracts BettingPhase correctly."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(
            starting_chips=1000,
            phases=[BettingPhase(min_bet=25, max_raises=3)]
        )
        sections = GenomeExtractor().extract(genome)

        assert len(sections.phases) == 1
        name, desc = sections.phases[0]
        assert "bet" in name.lower()
        assert "25" in desc

    def test_extract_multiple_phases(self):
        """Extracts multiple phases in order."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(phases=[
            DrawPhase(source=Location.DECK, count=1),
            PlayPhase(target=Location.DISCARD, min_cards=1, max_cards=3),
        ])
        sections = GenomeExtractor().extract(genome)

        assert len(sections.phases) == 2
        assert "draw" in sections.phases[0][0].lower()
        assert "play" in sections.phases[1][0].lower()

    def test_extract_discard_phase(self):
        """Extracts DiscardPhase correctly."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(phases=[
            DiscardPhase(target=Location.DISCARD, count=2, mandatory=False)
        ])
        sections = GenomeExtractor().extract(genome)

        assert len(sections.phases) == 1
        name, desc = sections.phases[0]
        assert "discard" in name.lower()
        assert "2" in desc
        assert "optional" in desc.lower()

    def test_extract_trick_phase(self):
        """Extracts TrickPhase correctly."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(phases=[
            TrickPhase(lead_suit_required=True, high_card_wins=True)
        ])
        sections = GenomeExtractor().extract(genome)

        assert len(sections.phases) == 1
        name, desc = sections.phases[0]
        assert "trick" in name.lower()
        assert "suit" in desc.lower()

    def test_extract_claim_phase(self):
        """Extracts ClaimPhase correctly."""
        from darwindeck.evolution.rulebook import GenomeExtractor
        genome = self._make_genome(phases=[
            ClaimPhase(min_cards=1, max_cards=4, sequential_rank=True, allow_challenge=True)
        ])
        sections = GenomeExtractor().extract(genome)

        assert len(sections.phases) == 1
        name, desc = sections.phases[0]
        assert "claim" in name.lower()
        assert "challenge" in desc.lower()

    def test_extract_skip_effect(self):
        """Extracts skip next player effect."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[DrawPhase(source=Location.DECK)]),
            special_effects=[
                SpecialEffect(trigger_rank=Rank.EIGHT, effect_type=EffectType.SKIP_NEXT, target=TargetSelector.NEXT_PLAYER)
            ],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
            scoring_rules=[],
        )
        sections = GenomeExtractor().extract(genome)

        assert len(sections.special_rules) >= 1
        assert any("8" in rule or "eight" in rule.lower() for rule in sections.special_rules)
        assert any("skip" in rule.lower() for rule in sections.special_rules)

    def test_extract_reverse_effect(self):
        """Extracts reverse direction effect."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[DrawPhase(source=Location.DECK)]),
            special_effects=[
                SpecialEffect(trigger_rank=Rank.ACE, effect_type=EffectType.REVERSE_DIRECTION, target=TargetSelector.ALL_OPPONENTS)
            ],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
            scoring_rules=[],
        )
        sections = GenomeExtractor().extract(genome)

        assert any("reverse" in rule.lower() for rule in sections.special_rules)

    def test_extract_draw_cards_effect(self):
        """Extracts draw cards effect."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[DrawPhase(source=Location.DECK)]),
            special_effects=[
                SpecialEffect(trigger_rank=Rank.TWO, effect_type=EffectType.DRAW_CARDS, target=TargetSelector.NEXT_PLAYER, value=2)
            ],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
            scoring_rules=[],
        )
        sections = GenomeExtractor().extract(genome)

        assert any("draw" in rule.lower() and "2" in rule for rule in sections.special_rules)

    def test_extract_extra_turn_effect(self):
        """Extracts extra turn effect."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[DrawPhase(source=Location.DECK)]),
            special_effects=[
                # EXTRA_TURN affects self, but schema uses target for consistency - use NEXT_PLAYER as placeholder
                SpecialEffect(trigger_rank=Rank.KING, effect_type=EffectType.EXTRA_TURN, target=TargetSelector.NEXT_PLAYER)
            ],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
            scoring_rules=[],
        )
        sections = GenomeExtractor().extract(genome)

        assert any("extra" in rule.lower() and "turn" in rule.lower() for rule in sections.special_rules)

    def test_extract_force_discard_effect(self):
        """Extracts force discard effect."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[DrawPhase(source=Location.DECK)]),
            special_effects=[
                SpecialEffect(trigger_rank=Rank.JACK, effect_type=EffectType.FORCE_DISCARD, target=TargetSelector.NEXT_PLAYER, value=2)
            ],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
            scoring_rules=[],
        )
        sections = GenomeExtractor().extract(genome)

        assert any("discard" in rule.lower() and "2" in rule for rule in sections.special_rules)

    def test_extract_wild_cards(self):
        """Extracts wild cards from setup."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=5, wild_cards=(Rank.JACK, Rank.QUEEN)),
            turn_structure=TurnStructure(phases=[DrawPhase(source=Location.DECK)]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
            scoring_rules=[],
        )
        sections = GenomeExtractor().extract(genome)

        assert any("wild" in rule.lower() for rule in sections.special_rules)
        assert any("jack" in rule.lower() or "queen" in rule.lower() for rule in sections.special_rules)


class TestEdgeCaseDefaults:
    """Tests for genome-conditional edge case defaults."""

    def _make_genome(self, win_conditions=None, phases=None, starting_chips=0):
        if win_conditions is None:
            win_conditions = [WinCondition(type="empty_hand")]
        if phases is None:
            phases = [DrawPhase(source=Location.DECK, count=1)]
        return GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=5, starting_chips=starting_chips),
            turn_structure=TurnStructure(phases=phases),
            special_effects=[],
            win_conditions=win_conditions,
            player_count=2,
            scoring_rules=[],
        )

    def test_deck_exhaustion_default_included(self):
        """Deck exhaustion default included for normal games."""
        from darwindeck.evolution.rulebook import select_applicable_defaults

        genome = self._make_genome()
        defaults = select_applicable_defaults(genome)

        assert any(d.name == "deck_exhaustion" for d in defaults)

    def test_deck_exhaustion_skipped_when_win_condition(self):
        """Deck exhaustion default skipped if it's a win condition."""
        from darwindeck.evolution.rulebook import select_applicable_defaults

        genome = self._make_genome(win_conditions=[WinCondition(type="deck_empty")])
        defaults = select_applicable_defaults(genome)

        assert not any(d.name == "deck_exhaustion" for d in defaults)

    def test_betting_defaults_only_with_betting(self):
        """Betting defaults only included when BettingPhase exists."""
        from darwindeck.evolution.rulebook import select_applicable_defaults

        # Without betting
        genome_no_bet = self._make_genome()
        defaults_no_bet = select_applicable_defaults(genome_no_bet)
        assert not any("betting" in d.name for d in defaults_no_bet)

        # With betting
        genome_bet = self._make_genome(
            starting_chips=1000,
            phases=[BettingPhase(min_bet=10)]
        )
        defaults_bet = select_applicable_defaults(genome_bet)
        assert any("betting" in d.name for d in defaults_bet)

    def test_hand_limit_skipped_for_capture_games(self):
        """Hand limit not applied to capture/accumulation games."""
        from darwindeck.evolution.rulebook import select_applicable_defaults

        genome = self._make_genome(win_conditions=[WinCondition(type="capture_all")])
        defaults = select_applicable_defaults(genome)

        assert not any(d.name == "hand_limit" for d in defaults)


class TestRulebookGenerator:
    """Tests for markdown rulebook generation."""

    def _make_genome(self):
        return GameGenome(
            schema_version="1.0",
            genome_id="TestGame",
            generation=1,
            setup=SetupRules(cards_per_player=7),
            turn_structure=TurnStructure(phases=[
                DrawPhase(source=Location.DECK, count=1),
                PlayPhase(target=Location.DISCARD, min_cards=1, max_cards=1),
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
            scoring_rules=[],
        )

    def test_render_markdown_has_all_sections(self):
        """Rendered markdown has all required sections."""
        genome = self._make_genome()
        generator = RulebookGenerator()
        markdown = generator.generate(genome)

        assert "# TestGame" in markdown
        assert "## Components" in markdown
        assert "## Setup" in markdown
        assert "## Objective" in markdown
        assert "## Turn Structure" in markdown
        assert "## Edge Cases" in markdown

    def test_render_markdown_includes_setup_steps(self):
        """Setup steps are included in markdown."""
        genome = self._make_genome()
        markdown = RulebookGenerator().generate(genome)

        assert "Shuffle the deck" in markdown
        assert "7 cards" in markdown

    def test_render_markdown_includes_phases(self):
        """Phases are included in markdown."""
        genome = self._make_genome()
        markdown = RulebookGenerator().generate(genome)

        assert "Draw" in markdown
        assert "Play" in markdown

    def test_generate_basic_mode(self):
        """Basic mode works without LLM."""
        genome = self._make_genome()
        markdown = RulebookGenerator().generate(genome, use_llm=False)

        assert "# TestGame" in markdown
        assert "## Overview" not in markdown or "Overview" in markdown  # May have template overview

    def test_generate_raises_on_invalid_genome(self):
        """Generate raises ValueError on invalid genome."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="InvalidGame",
            generation=1,
            setup=SetupRules(cards_per_player=30),  # 60 cards for 2 players > 52
            turn_structure=TurnStructure(phases=[
                DrawPhase(source=Location.DECK, count=1),
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
            scoring_rules=[],
        )
        with pytest.raises(ValueError, match="Invalid genome"):
            RulebookGenerator().generate(genome)

    def test_generate_includes_edge_cases(self):
        """Generated markdown includes edge case defaults."""
        genome = self._make_genome()
        markdown = RulebookGenerator().generate(genome)

        # Should include common edge cases like deck exhaustion, turn limit
        assert "Empty deck" in markdown or "Turn limit" in markdown or "Tie" in markdown


class TestRulebookEnhancer:
    """Tests for LLM enhancement."""

    def _make_sections(self):
        return RulebookSections(
            game_name="TestGame",
            player_count=2,
            objective="Empty your hand to win",
            components=["Standard 52-card deck"],
            setup_steps=["Deal 5 cards"],
            phases=[("Draw", "Draw 1 card")],
        )

    @pytest.fixture
    def mock_anthropic(self, monkeypatch):
        """Mock the anthropic module."""
        from unittest.mock import MagicMock
        mock_module = MagicMock()
        monkeypatch.setattr("darwindeck.evolution.rulebook.anthropic", mock_module)
        return mock_module

    def test_enhance_without_api_key_returns_unchanged(self, monkeypatch):
        """Without API key, sections returned unchanged."""
        monkeypatch.setenv("ANTHROPIC_API_KEY", "")
        sections = self._make_sections()
        enhanced = RulebookEnhancer().enhance(sections, None)

        assert enhanced.overview is None  # Not enhanced

    def test_enhance_adds_overview(self, monkeypatch, mock_anthropic):
        """LLM enhancement adds overview."""
        from unittest.mock import MagicMock

        monkeypatch.setenv("ANTHROPIC_API_KEY", "test-key")

        mock_client = MagicMock()
        mock_anthropic.Anthropic.return_value = mock_client
        mock_client.messages.create.return_value = MagicMock(
            content=[MagicMock(text="A fun card game about emptying your hand.")]
        )

        sections = self._make_sections()
        genome = GameGenome(
            schema_version="1.0",
            genome_id="TestGame",
            generation=1,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[DrawPhase(source=Location.DECK)]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
            scoring_rules=[],
        )

        enhanced = RulebookEnhancer().enhance(sections, genome)

        assert enhanced.overview is not None
        assert "fun" in enhanced.overview.lower() or "card" in enhanced.overview.lower()

    def test_enhance_handles_api_error_gracefully(self, monkeypatch, mock_anthropic):
        """LLM errors return unchanged sections."""
        monkeypatch.setenv("ANTHROPIC_API_KEY", "test-key")

        mock_anthropic.Anthropic.side_effect = Exception("API error")

        sections = self._make_sections()
        enhanced = RulebookEnhancer().enhance(sections, None)

        # Should return original sections unchanged
        assert enhanced.overview is None
        assert enhanced.game_name == "TestGame"
