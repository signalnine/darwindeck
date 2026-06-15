package shedding

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func followGenome() *genome.Genome {
	return &genome.Genome{
		ID: "follow-fixture", Skeleton: genome.Shedding, Players: 2, HandSize: 6,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
		Borrowed: []genome.BorrowedMechanic{{Source: genome.TrickTaking, Mechanic: genome.MechFollowSuit}},
	}
}

// TestFollowConstraintRestricts: holding the discard suit, a legal rank-match
// off-suit play is filtered out -- you must follow the suit.
func TestFollowConstraintRestricts(t *testing.T) {
	g := followGenome()
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Active = 0
	fiveD := sim.Card{Suit: 2, Rank: 5} // matches top 5C by RANK, but is a diamond
	nineC := sim.Card{Suit: 1, Rank: 9} // matches top 5C by SUIT (clubs == suit 1)
	state.Hands[0] = []sim.Card{fiveD, nineC}
	state.Hands[1] = []sim.Card{{Suit: 3, Rank: 4}}
	state.TopCard = &sim.Card{Suit: 1, Rank: 5} // 5 of clubs
	state.Discard = []sim.Card{*state.TopCard}

	moves := runner.GenerateMoves(state, g)
	for _, m := range moves {
		if m.Type == sim.MovePlay && m.Cards[0] == fiveD {
			t.Fatalf("follow-suit must forbid the off-suit rank-match 5D while a club is held; moves=%+v", moves)
		}
	}
	playedNineC := false
	for _, m := range moves {
		if m.Type == sim.MovePlay && m.Cards[0] == nineC {
			playedNineC = true
		}
	}
	if !playedNineC {
		t.Fatalf("the in-suit card 9C must be a legal follow, moves=%+v", moves)
	}
}

// TestFollowConstraintVoidReopens: void in the discard suit, normal plays return.
func TestFollowConstraintVoidReopens(t *testing.T) {
	g := followGenome()
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Active = 0
	fiveD := sim.Card{Suit: 2, Rank: 5} // diamond, matches top 5C by rank
	state.Hands[0] = []sim.Card{fiveD, {Suit: 3, Rank: 9}}
	state.Hands[1] = []sim.Card{{Suit: 3, Rank: 4}}
	state.TopCard = &sim.Card{Suit: 1, Rank: 5} // 5 of clubs; player holds no clubs
	state.Discard = []sim.Card{*state.TopCard}
	state.Deck = []sim.Card{{Suit: 4, Rank: 7}}

	moves := runner.GenerateMoves(state, g)
	found := false
	for _, m := range moves {
		if m.Type == sim.MovePlay && m.Cards[0] == fiveD {
			found = true
		}
	}
	if !found {
		t.Fatalf("void in the suit: the rank-match 5D must be legal again, moves=%+v", moves)
	}
}

// TestFollowConstraintMovesNeverEmpty + completion: the filter restricts but a
// forced play still sheds a card and a void hand keeps draw, so games run to
// completion and never deadlock.
func TestFollowConstraintNeverEmptyAndCompletes(t *testing.T) {
	g := followGenome()
	runner := &Runner{}
	completed := 0
	for seed := uint64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		state := runner.Setup(g, rng)
		maxTurns := g.MaxTurns()
		for turn := 0; turn < maxTurns; turn++ {
			runner.Upkeep(state, g)
			if runner.CheckEnd(state, g) >= 0 {
				completed++
				break
			}
			moves := runner.GenerateMoves(state, g)
			if len(moves) == 0 {
				t.Fatalf("seed %d turn %d: follow-constraint produced zero moves", seed, turn)
			}
			runner.ApplyMove(state, (&sim.RandomAI{}).SelectMove(moves, state, rng), g)
		}
	}
	t.Logf("FollowConstraint: %d/200 completed", completed)
	if completed < 190 {
		t.Fatalf("follow-constraint completes too rarely: %d/200", completed)
	}
}

func TestFollowConstraintDeterminism(t *testing.T) {
	g := followGenome()
	for seed := uint64(0); seed < 30; seed++ {
		r1, r2 := runGame(g, seed), runGame(g, seed)
		if r1.Winner != r2.Winner || r1.Turns != r2.Turns {
			t.Fatalf("seed %d non-deterministic: (%d,%d) vs (%d,%d)", seed, r1.Winner, r1.Turns, r2.Winner, r2.Turns)
		}
	}
}
