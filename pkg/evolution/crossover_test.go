package evolution

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// TestCrossoverExchangesSheddingRounds (Task 22): RoundsPerGame must
// participate in uniform shedding crossover like every other param -- a
// child takes parent A's or parent B's value, never anything else.
func TestCrossoverExchangesSheddingRounds(t *testing.T) {
	mk := func(rounds int) *genome.Genome {
		return &genome.Genome{
			ID:       "x",
			Skeleton: genome.Shedding,
			Players:  2,
			HandSize: 7,
			Shedding: &genome.SheddingParams{
				MatchRule:     genome.MatchEither,
				DrawPenalty:   1,
				RoundsPerGame: rounds,
			},
		}
	}
	a, b := mk(1), mk(5)

	sawA, sawB := false, false
	for seed := uint64(0); seed < 100; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		child := Crossover(a, b, rng)
		if child == nil {
			t.Fatal("same-skeleton crossover returned nil")
		}
		switch child.Shedding.RoundsPerGame {
		case 1:
			sawA = true
		case 5:
			sawB = true
		default:
			t.Fatalf("seed %d: child RoundsPerGame = %d, want parent value 1 or 5", seed, child.Shedding.RoundsPerGame)
		}
	}
	if !sawA || !sawB {
		t.Errorf("crossover never exchanged RoundsPerGame (sawA=%v sawB=%v): field excluded from crossoverShedding", sawA, sawB)
	}
}

// TestCrossoverRepairsScoringBorrowRounds (round 3 commit 6b): the Borrowed
// and Shedding.RoundsPerGame coin flips are independent, so a child can
// inherit a scoring borrow from one parent and single-round play from the
// other -- the inert combination mutation can no longer produce.
// repairCrossoverInvariants must restore RoundsPerGame >= 2 (and CardPoints
// for an avoidance borrow), keeping Crossover's "valid in, coherent out"
// contract.
func TestCrossoverRepairsScoringBorrowRounds(t *testing.T) {
	mk := func(borrow bool, rounds int) *genome.Genome {
		g := &genome.Genome{
			Skeleton: genome.Shedding,
			Players:  2,
			HandSize: 7,
			Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1, RoundsPerGame: rounds},
		}
		if borrow {
			g.Borrowed = []genome.BorrowedMechanic{
				{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
			}
			g.Scoring.CardPoints = []genome.CardScoring{{Suit: 3, Points: 1}}
		}
		return g
	}

	sawBorrowChild := false
	for seed := uint64(0); seed < 500; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		child := Crossover(mk(true, 3), mk(false, 1), rng)
		if child == nil {
			t.Fatalf("seed %d: same-skeleton crossover returned nil", seed)
		}
		if !child.HasScoringBorrow() {
			continue
		}
		sawBorrowChild = true
		if child.Shedding.RoundsPerGame < 2 {
			t.Fatalf("seed %d: child carries a scoring borrow with RoundsPerGame %d (inert combination)",
				seed, child.Shedding.RoundsPerGame)
		}
		for _, bm := range child.Borrowed {
			if bm.Mechanic == genome.MechAvoidance && len(child.Scoring.CardPoints) == 0 {
				t.Fatalf("seed %d: child carries MechAvoidance without CardPoints", seed)
			}
		}
	}
	if !sawBorrowChild {
		t.Fatal("coverage: no child ever inherited the scoring borrow in 500 trials")
	}
}
