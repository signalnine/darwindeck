package evolution

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// climbingVariant builds a valid climbing genome for crossover/mutation tests.
func climbingVariant(id string, players, hand int, pairs, triples, runs bool, minRun int) *genome.Genome {
	return &genome.Genome{
		ID:       id,
		Skeleton: genome.Climbing,
		Players:  players,
		HandSize: hand,
		Climbing: &genome.ClimbingParams{
			AllowPairs:   pairs,
			AllowTriples: triples,
			AllowRuns:    runs,
			MinRunLen:    minRun,
		},
	}
}

// TestClimbingSameSkeletonCrossoverStaysValid: same-skeleton climbing crossover
// produces a valid climbing child on every seed, and the child's combination
// toggles come from one of the two parents.
func TestClimbingSameSkeletonCrossoverStaysValid(t *testing.T) {
	a := seeds.BigTwo()
	b := climbingVariant("climb-b", 3, 10, true, false, false, 0)
	for seed := uint64(0); seed < 100; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		child := CrossoverWith(a, b, rng, false)
		if child == nil {
			t.Fatalf("seed %d: same-skeleton climbing crossover returned nil", seed)
		}
		if child.Skeleton != genome.Climbing {
			t.Fatalf("seed %d: climbing crossover changed skeleton to %s", seed, child.Skeleton)
		}
		if child.Climbing == nil {
			t.Fatalf("seed %d: climbing child missing climbing params", seed)
		}
		if errs := genome.Validate(child); len(errs) > 0 {
			t.Fatalf("seed %d: climbing crossover child invalid: %v", seed, errs)
		}
	}
}

// TestClimbingCrossoverExchangesParams: across many seeds, crossover must be
// able to pull each combination toggle from parent B (not just clone A).
func TestClimbingCrossoverExchangesParams(t *testing.T) {
	// A: all off. B: all on. A child taking a B value flips a toggle on.
	a := climbingVariant("a", 2, 13, false, false, false, 0)
	b := climbingVariant("b", 2, 13, true, true, true, 5)
	sawPairsFromB, sawRunsFromB := false, false
	for seed := uint64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		child := CrossoverWith(a, b, rng, false)
		if child.Climbing.AllowPairs {
			sawPairsFromB = true
		}
		if child.Climbing.AllowRuns {
			sawRunsFromB = true
		}
	}
	if !sawPairsFromB {
		t.Error("crossover never pulled AllowPairs from parent B")
	}
	if !sawRunsFromB {
		t.Error("crossover never pulled AllowRuns from parent B")
	}
}

// TestClimbingMutationStaysValid: repeated mutation of a climbing seed always
// yields a Tier-0-valid genome (the standard mutation-validity property).
func TestClimbingMutationStaysValid(t *testing.T) {
	g := seeds.BigTwo()
	for seed := uint64(0); seed < 2000; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		child := MutateWith(g, rng, seeds.All(), true)
		if errs := genome.Validate(child); len(errs) > 0 {
			t.Fatalf("seed %d: mutated genome invalid: %v (skeleton=%s)", seed, errs, child.Skeleton)
		}
	}
}

// TestClimbingMutationReachesAllToggles: mutation must be able to flip each
// climbing combination toggle and move MinRunLen, so none of the params are
// inert under evolution.
func TestClimbingMutationReachesAllToggles(t *testing.T) {
	g := climbingVariant("base", 2, 13, false, false, false, 0)
	var flippedPairs, flippedTriples, flippedRuns, movedRunLen bool
	for seed := uint64(0); seed < 5000; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		child := MutateWith(g, rng, nil, false) // nil seeds: never changeSkeleton
		if child.Skeleton != genome.Climbing || child.Climbing == nil {
			continue
		}
		if child.Climbing.AllowPairs {
			flippedPairs = true
		}
		if child.Climbing.AllowTriples {
			flippedTriples = true
		}
		if child.Climbing.AllowRuns {
			flippedRuns = true
		}
		if child.Climbing.MinRunLen != g.Climbing.MinRunLen {
			movedRunLen = true
		}
	}
	if !flippedPairs {
		t.Error("mutation never turned AllowPairs on")
	}
	if !flippedTriples {
		t.Error("mutation never turned AllowTriples on")
	}
	if !flippedRuns {
		t.Error("mutation never turned AllowRuns on")
	}
	if !movedRunLen {
		t.Error("mutation never moved MinRunLen")
	}
}

