# tests/unit/test_self_describing_types.py
"""Tests for self-describing genome types."""

import pytest
from darwindeck.genome.schema import ScoringTrigger


class TestScoringTrigger:
    def test_scoring_trigger_enum_values(self):
        """ScoringTrigger enum has expected values."""
        assert ScoringTrigger.TRICK_WIN.value == "trick_win"
        assert ScoringTrigger.CAPTURE.value == "capture"
        assert ScoringTrigger.PLAY.value == "play"
        assert ScoringTrigger.HAND_END.value == "hand_end"
        assert ScoringTrigger.SET_COMPLETE.value == "set_complete"
