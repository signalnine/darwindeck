package sim

import (
	"fmt"
	"math/rand/v2"
)

// Suit represents a card suit.
type Suit uint8

const (
	Clubs    Suit = iota
	Diamonds
	Hearts
	Spades
)

var suitNames = [4]string{"C", "D", "H", "S"}

func (s Suit) String() string { return suitNames[s] }

// Rank represents a card rank (2-14, where 11=J, 12=Q, 13=K, 14=A).
type Rank uint8

const (
	Two   Rank = 2
	Three Rank = 3
	Four  Rank = 4
	Five  Rank = 5
	Six   Rank = 6
	Seven Rank = 7
	Eight Rank = 8
	Nine  Rank = 9
	Ten   Rank = 10
	Jack  Rank = 11
	Queen Rank = 12
	King  Rank = 13
	Ace   Rank = 14
)

var rankNames = map[Rank]string{
	Two: "2", Three: "3", Four: "4", Five: "5", Six: "6",
	Seven: "7", Eight: "8", Nine: "9", Ten: "10",
	Jack: "J", Queen: "Q", King: "K", Ace: "A",
}

func (r Rank) String() string { return rankNames[r] }

// Card is a single playing card.
type Card struct {
	Suit Suit
	Rank Rank
}

func (c Card) String() string {
	return fmt.Sprintf("%s%s", c.Rank, c.Suit)
}

// StandardDeck returns a new 52-card deck in order.
func StandardDeck() []Card {
	deck := make([]Card, 0, 52)
	for suit := Clubs; suit <= Spades; suit++ {
		for rank := Two; rank <= Ace; rank++ {
			deck = append(deck, Card{Suit: suit, Rank: rank})
		}
	}
	return deck
}

// ShuffleDeck shuffles a deck in place using the provided RNG.
func ShuffleDeck(deck []Card, rng *rand.Rand) {
	for i := len(deck) - 1; i > 0; i-- {
		j := rng.IntN(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
}

// DrawN removes and returns the top n cards from the deck.
// Returns the drawn cards and the remaining deck.
func DrawN(deck []Card, n int) (drawn []Card, remaining []Card) {
	if n > len(deck) {
		n = len(deck)
	}
	return deck[:n:n], deck[n:]
}
