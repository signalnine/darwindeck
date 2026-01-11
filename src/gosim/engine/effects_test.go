package engine

import (
	"testing"
)

func TestApplySkipNext(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 3

	effect := &SpecialEffect{EffectType: EFFECT_SKIP_NEXT, Value: 1}
	ApplyEffect(state, effect, nil)

	if state.SkipCount != 1 {
		t.Errorf("SkipCount should be 1, got %d", state.SkipCount)
	}
}

func TestApplySkipNextCapped(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 3
	state.SkipCount = 2

	effect := &SpecialEffect{EffectType: EFFECT_SKIP_NEXT, Value: 5}
	ApplyEffect(state, effect, nil)

	// Should cap at NumPlayers-1 = 2
	if state.SkipCount != 2 {
		t.Errorf("SkipCount should cap at 2, got %d", state.SkipCount)
	}
}

func TestApplyReverse(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.PlayDirection = 1

	effect := &SpecialEffect{EffectType: EFFECT_REVERSE}
	ApplyEffect(state, effect, nil)

	if state.PlayDirection != -1 {
		t.Errorf("PlayDirection should be -1, got %d", state.PlayDirection)
	}

	// Reverse again
	ApplyEffect(state, effect, nil)
	if state.PlayDirection != 1 {
		t.Errorf("PlayDirection should be 1, got %d", state.PlayDirection)
	}
}

func TestApplyDrawCards(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 2
	state.CurrentPlayer = 0
	state.PlayDirection = 1
	state.Deck = []Card{{Rank: 5, Suit: 0}, {Rank: 7, Suit: 1}, {Rank: 9, Suit: 2}}
	state.Players[1].Hand = []Card{}

	effect := &SpecialEffect{
		EffectType: EFFECT_DRAW_CARDS,
		Target:     TARGET_NEXT_PLAYER,
		Value:      2,
	}
	ApplyEffect(state, effect, nil)

	if len(state.Players[1].Hand) != 2 {
		t.Errorf("Player 1 should have 2 cards, got %d", len(state.Players[1].Hand))
	}
	if len(state.Deck) != 1 {
		t.Errorf("Deck should have 1 card, got %d", len(state.Deck))
	}
}

func TestApplyExtraTurn(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 3

	effect := &SpecialEffect{EffectType: EFFECT_EXTRA_TURN}
	ApplyEffect(state, effect, nil)

	// Should skip all other players (NumPlayers - 1)
	if state.SkipCount != 2 {
		t.Errorf("SkipCount should be 2, got %d", state.SkipCount)
	}
}

func TestApplyForceDiscard(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 2
	state.CurrentPlayer = 0
	state.PlayDirection = 1
	state.Players[1].Hand = []Card{{Rank: 2, Suit: 0}, {Rank: 5, Suit: 1}, {Rank: 8, Suit: 2}}
	state.Discard = []Card{}

	effect := &SpecialEffect{
		EffectType: EFFECT_FORCE_DISCARD,
		Target:     TARGET_NEXT_PLAYER,
		Value:      2,
	}
	ApplyEffect(state, effect, nil)

	if len(state.Players[1].Hand) != 1 {
		t.Errorf("Player 1 should have 1 card, got %d", len(state.Players[1].Hand))
	}
	if len(state.Discard) != 2 {
		t.Errorf("Discard should have 2 cards, got %d", len(state.Discard))
	}
}

func TestResolveTargetNextPlayer(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 4
	state.CurrentPlayer = 1
	state.PlayDirection = 1

	target := resolveTarget(state, TARGET_NEXT_PLAYER)
	if target != 2 {
		t.Errorf("Next player from 1 (direction 1) should be 2, got %d", target)
	}

	state.PlayDirection = -1
	target = resolveTarget(state, TARGET_NEXT_PLAYER)
	if target != 0 {
		t.Errorf("Next player from 1 (direction -1) should be 0, got %d", target)
	}
}

func TestResolveTargetAllOpponents(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 3
	state.CurrentPlayer = 1

	target := resolveTarget(state, TARGET_ALL_OPPONENTS)
	if target != -1 {
		t.Errorf("ALL_OPPONENTS should return -1, got %d", target)
	}
}

func TestAdvanceTurnWithSkip(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 4
	state.CurrentPlayer = 0
	state.PlayDirection = 1
	state.SkipCount = 1

	AdvanceTurn(state)

	if state.CurrentPlayer != 2 {
		t.Errorf("Should skip to player 2, got %d", state.CurrentPlayer)
	}
	if state.SkipCount != 0 {
		t.Errorf("SkipCount should reset to 0, got %d", state.SkipCount)
	}
}

func TestAdvanceTurnReversed(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 4
	state.CurrentPlayer = 1
	state.PlayDirection = -1
	state.SkipCount = 0

	AdvanceTurn(state)

	if state.CurrentPlayer != 0 {
		t.Errorf("Reversed from 1 should go to 0, got %d", state.CurrentPlayer)
	}
}

func TestAdvanceTurnWraparound(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 3
	state.CurrentPlayer = 2
	state.PlayDirection = 1
	state.SkipCount = 0

	AdvanceTurn(state)

	if state.CurrentPlayer != 0 {
		t.Errorf("Should wrap to 0, got %d", state.CurrentPlayer)
	}
}
