package vying

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func tieGenome(players int) *genome.Genome {
	return &genome.Genome{
		Skeleton: genome.Vying, Players: players, HandSize: 5,
		Vying: &genome.VyingParams{StartingChips: 1000, MinBet: 10, MaxRaises: 3, RoundsPerGame: 4},
	}
}

// showdownState is a closed betting round ready for Upkeep to resolve: nobody
// is owed an action, the pot is set, and every non-folded player holds the
// given hand.
func showdownState(players, round, pot int, hands [][]sim.Card, folded []bool) *sim.GameState {
	st := sim.NewGameState(players)
	st.Committed = make([]int, players)
	st.Folded = append([]bool(nil), folded...)
	for i := range hands {
		st.Hands[i] = hands[i]
	}
	for i := range st.Scores {
		st.Scores[i] = 100
	}
	st.Pot = pot
	st.Round = round
	st.MaxRound = round + 1 // resolve as the final deal: no redeal needed
	st.ToAct = 0
	return st
}

var (
	kingHighA = []sim.Card{{Suit: sim.Hearts, Rank: 2}, {Suit: sim.Clubs, Rank: 5}, {Suit: sim.Diamonds, Rank: 7}, {Suit: sim.Spades, Rank: 9}, {Suit: sim.Hearts, Rank: sim.King}}
	kingHighB = []sim.Card{{Suit: sim.Clubs, Rank: 2}, {Suit: sim.Hearts, Rank: 5}, {Suit: sim.Spades, Rank: 7}, {Suit: sim.Diamonds, Rank: 9}, {Suit: sim.Clubs, Rank: sim.King}}
	kingHighC = []sim.Card{{Suit: sim.Diamonds, Rank: 2}, {Suit: sim.Spades, Rank: 5}, {Suit: sim.Hearts, Rank: 7}, {Suit: sim.Clubs, Rank: 9}, {Suit: sim.Spades, Rank: sim.King}}
	tripAces  = []sim.Card{{Suit: sim.Hearts, Rank: sim.Ace}, {Suit: sim.Clubs, Rank: sim.Ace}, {Suit: sim.Diamonds, Rank: sim.Ace}, {Suit: sim.Clubs, Rank: 9}, {Suit: sim.Clubs, Rank: 3}}
)

// TestSplitPotOddChipGoesToFirstToAct: a split pot's odd chip used to go to the
// lowest tied seat. It now goes to the tied winner who acts first in that deal
// (the seat after the rotating blind) -- the table rule, and no fixed seat.
func TestSplitPotOddChipGoesToFirstToAct(t *testing.T) {
	r := &Runner{}
	g := tieGenome(3)
	for round, wantOdd := range map[int]int{0: 1, 1: 2, 2: 0} {
		// Blind = round % 3; first to act = blind + 1. All three tie; pot 31.
		st := showdownState(3, round, 31, [][]sim.Card{kingHighA, kingHighB, kingHighC}, []bool{false, false, false})
		r.Upkeep(st, g)
		for p := 0; p < 3; p++ {
			want := 110
			if p == wantOdd {
				want = 111
			}
			if st.Scores[p] != want {
				t.Errorf("deal %d (blind P%d): P%d has %d chips, want %d (odd chip to P%d, first to act); scores %v",
					round, round%3, p, st.Scores[p], want, wantOdd, st.Scores)
			}
		}
	}

	// Two of three tie (the third folded a better hand): odd chip to the tied
	// winner nearest after the blind, skipping the folded seat.
	st := showdownState(3, 0, 31, [][]sim.Card{kingHighA, tripAces, kingHighB}, []bool{false, true, false})
	r.Upkeep(st, g)
	if st.Scores[0] != 115 || st.Scores[1] != 100 || st.Scores[2] != 116 {
		t.Errorf("blind P0, P1 folded: scores %v, want [115 100 116] (odd chip to P2, the first tied winner after the blind)", st.Scores)
	}
}

// TestChipTieGoesToLastPotWinner: equal final stacks used to go to the lowest
// seat. The tie now goes to the tied player who won the most recent pot,
// otherwise to the tied player next in seat order after that pot's winner.
func TestChipTieGoesToLastPotWinner(t *testing.T) {
	r := &Runner{}
	g := tieGenome(3)
	cases := []struct {
		name    string
		scores  []int
		lastPot int
		want    int
	}{
		{"untied: biggest stack wins", []int{90, 130, 80}, 0, 1},
		{"tied, last pot winner among the tied", []int{110, 110, 80}, 1, 1},
		{"tied, last pot winner is seat 0", []int{110, 110, 80}, 0, 0},
		{"tied, last pot winner not tied: next tied seat after them", []int{80, 110, 110}, 0, 1},
		{"tied, wraps around", []int{110, 80, 110}, 2, 2},
		{"three-way tie", []int{100, 100, 100}, 2, 2},
	}
	for _, tc := range cases {
		st := sim.NewGameState(3)
		copy(st.Scores, tc.scores)
		st.Round, st.MaxRound = 4, 4
		st.TrickLeader = tc.lastPot
		if got := r.CheckEnd(st, g); got != tc.want {
			t.Errorf("%s: stacks %v, last pot P%d: winner = P%d, want P%d", tc.name, tc.scores, tc.lastPot, got, tc.want)
		}
	}

	// resolveShowdown records the pot winner the tie rule reads.
	st := showdownState(3, 0, 30, [][]sim.Card{kingHighA, tripAces, kingHighB}, []bool{false, false, false})
	r.Upkeep(st, g)
	if st.TrickLeader != 1 {
		t.Errorf("showdown must record the pot winner (P1, three aces): TrickLeader = %d", st.TrickLeader)
	}
	// Uncontested pot: the last player standing is the pot winner.
	st = showdownState(3, 0, 30, [][]sim.Card{kingHighA, tripAces, kingHighB}, []bool{true, true, false})
	r.Upkeep(st, g)
	if st.TrickLeader != 2 {
		t.Errorf("uncontested pot must record the survivor (P2): TrickLeader = %d", st.TrickLeader)
	}
}
