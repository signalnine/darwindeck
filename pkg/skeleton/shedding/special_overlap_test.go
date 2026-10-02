package shedding

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// overlapState builds a 3-player state where P0 holds a 9 of hearts that
// matches the discard top, P1 (the victim) holds one card, and the deck is deep
// enough to cover any draw penalty.
func overlapState() *sim.GameState {
	st := sim.NewGameState(3)
	st.Hands[0] = []sim.Card{{Suit: sim.Hearts, Rank: 9}, {Suit: sim.Spades, Rank: 2}}
	st.Hands[1] = []sim.Card{{Suit: sim.Spades, Rank: 3}}
	st.Hands[2] = []sim.Card{{Suit: sim.Spades, Rank: 4}}
	top := sim.Card{Suit: sim.Hearts, Rank: 5}
	st.Discard = []sim.Card{top}
	st.TopCard = &top
	for r := sim.Two; r <= sim.Ace; r++ {
		st.Deck = append(st.Deck, sim.Card{Suit: sim.Clubs, Rank: r})
	}
	st.Direction = 1
	st.RNG = rand.New(rand.NewPCG(1, 2))
	return st
}

// TestOverlappingDrawEffectsTakeLarger: when a draw_two rule AND a draw_four
// rule both match the played card, the victim draws 4 whatever order the rules
// sit in the genome. The runner used to apply whichever rule came FIRST in the
// slice (2 for [draw_two, draw_four], 4 for the reverse) while the rulebook
// printed both lines -- so a mutation or crossover that merely permuted the
// special-card list changed the game.
func TestOverlappingDrawEffectsTakeLarger(t *testing.T) {
	orders := [][]genome.SpecialCardType{
		{genome.SpecialDrawTwo, genome.SpecialDrawFour},
		{genome.SpecialDrawFour, genome.SpecialDrawTwo},
	}
	for _, order := range orders {
		g := &genome.Genome{
			Skeleton: genome.Shedding, Players: 3, HandSize: 7,
			Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
		}
		for _, ty := range order {
			g.SpecialCards = append(g.SpecialCards, genome.SpecialCard{Type: ty, ByRank: 9})
		}
		st := overlapState()
		(&Runner{}).ApplyMove(st, sim.Move{Type: sim.MovePlay, Cards: []sim.Card{{Suit: sim.Hearts, Rank: 9}}}, g)

		if got := len(st.Hands[1]); got != 1+4 {
			t.Errorf("rules %v: victim holds %d cards, want 5 (1 + the larger draw of 4)", order, got)
		}
		// The victim still loses exactly one turn.
		if st.Active != 2 {
			t.Errorf("rules %v: next active = P%d, want P2 (victim skipped once)", order, st.Active)
		}
	}
}

// TestSingleDrawEffectsUnchanged: the take-larger rule must not disturb the
// ordinary one-rule cases.
func TestSingleDrawEffectsUnchanged(t *testing.T) {
	for _, tc := range []struct {
		ty   genome.SpecialCardType
		want int
	}{{genome.SpecialDrawTwo, 2}, {genome.SpecialDrawFour, 4}} {
		g := &genome.Genome{
			Skeleton: genome.Shedding, Players: 3, HandSize: 7,
			Shedding:     &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
			SpecialCards: []genome.SpecialCard{{Type: tc.ty, ByRank: 9}, {Type: tc.ty, ByRank: 9, BySuit: 3}},
		}
		st := overlapState()
		(&Runner{}).ApplyMove(st, sim.Move{Type: sim.MovePlay, Cards: []sim.Card{{Suit: sim.Hearts, Rank: 9}}}, g)
		if got := len(st.Hands[1]); got != 1+tc.want {
			t.Errorf("%s (duplicated rule): victim holds %d cards, want %d", tc.ty, got, 1+tc.want)
		}
	}
}
