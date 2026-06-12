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
