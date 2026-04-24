package evolution

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

func TestMutateScoringNeverGeneratesInvalidRank(t *testing.T) {
	// Valid card ranks are 2-14 (per pkg/sim/card.go); Rank=0 means
	// "all ranks" wildcard. Rank=1 is invalid: it never matches any
	// card in cardPointValue, so any scoring rule with Rank=1 is dead.
	for seed := uint64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		g := &genome.Genome{
			Scoring: genome.ScoringConfig{CardPoints: nil},
		}
		mutateScoring(g, rng)
		if len(g.Scoring.CardPoints) == 0 {
			t.Fatalf("seed %d: expected mutateScoring to add a card point rule", seed)
		}
		got := g.Scoring.CardPoints[0].Rank
		if got == 1 {
			t.Errorf("seed %d: mutateScoring produced invalid Rank=1", seed)
		}
		if got < 2 || got > 14 {
			t.Errorf("seed %d: mutateScoring produced out-of-range Rank=%d (want 2-14)", seed, got)
		}
	}
}
