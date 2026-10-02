package judge

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func mustRunner(t *testing.T, g *genome.Genome) sim.GenericRunner {
	t.Helper()
	r := fitness.GetRunner(g)
	if r == nil {
		t.Fatalf("no runner for %v", g.Skeleton)
	}
	return r
}

func mustAI(g *genome.Genome) sim.AIPlayer {
	return fitness.GetGreedyAI(g)
}

// runPlayMeldShedding returns a Tier-0-valid multi-round shedding genome
// carrying the run_play deep borrow plus the hook-scored meld_bonus borrow --
// the published "novel hybrid" shape (results/2026-06-14-evolved-novel-hybrids).
// Its WINNER is decided by the banked meld points, so a simulation that skips
// the borrow hooks plays a different game.
func runPlayMeldShedding(t *testing.T) *genome.Genome {
	t.Helper()
	g := &genome.Genome{
		ID:       "hybrid",
		Skeleton: genome.Shedding,
		Players:  4,
		HandSize: 11,
		Shedding: &genome.SheddingParams{
			MatchRule:     genome.MatchEither,
			DrawPenalty:   2,
			RoundsPerGame: 3,
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.Climbing, Mechanic: genome.MechRunPlay},
			{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
		},
	}
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("run_play+meld_bonus fixture fails Tier-0 validation: %v", errs)
	}
	return g
}

// drawPenaltyRummy returns a Tier-0-valid rummy genome carrying the
// hook-driven draw_penalty borrow (the flagship-r4 rank24 shape). The hook
// changes hand sizes, so it changes how often and how fast the game completes.
func drawPenaltyRummy(t *testing.T) *genome.Genome {
	t.Helper()
	g := &genome.Genome{
		ID:       "penalty-rummy",
		Skeleton: genome.Rummy,
		Players:  3,
		HandSize: 11,
		Rummy: &genome.RummyParams{
			MeldTypes:      genome.MeldBoth,
			MinMeldSize:    3,
			DrawFrom:       genome.DrawEither,
			KnockThreshold: 27,
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.Shedding, Mechanic: genome.MechDrawPenalty},
		},
	}
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("rummy+draw_penalty fixture fails Tier-0 validation: %v", errs)
	}
	return g
}
