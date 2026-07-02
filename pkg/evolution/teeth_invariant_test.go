package evolution

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// assertTeethInvariants checks, for every borrow g currently carries, the
// outcome-significance wiring giveBorrowTeeth guarantees. Teeth are an
// INVARIANT of a borrowed genome, not a graft-time event: MutateWith re-runs
// the repair after every mutation, so these must hold on every mutant --
// otherwise a later HandSize/MatchRule/RoundsPerGame/KnockThreshold tweak
// silently turns a live borrow vestigial (the "graft-time-only teeth" bug).
func assertTeethInvariants(t *testing.T, g *genome.Genome, ctx string) {
	t.Helper()
	for _, bm := range g.Borrowed {
		switch bm.Mechanic {
		case genome.MechAvoidance:
			if !avoidanceSetIsMeaningful(g) {
				t.Fatalf("%s: avoidance borrow with a meaningless penalty set (CardPoints: %v)", ctx, g.Scoring.CardPoints)
			}
			switch g.Skeleton {
			case genome.Shedding:
				if g.Shedding != nil && g.Shedding.RoundsPerGame < 2 {
					t.Fatalf("%s: avoidance borrow on single-round shedding (RoundsPerGame=%d)", ctx, g.Shedding.RoundsPerGame)
				}
			case genome.TrickTaking:
				if g.TrickTaking != nil {
					if g.TrickTaking.LeadRestriction != genome.LeadNoTrumpUntilBroken {
						t.Fatalf("%s: avoidance borrow on trick host without lead restriction", ctx)
					}
					if g.TrickTaking.RoundsPerGame < 2 {
						t.Fatalf("%s: avoidance borrow on single-round trick host (RoundsPerGame=%d)", ctx, g.TrickTaking.RoundsPerGame)
					}
				}
			case genome.Rummy:
				if g.Rummy != nil && g.Rummy.KnockThreshold < rummyAvoidanceKnockThreshold {
					t.Fatalf("%s: avoidance borrow on rummy with tight knock (%d < %d)", ctx, g.Rummy.KnockThreshold, rummyAvoidanceKnockThreshold)
				}
				for _, cp := range g.Scoring.CardPoints {
					if cp.Points < rummyAvoidancePenaltyPoints {
						t.Fatalf("%s: rummy avoidance penalty below deadwood scale (%d < %d)", ctx, cp.Points, rummyAvoidancePenaltyPoints)
					}
				}
			}
		case genome.MechMeldBonus, genome.MechTrickScoring:
			switch g.Skeleton {
			case genome.Shedding:
				if g.Shedding != nil && g.Shedding.RoundsPerGame < 2 {
					t.Fatalf("%s: banking borrow %d on single-round shedding (RoundsPerGame=%d)", ctx, bm.Mechanic, g.Shedding.RoundsPerGame)
				}
			case genome.TrickTaking:
				if g.TrickTaking != nil && g.TrickTaking.RoundsPerGame < 2 {
					t.Fatalf("%s: banking borrow %d on single-round trick host (RoundsPerGame=%d)", ctx, bm.Mechanic, g.TrickTaking.RoundsPerGame)
				}
			}
		case genome.MechDrawPenalty:
			switch g.Skeleton {
			case genome.Climbing:
				if g.HandSize*g.Players > 52-climbingDeckHeadroom {
					t.Fatalf("%s: draw-penalty borrow on climbing with no draw pile (hand %d x players %d > %d)", ctx, g.HandSize, g.Players, 52-climbingDeckHeadroom)
				}
			case genome.Rummy:
				if g.Rummy != nil && g.Rummy.KnockThreshold < rummyDrawKnockThreshold {
					t.Fatalf("%s: draw-penalty borrow on rummy with tight knock (%d < %d)", ctx, g.Rummy.KnockThreshold, rummyDrawKnockThreshold)
				}
			}
		case genome.MechRunPlay:
			if g.Skeleton == genome.Shedding && g.Shedding != nil {
				if g.HandSize < 6 {
					t.Fatalf("%s: run_play borrow with HandSize %d < 6 (combos cannot form)", ctx, g.HandSize)
				}
				if g.Shedding.MatchRule == genome.MatchBoth {
					t.Fatalf("%s: run_play borrow with MatchBoth (combos cannot match)", ctx)
				}
			}
		case genome.MechFollowSuit:
			if g.Skeleton == genome.Shedding && g.Shedding != nil {
				if g.HandSize < 6 {
					t.Fatalf("%s: follow_suit borrow with HandSize %d < 6 (constraint rarely binds)", ctx, g.HandSize)
				}
				if g.Shedding.MatchRule == genome.MatchRank || g.Shedding.MatchRule == genome.MatchBoth {
					t.Fatalf("%s: follow_suit borrow with MatchRule %d (suit cards unplayable, constraint collapses)", ctx, g.Shedding.MatchRule)
				}
			}
		}
	}
}

// toothyStarts returns one wired hybrid per teeth family: move-superset
// (run_play), move-restriction (follow_suit), banking (trick_scoring),
// avoidance-on-rummy, and draw-penalty-on-climbing.
func toothyStarts() map[string]*genome.Genome {
	return map[string]*genome.Genome{
		"shedding+run_play":      wiredHybrid(seeds.CrazyEights(), genome.Climbing, genome.MechRunPlay),
		"shedding+follow_suit":   wiredHybrid(seeds.CrazyEights(), genome.TrickTaking, genome.MechFollowSuit),
		"shedding+trick_scoring": wiredHybrid(seeds.CrazyEights(), genome.TrickTaking, genome.MechTrickScoring),
		"rummy+avoidance":        wiredHybrid(seeds.GinRummy(), genome.TrickTaking, genome.MechAvoidance),
		"climbing+draw_penalty":  wiredHybrid(seeds.BigTwo(), genome.Shedding, genome.MechDrawPenalty),
	}
}

// TestMutateWithPreservesTeethInvariants: a single MutateWith on a toothy
// genome must leave every carried borrow toothy -- tweakParameter can shrink
// HandSize and changeEnum can re-tighten MatchRule, so without the end-of-
// mutation repair these seeds find inert borrows within a handful of tries.
func TestMutateWithPreservesTeethInvariants(t *testing.T) {
	pool := allSeeds()
	for name, start := range toothyStarts() {
		for seed := uint64(0); seed < 300; seed++ {
			rng := rand.New(rand.NewPCG(seed, 1))
			child := MutateWith(start, rng, pool, true)
			assertTeethInvariants(t, child, fmt.Sprintf("%s seed %d", name, seed))
		}
	}
}

// TestMutationChainPreservesTeethInvariants: cumulative drift across a long
// mutation lineage (the production shape: each generation mutates the last)
// must also keep every surviving borrow toothy after EVERY step, including
// borrows added or re-added mid-chain.
func TestMutationChainPreservesTeethInvariants(t *testing.T) {
	pool := allSeeds()
	for name, start := range toothyStarts() {
		g := start
		rng := rand.New(rand.NewPCG(42, 2))
		for i := 0; i < 300; i++ {
			g = MutateWith(g, rng, pool, true)
			assertTeethInvariants(t, g, fmt.Sprintf("%s chain step %d", name, i))
		}
	}
}
