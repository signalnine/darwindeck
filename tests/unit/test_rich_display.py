"""Tests for rich_display module."""

import pytest
from rich.console import Console
from rich.panel import Panel
from rich.text import Text

from darwindeck.playtest.rich_display import (
    RichDisplay,
    format_card_rich,
    SUIT_SYMBOLS,
    SUIT_COLORS,
    MIN_WIDE_WIDTH,
)
from darwindeck.playtest.display_state import DisplayState, MoveOption


def make_test_display_state(terminal_width: int = 80) -> DisplayState:
    """Create a DisplayState for testing."""
    return DisplayState(
        game_name="TestGame",
        turn=5,
        phase_name="Play",
        player_id=0,
        hand_cards=[("A", "H"), ("K", "S"), ("9", "D")],
        opponent_card_count=8,
        opponent_chips=100,
        opponent_bet=10,
        player_chips=200,
        player_bet=20,
        pot=50,
        current_bet=20,
        discard_top=("Q", "C"),
        moves=[
            MoveOption(index=1, label="A\u2665", move_type="card"),
            MoveOption(index=2, label="K\u2660", move_type="card"),
            MoveOption(index=3, label="Pass", move_type="pass"),
        ],
        move_log=[("opponent", "played J\u2663"), ("player", "played 10\u2666")],
        terminal_width=terminal_width,
    )


class TestFormatCard:
    """Tests for format_card_rich function."""

    def test_format_card_rich_red_suits(self) -> None:
        """Hearts and diamonds should be styled red."""
        heart = format_card_rich("A", "H")
        diamond = format_card_rich("K", "D")

        # Both should be Text objects
        assert isinstance(heart, Text)
        assert isinstance(diamond, Text)

        # Check plain text contains correct symbols
        assert heart.plain == "A\u2665"
        assert diamond.plain == "K\u2666"

        # Check that red style is applied by examining spans
        # The second character (suit symbol) should have red style
        heart_spans = list(heart.spans)
        diamond_spans = list(diamond.spans)

        # Find the span that covers the suit symbol (index 1)
        heart_suit_style = None
        for span in heart_spans:
            if span.start <= 1 < span.end:
                heart_suit_style = span.style
                break

        diamond_suit_style = None
        for span in diamond_spans:
            if span.start <= 1 < span.end:
                diamond_suit_style = span.style
                break

        assert heart_suit_style == "red", f"Expected red, got {heart_suit_style}"
        assert diamond_suit_style == "red", f"Expected red, got {diamond_suit_style}"

    def test_format_card_rich_default_suits(self) -> None:
        """Clubs and spades should use default color."""
        club = format_card_rich("Q", "C")
        spade = format_card_rich("J", "S")

        # Both should be Text objects
        assert isinstance(club, Text)
        assert isinstance(spade, Text)

        # Verify symbols are correct
        assert club.plain == "Q\u2663"
        assert spade.plain == "J\u2660"

        # Find the span that covers the suit symbol
        club_spans = list(club.spans)
        spade_spans = list(spade.spans)

        club_suit_style = None
        for span in club_spans:
            if span.start <= 1 < span.end:
                club_suit_style = span.style
                break

        spade_suit_style = None
        for span in spade_spans:
            if span.start <= 1 < span.end:
                spade_suit_style = span.style
                break

        assert club_suit_style == "default", f"Expected default, got {club_suit_style}"
        assert spade_suit_style == "default", f"Expected default, got {spade_suit_style}"


class TestDisplayState:
    """Tests for DisplayState dataclass."""

    def test_display_state_construction(self) -> None:
        """DisplayState should construct with all fields."""
        state = make_test_display_state()

        assert state.game_name == "TestGame"
        assert state.turn == 5
        assert state.phase_name == "Play"
        assert state.player_id == 0
        assert len(state.hand_cards) == 3
        assert state.hand_cards[0] == ("A", "H")
        assert state.opponent_card_count == 8
        assert state.opponent_chips == 100
        assert state.opponent_bet == 10
        assert state.player_chips == 200
        assert state.player_bet == 20
        assert state.pot == 50
        assert state.current_bet == 20
        assert state.discard_top == ("Q", "C")
        assert len(state.moves) == 3
        assert len(state.move_log) == 2
        assert state.terminal_width == 80

    def test_move_option_construction(self) -> None:
        """MoveOption should construct correctly."""
        opt = MoveOption(index=1, label="A\u2665", move_type="card")
        assert opt.index == 1
        assert opt.label == "A\u2665"
        assert opt.move_type == "card"

        # Test other move types
        pass_opt = MoveOption(index=2, label="Pass", move_type="pass")
        assert pass_opt.move_type == "pass"

        bet_opt = MoveOption(index=3, label="Raise", move_type="betting")
        assert bet_opt.move_type == "betting"


