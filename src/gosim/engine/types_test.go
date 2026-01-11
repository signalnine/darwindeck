package engine

import (
	"testing"
)

func TestStatePool(t *testing.T) {
	// Acquire and release
	s1 := GetState()
	if len(s1.Players) != 4 {
		t.Errorf("Expected 4 players, got %d", len(s1.Players))
	}

	PutState(s1)

	// Should get same instance back
	s2 := GetState()
	if &s1.Players[0] != &s2.Players[0] {
		t.Error("Pool did not reuse memory")
	}

	PutState(s2)
}

func TestGameStateClone(t *testing.T) {
	s1 := GetState()
	s1.Players[0].Hand = append(s1.Players[0].Hand, Card{Rank: 0, Suit: 0})
	s1.Deck = append(s1.Deck, Card{Rank: 1, Suit: 1})

	s2 := s1.Clone()

	// Modify original
	s1.Players[0].Hand[0].Rank = 12
	s1.Deck[0].Suit = 3

	// Clone should be unchanged
	if s2.Players[0].Hand[0].Rank != 0 {
		t.Error("Clone was not deep copied")
	}
	if s2.Deck[0].Suit != 1 {
		t.Error("Clone deck was not deep copied")
	}

	PutState(s1)
	PutState(s2)
}

func TestDrawAndPlay(t *testing.T) {
	s := GetState()
	s.Deck = append(s.Deck, Card{Rank: 5, Suit: 2})

	// Draw card
	if !s.DrawCard(0, LocationDeck) {
		t.Error("Failed to draw card")
	}

	if len(s.Players[0].Hand) != 1 {
		t.Errorf("Expected 1 card in hand, got %d", len(s.Players[0].Hand))
	}

	// Play card
	if !s.PlayCard(0, 0, LocationDiscard) {
		t.Error("Failed to play card")
	}

	if len(s.Players[0].Hand) != 0 {
		t.Error("Hand should be empty after playing")
	}

	if len(s.Discard) != 1 {
		t.Errorf("Expected 1 card in discard, got %d", len(s.Discard))
	}

	PutState(s)
}
