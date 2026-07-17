package mechanic

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// On a SHEDDING host, state.Tableau is the shed-cards tally kept for
// MechTrickScoring (SheddingTrickScored) -- not a captured pile. The scoring
// hooks must read the residual hand only: counting the tableau penalized
// players for penalty cards they successfully got RID of, and a shed card
// recycled by refillDeckFromDiscard and redrawn counted twice.

func TestAvoidanceIgnoresSheddingShedTally(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Suit: uint8(sim.Hearts) + 1, Points: 1},
			},
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.TrickTaking, Mechanic: genome.MechTrickScoring},
			{Source: genome.Rummy, Mechanic: genome.MechAvoidance},
		},
	}
	state := &sim.GameState{
		Hands: [][]sim.Card{
			{{Suit: sim.Hearts, Rank: sim.Two}}, // 1 penalty heart still held
			{{Suit: sim.Spades, Rank: sim.Ace}},
		},
		// P0 SHED two hearts earlier; they are the trick-scoring tally, not captures.
		Tableau: [][]sim.Card{
			{{Suit: sim.Hearts, Rank: sim.Five}, {Suit: sim.Hearts, Rank: sim.King}},
			{},
		},
		Scores:     []int{0, 0},
		NumPlayers: 2,
	}

	applyAvoidance(state, g, sim.Event{})

	if state.Scores[0] != -1 {
		t.Fatalf("shedding avoidance must penalize the held heart only (-1), got %d (shed tally counted)", state.Scores[0])
	}
	if state.Scores[1] != 0 {
		t.Fatalf("player 1 holds no hearts, want 0, got %d", state.Scores[1])
	}
}

func TestMeldBonusIgnoresSheddingShedTally(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.TrickTaking, Mechanic: genome.MechTrickScoring},
			{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
		},
	}
	state := &sim.GameState{
		// P0's residual hand holds no meld; the shed tally holds a triple that
		// must NOT be counted as one.
		Hands: [][]sim.Card{
			{{Suit: sim.Hearts, Rank: sim.Two}},
			{{Suit: sim.Spades, Rank: sim.Ace}},
		},
		Tableau: [][]sim.Card{
			{
				{Suit: sim.Hearts, Rank: sim.Nine},
				{Suit: sim.Spades, Rank: sim.Nine},
				{Suit: sim.Clubs, Rank: sim.Nine},
			},
			{},
		},
		Scores:     []int{0, 0},
		NumPlayers: 2,
	}

	applyMeldBonus(state, g, sim.Event{})

	if state.Scores[0] != 0 {
		t.Fatalf("shed-tally triple must not score as a meld on shedding, got %d", state.Scores[0])
	}
}

// The trick-taking capture semantics are unchanged: tableau IS the captured
// pile there (pinned separately by TestAvoidanceIncludesTableauCaptures).
