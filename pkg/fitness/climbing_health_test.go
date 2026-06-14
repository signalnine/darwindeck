package fitness_test

// Climbing skeleton HEALTH checks (novelty evolution, fourth skeleton).
//
// These are deliberately SEPARATE from the calibration ground-truth suite
// (calibration_test.go): the climbing seed (seeds.BigTwo) is NOT in seeds.All()
// because the project has no human fun-rating for a climbing game to calibrate
// against. This file only asserts the climbing skeleton is PLAYABLE and HEALTHY
// through the real pipeline -- it never compares climbing fitness against the
// classics, so it cannot perturb the calibration gate.

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestClimbingSeedIsHealthyAcrossSeeds: the Big Two seed must pass Tier 1 on
// every calibration seed, never trip a degeneracy veto (the general vetoes
// apply to climbing batches), and evaluate as VALID -- the playability-by-
// construction guarantee, end to end.
func TestClimbingSeedIsHealthyAcrossSeeds(t *testing.T) {
	g := seeds.BigTwo()
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("BigTwo seed is not Tier-0 valid: %v", errs)
	}
	for _, seed := range fitness.CalibrationSeeds {
		res := fitness.Evaluate(g, seed)
		if !res.Tier1.Passed {
			t.Errorf("seed %d: BigTwo failed Tier 1 (%q) -- a playable-by-construction skeleton must pass", seed, res.Tier1.Reason)
		}
		if res.DegenerateReason != "" {
			t.Errorf("seed %d: BigTwo flagged degenerate %q -- the seed must read as a healthy game", seed, res.DegenerateReason)
		}
		if !res.Valid {
			t.Errorf("seed %d: BigTwo evaluated invalid (tier0=%v tier1=%q)", seed, res.Tier0Errors, res.Tier1.Reason)
		}
	}
}

// TestClimbingVariantsArePlayable: a spread of climbing parameter shapes (the
// extremes of the search space) all pass Tier 1 and complete games. This pins
// the playability invariant across the whole parameter range, not just the
// seed's shape.
func TestClimbingVariantsArePlayable(t *testing.T) {
	mk := func(players, hand int, pairs, triples, runs bool, minRun int) *genome.Genome {
		return &genome.Genome{
			ID: "variant", Skeleton: genome.Climbing, Players: players, HandSize: hand,
			Climbing: &genome.ClimbingParams{AllowPairs: pairs, AllowTriples: triples, AllowRuns: runs, MinRunLen: minRun},
		}
	}
	variants := []*genome.Genome{
		mk(2, 3, false, false, false, 0),  // minimal singles-only
		mk(2, 13, true, true, true, 3),    // full Big Two, 2p
		mk(6, 8, true, true, true, 3),     // max players
		mk(2, 13, false, false, true, 5),  // runs-only, long
		mk(4, 13, true, false, false, 0),  // pairs-only
	}
	for _, g := range variants {
		if errs := genome.Validate(g); len(errs) > 0 {
			t.Errorf("%s shape invalid: %v", g.ActiveParams(), errs)
			continue
		}
		passed := 0
		for _, seed := range fitness.CalibrationSeeds {
			if fitness.Evaluate(g, seed).Tier1.Passed {
				passed++
			}
		}
		if passed < len(fitness.CalibrationSeeds) {
			t.Errorf("%s passed Tier 1 only %d/%d seeds -- a playable-by-construction game must pass every seed",
				g.ActiveParams(), passed, len(fitness.CalibrationSeeds))
		}
	}
}
