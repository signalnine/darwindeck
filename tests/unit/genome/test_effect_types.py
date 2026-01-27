"""Tests for EffectType and TargetSelector enum completeness and format."""

import pytest
from darwindeck.genome.schema import EffectType, TargetSelector


class TestEffectTypeCompleteness:
    """Test that EffectType enum includes all Go-supported effect types."""

    def test_all_go_effect_types_exist(self) -> None:
        """All effect types supported by Go simulator must exist in Python enum."""
        # These are the effect types defined in Go's genome/effects.go
        required_effect_types = {
            "skip_next",
            "reverse_direction",  # Note: Go may use "reverse", Python uses "reverse"
            "draw_cards",
            "extra_turn",
            "force_discard",
            "wild_card",
            "block_next",
            "swap_hands",
            "steal_card",
            "peek_hand",
        }

        # Get all string values from the enum
        actual_values = {e.value for e in EffectType}

        # Handle the reverse vs reverse_direction mismatch
        # The Python enum uses "reverse" as the value, not "reverse_direction"
        if "reverse" in actual_values:
            actual_values.add("reverse_direction")

        missing = required_effect_types - actual_values
        assert not missing, f"Missing effect types: {missing}"

    def test_effect_type_has_string_values(self) -> None:
        """All EffectType enum members must have string values."""
        for effect_type in EffectType:
            assert isinstance(
                effect_type.value, str
            ), f"{effect_type.name} has non-string value: {type(effect_type.value)}"

    def test_effect_type_count(self) -> None:
        """EffectType should have exactly 10 members."""
        assert len(EffectType) == 10, f"Expected 10 effect types, got {len(EffectType)}"

    def test_effect_type_names_are_uppercase(self) -> None:
        """All EffectType enum names should be uppercase (Python convention)."""
        for effect_type in EffectType:
            assert (
                effect_type.name.isupper()
            ), f"Effect type name {effect_type.name} is not uppercase"


class TestTargetSelectorCompleteness:
    """Test that TargetSelector enum includes SELF for Go compatibility."""

    def test_target_selector_has_self(self) -> None:
        """TargetSelector must include SELF for self-targeting effects."""
        assert hasattr(TargetSelector, "SELF"), "Missing SELF target"
        assert TargetSelector.SELF.value == "self"

    def test_target_selector_has_string_values(self) -> None:
        """All TargetSelector enum members must have string values."""
        for target in TargetSelector:
            assert isinstance(
                target.value, str
            ), f"{target.name} has non-string value: {type(target.value)}"