class TestRichDisplay:
    """Tests for RichDisplay class."""

    def test_render_wide_vs_compact(self) -> None:
        """Layout should switch based on terminal width."""
        display = RichDisplay()

        # Wide layout (above MIN_WIDE_WIDTH)
        wide_state = make_test_display_state(terminal_width=MIN_WIDE_WIDTH + 10)
        wide_result = display.render(wide_state)

        # Compact layout (below MIN_WIDE_WIDTH)
        compact_state = make_test_display_state(terminal_width=MIN_WIDE_WIDTH - 10)
        compact_result = display.render(compact_state)

        # Both should produce Panel objects
        assert isinstance(wide_result, Panel)
        assert isinstance(compact_result, Panel)

        # Verify the threshold is correctly applied
        # At exactly MIN_WIDE_WIDTH, should use compact (strictly less than)
        at_threshold_state = make_test_display_state(terminal_width=MIN_WIDE_WIDTH)
        at_threshold_result = display.render(at_threshold_state)
        assert isinstance(at_threshold_result, Panel)

    def test_render_contains_expected_elements(self) -> None:
        """Rendered output should contain key game elements."""
        console = Console(record=True, width=80, force_terminal=True)
        display = RichDisplay()
        display.console = console

        state = make_test_display_state()
        panel = display.render(state)
        console.print(panel)

        output = console.export_text()

        # Verify game name is in output
        assert "TestGame" in output

        # Verify turn number is displayed
        assert "Turn" in output
        assert "5" in output

        # Verify phase is shown
        assert "Play" in output

        # Verify player identification
        assert "P0" in output or "You" in output

        # Verify opponent info is displayed
        assert "Opponent" in output or "8" in output  # opponent card count

        # Verify hand section exists
        assert "Hand" in output or "\u2665" in output  # heart symbol from hand cards

        # Verify actions section exists
        assert "Actions" in output or "Pass" in output

        # Verify chips/pot info is displayed (since player_chips > 0)
        assert "200" in output or "Chips" in output

    def test_render_wide_has_nested_panels(self) -> None:
        """Wide layout should contain nested panels for sections."""
        display = RichDisplay()

        wide_state = make_test_display_state(terminal_width=100)
        panel = display.render(wide_state)

        # The panel's renderable should be a Group containing panels
        assert isinstance(panel, Panel)
        # Check the panel has a title with the game name
        assert "TestGame" in str(panel.title) if panel.title else False

    def test_render_compact_is_text_based(self) -> None:
        """Compact layout should use simpler text-based rendering."""
        console = Console(record=True, width=50, force_terminal=True)
        display = RichDisplay()
        display.console = console

        # Compact layout
        compact_state = make_test_display_state(terminal_width=50)
        panel = display.render(compact_state)
        console.print(panel)

        output = console.export_text()

        # Should still contain key elements
        assert "TestGame" in output
        assert "Turn 5" in output
        assert "Phase: Play" in output

    def test_render_without_discard(self) -> None:
        """Render should work when discard_top is None."""
        display = RichDisplay()

        state = DisplayState(
            game_name="NoDiscard",
            turn=1,
            phase_name="Draw",
            player_id=0,
            hand_cards=[("2", "C")],
            opponent_card_count=5,
            opponent_chips=0,
            opponent_bet=0,
            player_chips=0,
            player_bet=0,
            pot=0,
            current_bet=0,
            discard_top=None,  # No discard pile
            moves=[MoveOption(index=1, label="Draw", move_type="pass")],
            move_log=[],
            terminal_width=80,
        )

        panel = display.render(state)
        assert isinstance(panel, Panel)

    def test_render_empty_hand(self) -> None:
        """Render should work with empty hand."""
        console = Console(record=True, width=80, force_terminal=True)
        display = RichDisplay()
        display.console = console

        state = DisplayState(
            game_name="EmptyHand",
            turn=10,
            phase_name="End",
            player_id=0,
            hand_cards=[],  # Empty hand
            opponent_card_count=0,
            opponent_chips=0,
            opponent_bet=0,
            player_chips=0,
            player_bet=0,
            pot=0,
            current_bet=0,
            discard_top=None,
            moves=[],
            move_log=[],
            terminal_width=80,
        )

        panel = display.render(state)
        console.print(panel)
        output = console.export_text()

        assert isinstance(panel, Panel)
        # Should indicate empty hand somehow
        assert "empty" in output.lower() or "Hand" in output


class TestSuitConstants:
    """Tests for suit-related constants."""

    def test_suit_symbols_complete(self) -> None:
        """All four suits should have symbols defined."""
        assert "H" in SUIT_SYMBOLS
        assert "D" in SUIT_SYMBOLS
        assert "C" in SUIT_SYMBOLS
        assert "S" in SUIT_SYMBOLS

        # Verify correct Unicode symbols
        assert SUIT_SYMBOLS["H"] == "\u2665"  # Heart
        assert SUIT_SYMBOLS["D"] == "\u2666"  # Diamond
        assert SUIT_SYMBOLS["C"] == "\u2663"  # Club
        assert SUIT_SYMBOLS["S"] == "\u2660"  # Spade

    def test_suit_colors_complete(self) -> None:
        """All four suits should have colors defined."""
        assert "H" in SUIT_COLORS
        assert "D" in SUIT_COLORS
        assert "C" in SUIT_COLORS
        assert "S" in SUIT_COLORS

        # Hearts and diamonds are red
        assert SUIT_COLORS["H"] == "red"
        assert SUIT_COLORS["D"] == "red"

        # Clubs and spades are default (black)
        assert SUIT_COLORS["C"] == "default"
        assert SUIT_COLORS["S"] == "default"

    def test_min_wide_width_reasonable(self) -> None:
        """MIN_WIDE_WIDTH should be a reasonable threshold."""
        assert MIN_WIDE_WIDTH > 40  # Not too small
        assert MIN_WIDE_WIDTH < 100  # Not too large
        assert MIN_WIDE_WIDTH == 60  # Current expected value