// TestClimbingAsHybridHostBorrowsDrawPenalty: crossing a climbing parent (host)
// with a shedding parent under the flag must, on some seed, produce a climbing
// host carrying a LIVE MechDrawPenalty borrow -- the only whitelisted climbing
// borrow, whose hook both fires and affects the hand-based winner. The hybrid
// must be valid and survive the real pipeline on at least one calibration seed.
func TestClimbingAsHybridHostBorrowsDrawPenalty(t *testing.T) {
	var champion *genome.Genome
	for seed := uint64(0); seed < 300 && champion == nil; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		child := CrossoverWith(seeds.BigTwo(), seeds.CrazyEights(), rng, true)
		if child == nil || child.Skeleton != genome.Climbing {
			continue
		}
		if errs := genome.Validate(child); len(errs) > 0 {
			t.Fatalf("seed %d: climbing hybrid invalid: %v", seed, errs)
		}
		for _, bm := range child.Borrowed {
			if bm.Mechanic == genome.MechDrawPenalty {
				champion = child
			}
		}
	}
	if champion == nil {
		t.Fatal("no climbing+draw-penalty hybrid produced across 300 seeds")
	}

	// MechDrawPenalty acts directly (appends cards), so it is always live.
	live := champion.LiveBorrows()
	foundDP := false
	for _, bm := range live {
		if bm.Mechanic == genome.MechDrawPenalty {
			foundDP = true
		}
		if bm.Source == champion.Skeleton {
			t.Fatalf("climbing hybrid borrow sourced from its own skeleton: %+v", bm)
		}
	}
	if !foundDP {
		t.Fatalf("MechDrawPenalty not live on climbing hybrid: live=%+v", live)
	}

	survived := 0
	for _, s := range fitness.CalibrationSeeds {
		if fitness.Evaluate(champion, s).Valid {
			survived++
		}
	}
	if survived == 0 {
		t.Error("climbing+draw-penalty hybrid was killed on every calibration seed")
	}
	t.Logf("climbing+draw-penalty hybrid survived %d/%d calibration seeds", survived, len(fitness.CalibrationSeeds))
}

// TestClimbingAsCrossPartnerKeepsHostValid: when climbing is the OTHER parent
// (host is shedding/trick/rummy), the hybrid child must still be a valid genome
// of the host's skeleton (climbing contributes no distinct scoring mechanic, so
// the host falls back to its own cross-family candidates -- the child is never
// invalid).
func TestClimbingAsCrossPartnerKeepsHostValid(t *testing.T) {
	hosts := []*genome.Genome{seeds.CrazyEights(), seeds.Whist(), seeds.GinRummy()}
	climber := seeds.BigTwo()
	for _, host := range hosts {
		t.Run(host.Skeleton.String(), func(t *testing.T) {
			for seed := uint64(0); seed < 80; seed++ {
				rng := rand.New(rand.NewPCG(seed, 0))
				child := CrossoverWith(host, climber, rng, true)
				if child == nil {
					t.Fatalf("seed %d: hybrid with climbing partner returned nil", seed)
				}
				if child.Skeleton != host.Skeleton {
					t.Fatalf("seed %d: host skeleton changed to %s", seed, child.Skeleton)
				}
				if errs := genome.Validate(child); len(errs) > 0 {
					t.Fatalf("seed %d: hybrid invalid: %v", seed, errs)
				}
			}
		})
	}
}

// TestClimbingDrawPenaltyMutationReachable: mutation can add the MechDrawPenalty
// borrow to a climbing host (so the borrow is reachable through mutation, not
// only crossover).
func TestClimbingDrawPenaltyMutationReachable(t *testing.T) {
	g := seeds.BigTwo()
	seen := false
	for seed := uint64(0); seed < 5000 && !seen; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		child := MutateWith(g, rng, nil, true)
		if child.Skeleton != genome.Climbing {
			continue
		}
		for _, bm := range child.Borrowed {
			if bm.Mechanic == genome.MechDrawPenalty {
				seen = true
			}
		}
	}
	if !seen {
		t.Error("mutation never added MechDrawPenalty to a climbing host across 5000 seeds")
	}
}
