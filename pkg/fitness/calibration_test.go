//go:build calibration

// Package fitness_test: seed-calibration suite (audit remediation Task 2).
//
// This is the falsifiable acceptance test for the whole remediation plan.
// It is committed FAILING by design, gated by the `calibration` build tag:
//
//	go test -tags calibration ./pkg/fitness/ -run TestCalibration -v
//
// The tag is removed in Task 14, at which point these tests must pass in the
// default suite and become the fitness function's permanent regression gate.
package fitness_test

import (
	"math"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// CalibrationSeeds is the canonical pinned seed list for ALL calibration
// evaluations. Every task that measures seed-game fitness uses this list --
// never ad-hoc seeds -- so numbers are comparable across the whole plan.
var CalibrationSeeds = []uint64{11, 22, 33, 44, 55, 66, 77, 88, 99, 110}

// BASELINE (measured at the Task 2 commit, BEFORE any metric fixes; means
// over CalibrationSeeds, sd in parens, n = evaluations of 10 that passed
// Tier 1):
//
//	classic crazy-eights        0.725 (sd 0.016, n  9/10)
//	classic mau-mau             0.722 (sd 0.010, n 10/10)
//	classic whist               0.636 (sd 0.014, n 10/10)
//	classic hearts              0.692 (sd 0.006, n 10/10)
//	classic spades              0.660 (sd 0.017, n 10/10)
//	classic oh-hell             0.667 (sd 0.014, n  9/10)
//	classic gin-rummy           0.813 (sd 0.016, n  8/10)
//	classic knock-rummy         0.823 (sd 0.012, n  9/10)
//	degen   instant-knock-rummy 0.826 (sd 0.023, n  7/10)
//	degen   forced-shedding     0.716 (sd 0.000, n  1/10)
//
// Failure shape (the audit's falsification finding; Task 14 must invert it):
//   - The instant-knock fixture (0.826) outscores EVERY classic, including
//     gin rummy (0.813) -- TestCalibrationGinBeatsInstantKnock needs +0.10.
//   - Worst classic whist (0.636) trails the best degenerate by 0.190.
//   - All four trick-taking classics (whist/hearts/spades/oh-hell) sit below
//     the 0.70 FitnessFloor, so QD selection zeroes their gradient.
//   - forced-shedding's single surviving eval (0.716) still beats four
//     classics; it was Tier 1-killed on 9/10 seeds by single-game timeouts
//     (the false-reject noise Task 16 addresses).

// Ground truth: the 8 classic seeds are the only human-validated "fun" games
// in the repo. Any fitness function that scores a classic below a degenerate
// fixture is falsified. Evaluations are averaged over the pinned seed list
// because per-eval noise is sd ~0.02.
//
// OVERFITTING GUARD: 8 classics is a narrow truth source. Tune to MARGINS
// (worst classic must beat best degenerate by 0.05), never to exact
// orderings among the classics; as more classics are added (Hoyle-derived
// trick/shedding/rummy variants), the suite grows but thresholds do not move.

// calResult is one genome's calibration measurement.
type calResult struct {
	mean  float64
	sd    float64
	valid int // evaluations that passed Tier 1 (of len(CalibrationSeeds))
}

// calCache memoizes per-genome measurements so the three gate tests share one
// evaluation pass. Tests in this package do not use t.Parallel, so a plain
// map is safe.
var calCache = map[string]calResult{}

// meanFit evaluates g once per calibration seed through the REAL pipeline
// (fitness.Evaluate: Tier 0 -> Tier 1 -> Tier 2) and returns the mean
// TotalFitness over evaluations that reached Tier 2. Tier 1 kills are
// excluded from the mean (and logged): Tier 1 false-reject noise is a
// separate finding with its own task (Task 16); this suite measures the
// metric stack. A genome killed by Tier 1 on EVERY seed scores 0 -- the
// pipeline rejecting a degenerate outright satisfies the gate.
func meanFit(t *testing.T, g *genome.Genome) calResult {
	t.Helper()
	if r, ok := calCache[g.ID]; ok {
		return r
	}

	var fits []float64
	for _, seed := range CalibrationSeeds {
		res := fitness.Evaluate(g, seed)
		if len(res.Tier0Errors) > 0 {
			t.Fatalf("%s: tier-0 errors (fixture must be statically valid): %v", g.ID, res.Tier0Errors)
		}
		if !res.Valid {
			t.Logf("%s: seed %d killed by tier 1: %s", g.ID, seed, res.Tier1.Reason)
			continue
		}
		fits = append(fits, res.Metrics.TotalFitness)
	}

	r := calResult{valid: len(fits)}
	if len(fits) > 0 {
		var sum float64
		for _, f := range fits {
			sum += f
		}
		r.mean = sum / float64(len(fits))
		var sq float64
		for _, f := range fits {
			sq += (f - r.mean) * (f - r.mean)
		}
		r.sd = math.Sqrt(sq / float64(len(fits)))
	}
	calCache[g.ID] = r
	t.Logf("%s: mean %.3f (sd %.3f, n %d/%d)", g.ID, r.mean, r.sd, r.valid, len(CalibrationSeeds))
	return r
}

// TestDegenerateFixturesAreTier0Valid pins the fixture contract: degenerate
// fixtures (and all classics) must pass genome.Validate -- they are negative
// ground truth for the METRICS, so they must not be rejectable by static
// analysis.
func TestDegenerateFixturesAreTier0Valid(t *testing.T) {
	all := append(seeds.All(), seeds.InstantKnockRummy(), seeds.ForcedShedding())
	for _, g := range all {
		if errs := genome.Validate(g); len(errs) > 0 {
			t.Errorf("%s: tier-0 violations: %v", g.ID, errs)
		}
	}
	if n := len(seeds.All()); n != 8 {
		t.Errorf("seeds.All() returned %d classics, want 8", n)
	}
}

// TestCalibrationClassicsBeatDegenerates is the core gate: the worst classic
// must beat the best degenerate fixture by a 0.05 margin.
func TestCalibrationClassicsBeatDegenerates(t *testing.T) {
	classics := seeds.All() // 8 games
	degens := []*genome.Genome{seeds.InstantKnockRummy(), seeds.ForcedShedding()}

	worstClassic, bestDegen := 1.0, 0.0
	worstName, bestName := "", ""
	for _, c := range classics {
		if m := meanFit(t, c).mean; m < worstClassic {
			worstClassic, worstName = m, c.ID
		}
	}
	for _, d := range degens {
		if m := meanFit(t, d).mean; m > bestDegen {
			bestDegen, bestName = m, d.ID
		}
	}

	if worstClassic < bestDegen+0.05 {
		t.Errorf("calibration failed: worst classic %s %.3f vs best degenerate %s %.3f (need +0.05 margin)",
			worstName, worstClassic, bestName, bestDegen)
	}
}

// TestCalibrationClassicsAboveFloor: every classic must clear the QD
// viability floor (evolution.FitnessFloor; derived from calibration in
// Task 15). A floor that rejects human-validated games zeroes the selection
// gradient for exactly the genomes evolution should be reaching for.
func TestCalibrationClassicsAboveFloor(t *testing.T) {
	for _, c := range seeds.All() {
		if m := meanFit(t, c).mean; m < evolution.FitnessFloor {
			t.Errorf("classic %s mean fitness %.3f below FitnessFloor %.2f", c.ID, m, evolution.FitnessFloor)
		}
	}
}

// TestCalibrationGinBeatsInstantKnock: the named pair from the audit. The
// flagship "champion" was an instant-knock coin flip scored ABOVE gin rummy;
// any sane metric stack separates them by a wide margin.
func TestCalibrationGinBeatsInstantKnock(t *testing.T) {
	gin := meanFit(t, seeds.GinRummy()).mean
	knock := meanFit(t, seeds.InstantKnockRummy()).mean
	if gin < knock+0.10 {
		t.Errorf("gin rummy %.3f does not beat instant-knock %.3f by 0.10", gin, knock)
	}
}
