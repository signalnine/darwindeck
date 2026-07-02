package vying

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

func c(r sim.Rank, s sim.Suit) sim.Card { return sim.Card{Rank: r, Suit: s} }

// TestHandRankOrdering: every category beats every weaker one, using canonical
// example hands in ascending strength.
func TestHandRankOrdering(t *testing.T) {
	hands := []struct {
		name  string
		cards []sim.Card
	}{
		{"high card", []sim.Card{c(sim.Two, sim.Clubs), c(sim.Five, sim.Hearts), c(sim.Eight, sim.Spades), c(sim.Jack, sim.Diamonds), c(sim.King, sim.Clubs)}},
		{"pair", []sim.Card{c(sim.Two, sim.Clubs), c(sim.Two, sim.Hearts), c(sim.Eight, sim.Spades), c(sim.Jack, sim.Diamonds), c(sim.King, sim.Clubs)}},
		{"two pair", []sim.Card{c(sim.Two, sim.Clubs), c(sim.Two, sim.Hearts), c(sim.Eight, sim.Spades), c(sim.Eight, sim.Diamonds), c(sim.King, sim.Clubs)}},
		{"trips", []sim.Card{c(sim.Two, sim.Clubs), c(sim.Two, sim.Hearts), c(sim.Two, sim.Spades), c(sim.Jack, sim.Diamonds), c(sim.King, sim.Clubs)}},
		{"straight", []sim.Card{c(sim.Five, sim.Clubs), c(sim.Six, sim.Hearts), c(sim.Seven, sim.Spades), c(sim.Eight, sim.Diamonds), c(sim.Nine, sim.Clubs)}},
		{"flush", []sim.Card{c(sim.Two, sim.Clubs), c(sim.Five, sim.Clubs), c(sim.Eight, sim.Clubs), c(sim.Jack, sim.Clubs), c(sim.King, sim.Clubs)}},
		{"full house", []sim.Card{c(sim.Two, sim.Clubs), c(sim.Two, sim.Hearts), c(sim.Two, sim.Spades), c(sim.King, sim.Diamonds), c(sim.King, sim.Clubs)}},
		{"quads", []sim.Card{c(sim.Two, sim.Clubs), c(sim.Two, sim.Hearts), c(sim.Two, sim.Spades), c(sim.Two, sim.Diamonds), c(sim.King, sim.Clubs)}},
		{"straight flush", []sim.Card{c(sim.Five, sim.Clubs), c(sim.Six, sim.Clubs), c(sim.Seven, sim.Clubs), c(sim.Eight, sim.Clubs), c(sim.Nine, sim.Clubs)}},
	}
	prev := int64(-1)
	for _, h := range hands {
		s := HandStrength(h.cards)
		if s <= prev {
			t.Fatalf("%s strength %d not greater than previous %d (ordering broken)", h.name, s, prev)
		}
		prev = s
	}
}

// TestHandRankTiebreaks: within a category, higher kickers win; the wheel is the
// lowest straight; a split is an exact tie.
func TestHandRankTiebreaks(t *testing.T) {
	// Higher pair beats lower pair.
	aces := HandStrength([]sim.Card{c(sim.Ace, sim.Clubs), c(sim.Ace, sim.Hearts), c(sim.Five, sim.Spades), c(sim.Eight, sim.Diamonds), c(sim.King, sim.Clubs)})
	kings := HandStrength([]sim.Card{c(sim.King, sim.Clubs), c(sim.King, sim.Hearts), c(sim.Five, sim.Spades), c(sim.Eight, sim.Diamonds), c(sim.Queen, sim.Clubs)})
	if aces <= kings {
		t.Fatalf("pair of aces (%d) must beat pair of kings (%d)", aces, kings)
	}

	// The wheel (A-2-3-4-5) is the lowest straight, below 2-3-4-5-6.
	wheel := HandStrength([]sim.Card{c(sim.Ace, sim.Clubs), c(sim.Two, sim.Hearts), c(sim.Three, sim.Spades), c(sim.Four, sim.Diamonds), c(sim.Five, sim.Clubs)})
	sixHigh := HandStrength([]sim.Card{c(sim.Two, sim.Clubs), c(sim.Three, sim.Hearts), c(sim.Four, sim.Spades), c(sim.Five, sim.Diamonds), c(sim.Six, sim.Clubs)})
	if wheel >= sixHigh {
		t.Fatalf("wheel straight (%d) must be the lowest, below 6-high (%d)", wheel, sixHigh)
	}
	if CategoryOf(wheel) != Straight {
		t.Fatalf("wheel must rank as a straight, got category %d", CategoryOf(wheel))
	}

	// Identical hands (different suits) tie exactly.
	a := HandStrength([]sim.Card{c(sim.Ace, sim.Clubs), c(sim.Ace, sim.Hearts), c(sim.King, sim.Spades), c(sim.King, sim.Diamonds), c(sim.Two, sim.Clubs)})
	b := HandStrength([]sim.Card{c(sim.Ace, sim.Spades), c(sim.Ace, sim.Diamonds), c(sim.King, sim.Hearts), c(sim.King, sim.Clubs), c(sim.Two, sim.Hearts)})
	if a != b {
		t.Fatalf("identical aces-over-kings must tie: %d vs %d", a, b)
	}
}

// TestHandRankBestOfSeven: HandStrength picks the best 5 from a larger hand.
func TestHandRankBestOfSeven(t *testing.T) {
	// Seven cards containing a flush; best-5 must be the flush.
	s := HandStrength([]sim.Card{
		c(sim.Two, sim.Clubs), c(sim.Five, sim.Clubs), c(sim.Eight, sim.Clubs), c(sim.Jack, sim.Clubs), c(sim.King, sim.Clubs),
		c(sim.Ace, sim.Hearts), c(sim.Ace, sim.Spades),
	})
	if CategoryOf(s) != Flush {
		t.Fatalf("best-of-seven with a flush must rank Flush, got category %d", CategoryOf(s))
	}
}

// TestHandStrengthEmptyHand: a mucked (emptied) hand must rank below every real
// hand instead of panicking in eval5.
func TestHandStrengthEmptyHand(t *testing.T) {
	if got := HandStrength(nil); got != 0 {
		t.Fatalf("HandStrength(nil) = %d, want 0", got)
	}
	real := []sim.Card{{Rank: 2, Suit: 0}, {Rank: 5, Suit: 1}, {Rank: 7, Suit: 2}, {Rank: 9, Suit: 3}, {Rank: 12, Suit: 0}}
	if HandStrength(real) <= HandStrength(nil) {
		t.Fatalf("a real hand must outrank a mucked hand")
	}
}
