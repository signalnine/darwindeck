package vying

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// TestVyingAvoidanceNeverInsolvent: a vying host with an avoidance borrow
// loses chips at every showdown beyond the pot (penalty cards in the shown
// hand). The Tier-0 stack rule used to count only the betting commitment, so a
// valid game could push a stack below the bet: the player was left with Fold
// as the only legal move (or negative chips) -- a forced fold the rulebook
// never mentions. With the penalty in the bound (genome.VyingWorstCaseCommitment)
// a stack funded exactly at the bound stays solvent in every reachable state.
func TestVyingAvoidanceNeverInsolvent(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Vying, Players: 2, HandSize: 5,
		Vying:    &genome.VyingParams{MinBet: 10, MaxRaises: 3, RoundsPerGame: 2},
		Scoring:  genome.ScoringConfig{CardPoints: []genome.CardScoring{{Suit: 3, Points: 20}}},
		Borrowed: []genome.BorrowedMechanic{{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance}},
	}
	// Fund the stack EXACTLY at the Tier-0 bound: the tightest valid game.
	g.Vying.StartingChips = g.VyingWorstCaseCommitment()
	if errs := genome.Validate(g); len(errs) != 0 {
		t.Fatalf("fixture at the solvency bound must be Tier-0 valid, got %v", errs)
	}
	hooks := mechanic.HooksFor(g)
	runner := &Runner{}
	for seed := uint64(0); seed < 400; seed++ {
		rng := rand.New(rand.NewPCG(seed, 1))
		st := runner.Setup(g, rng)
		for it := 0; it < 1000; it++ {
			runner.Upkeep(st, g)
			if runner.CheckEnd(st, g) >= 0 {
				break
			}
			for p, s := range st.Scores {
				if s < 0 {
					t.Fatalf("seed %d turn %d: player %d has negative chips (%d)", seed, st.Turn, p, s)
				}
			}
			moves := runner.GenerateMoves(st, g)
			if len(moves) == 1 && moves[0].Type == sim.MoveFold {
				t.Fatalf("seed %d turn %d: player %d (chips %d, bet %d) can only fold", seed, st.Turn, st.Active, st.Scores[st.Active], st.CurrentBet)
			}
			for _, e := range runner.ApplyMove(st, moves[rng.IntN(len(moves))], g) {
				for _, h := range hooks {
					h(st, g, e)
				}
			}
		}
		for p, s := range st.Scores {
			if s < 0 {
				t.Fatalf("seed %d: player %d finished with negative chips (%d)", seed, p, s)
			}
		}
	}
}
