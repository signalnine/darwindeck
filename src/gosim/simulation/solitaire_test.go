package simulation

import (
	"testing"

	"github.com/signalnine/darwindeck/gosim/engine"
)

func TestMovesDisrupted_DifferentLength(t *testing.T) {
	before := []engine.LegalMove{{PhaseIndex: 0, TargetLoc: 1}}
	after := []engine.LegalMove{{PhaseIndex: 0, TargetLoc: 1}, {PhaseIndex: 0, TargetLoc: 2}}

	if !movesDisrupted(before, after) {
		t.Error("Expected disruption when move counts differ")
	}
}

func TestMovesDisrupted_SameLength_DifferentMoves(t *testing.T) {
	before := []engine.LegalMove{{PhaseIndex: 0, TargetLoc: 1}}
	after := []engine.LegalMove{{PhaseIndex: 0, TargetLoc: 2}}

	if !movesDisrupted(before, after) {
		t.Error("Expected disruption when moves differ")
	}
}

func TestMovesDisrupted_Identical(t *testing.T) {
	before := []engine.LegalMove{{PhaseIndex: 0, TargetLoc: 1}, {PhaseIndex: 1, TargetLoc: 2}}
	after := []engine.LegalMove{{PhaseIndex: 0, TargetLoc: 1}, {PhaseIndex: 1, TargetLoc: 2}}

	if movesDisrupted(before, after) {
		t.Error("Expected no disruption when moves are identical")
	}
}

func TestMovesDisrupted_BothEmpty(t *testing.T) {
	before := []engine.LegalMove{}
	after := []engine.LegalMove{}

	if movesDisrupted(before, after) {
		t.Error("Expected no disruption when both empty")
	}
}

func TestMovesDisrupted_OneEmpty(t *testing.T) {
	before := []engine.LegalMove{{PhaseIndex: 0, TargetLoc: 1}}
	after := []engine.LegalMove{}

	if !movesDisrupted(before, after) {
		t.Error("Expected disruption when one is empty")
	}
}
