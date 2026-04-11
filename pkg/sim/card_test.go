package sim

import (
	"math/rand/v2"
	"testing"
)

func TestStandardDeck(t *testing.T) {
	deck := StandardDeck()
	if len(deck) != 52 {
		t.Fatalf("expected 52 cards, got %d", len(deck))
	}

	// Check no duplicates
	seen := make(map[Card]bool)
	for _, c := range deck {
		if seen[c] {
			t.Fatalf("duplicate card: %s", c)
		}
		seen[c] = true
	}
}

func TestShuffleDeck(t *testing.T) {
	deck := StandardDeck()
	original := make([]Card, len(deck))
	copy(original, deck)

	rng := rand.New(rand.NewPCG(42, 0))
	ShuffleDeck(deck, rng)

	// Shuffled deck should have same cards but different order
	if len(deck) != 52 {
		t.Fatalf("shuffle changed deck size")
	}

	same := true
	for i := range deck {
		if deck[i] != original[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("shuffle did not change card order")
	}
}

func TestDrawN(t *testing.T) {
	deck := StandardDeck()
	drawn, remaining := DrawN(deck, 5)
	if len(drawn) != 5 {
		t.Fatalf("expected 5 drawn, got %d", len(drawn))
	}
	if len(remaining) != 47 {
		t.Fatalf("expected 47 remaining, got %d", len(remaining))
	}
}

func TestDrawNMoreThanAvailable(t *testing.T) {
	deck := []Card{{Suit: Hearts, Rank: Ace}}
	drawn, remaining := DrawN(deck, 5)
	if len(drawn) != 1 {
		t.Fatalf("expected 1 drawn, got %d", len(drawn))
	}
	if len(remaining) != 0 {
		t.Fatalf("expected 0 remaining, got %d", len(remaining))
	}
}

func TestCardString(t *testing.T) {
	c := Card{Suit: Spades, Rank: Ace}
	if c.String() != "AS" {
		t.Fatalf("expected AS, got %s", c.String())
	}
}
