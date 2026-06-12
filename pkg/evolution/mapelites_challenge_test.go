package evolution

import (
	"math"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// archiveQDConsistent recomputes the QD-score from the archive cells'
// published fitness and compares it to the bookkept value. Challenge
// re-evaluation moves incumbents' published means, so the accounting must
// follow every adjustment.
func archiveQDConsistent(t *testing.T, e *MAPElitesEngine, skel genome.SkeletonType) {
	t.Helper()
	archive := e.Archives[skel]
	sum := 0.0
	occupied := 0
	for r := 0; r < GridSize; r++ {
		for c := 0; c < GridSize; c++ {
			if cell := archive.Cells[r][c]; cell != nil {
				sum += cell.Individual.Fitness.TotalFitness
				occupied++
			}
		}
	}
	if math.Abs(archive.QDScore-sum) > 1e-9 {
		t.Fatalf("QDScore = %.6f, want %.6f (sum of published cell fitness)", archive.QDScore, sum)
	}
	if archive.Occupied != occupied {
		t.Fatalf("Occupied = %d, want %d", archive.Occupied, occupied)
	}
}

// stubEval returns an eval seam that always reports a valid evaluation with
// the given TotalFitness, recording each seed it was asked to evaluate.
func stubEval(value float64, seeds *[]uint64) func(*genome.Genome, uint64) fitness.EvaluationResult {
	return func(g *genome.Genome, seed uint64) fitness.EvaluationResult {
		if seeds != nil {
			*seeds = append(*seeds, seed)
		}
		return fitness.EvaluationResult{
			Valid:   true,
			Metrics: fitness.Metrics{TotalFitness: value},
		}
	}
}

// TestChallengeReevaluationEvictsLuckyIncumbent pins the MAP-Elites
// winner's-curse fix (reviewer finding 3): the old insert admitted on a
// single-seed evaluation and NEVER re-evaluated, so a lucky high eval (the
// instant-knock class: 0.431 on its one surviving seed, clearing the 0.42
// output floor) held its cell permanently. Now every challenge to an
// occupied cell re-evaluates the incumbent once at a fresh seed and compares
// running means on both sides; repeated challenges drag a lucky mean toward
// truth until a genuinely better challenger takes the cell. Cost stays
// bounded: re-evaluations happen on cell collisions only.
func TestChallengeReevaluationEvictsLuckyIncumbent(t *testing.T) {
	e := NewMAPElitesEngine(Config{BaseSeed: 1, Workers: 1}, allSeeds())
	var evalSeeds []uint64
	e.evaluate = stubEval(0.45, &evalSeeds) // the incumbent's "true" per-eval fitness

	b := BehaviorDescriptor{0.55, 0.25}
	lucky := &genome.Genome{ID: "lucky", Skeleton: genome.Shedding}
	challenger := &genome.Genome{ID: "challenger", Skeleton: genome.Shedding}

	if !e.insert(lucky, fitness.Metrics{TotalFitness: 0.95}, b) {
		t.Fatal("first insert into an empty cell must succeed")
	}
	cell := e.Archives[genome.Shedding].Cells[2][5]
	if cell == nil {
		t.Fatal("cell (2,5) empty after insert")
	}
	if cell.Individual.EvalCount != 1 || cell.Individual.FitnessSum != 0.95 {
		t.Fatalf("admitted occupant must start its running mean: EvalCount=%d FitnessSum=%.2f, want 1/0.95",
			cell.Individual.EvalCount, cell.Individual.FitnessSum)
	}
	archiveQDConsistent(t, e, genome.Shedding)

	// Expected incumbent means after k challenges: (0.95 + 0.45k)/(k+1).
	// Challenge 1: 0.700, challenge 2: 0.617 (challenger 0.60 still loses),
	// challenge 3: 0.575 (challenger takes the cell).
	wantMeans := []float64{0.70, (0.95 + 0.90) / 3}
	for k, want := range wantMeans {
		if e.insert(challenger, fitness.Metrics{TotalFitness: 0.60}, b) {
			t.Fatalf("challenge %d: challenger admitted while incumbent mean should still be above 0.60", k+1)
		}
		cell = e.Archives[genome.Shedding].Cells[2][5]
		if cell.Individual.Genome.ID != "lucky" {
			t.Fatalf("challenge %d: cell flipped early to %q", k+1, cell.Individual.Genome.ID)
		}
		if math.Abs(cell.Individual.Fitness.TotalFitness-want) > 1e-9 {
			t.Fatalf("challenge %d: incumbent published mean = %.6f, want %.6f",
				k+1, cell.Individual.Fitness.TotalFitness, want)
		}
		archiveQDConsistent(t, e, genome.Shedding)
	}

	if !e.insert(challenger, fitness.Metrics{TotalFitness: 0.60}, b) {
		t.Fatal("challenge 3: challenger must take the cell once the lucky mean fell below 0.60")
	}
	cell = e.Archives[genome.Shedding].Cells[2][5]
	if cell.Individual.Genome.ID != "challenger" {
		t.Fatalf("cell held by %q, want challenger", cell.Individual.Genome.ID)
	}
	if cell.Individual.EvalCount != 1 || math.Abs(cell.Individual.FitnessSum-0.60) > 1e-12 {
		t.Fatalf("new occupant must start a fresh running mean: EvalCount=%d FitnessSum=%.2f, want 1/0.60",
			cell.Individual.EvalCount, cell.Individual.FitnessSum)
	}
	if e.Archives[genome.Shedding].Occupied != 1 {
		t.Fatalf("Occupied = %d, want 1", e.Archives[genome.Shedding].Occupied)
	}
	archiveQDConsistent(t, e, genome.Shedding)

	// Fresh-seed discipline: one re-evaluation per challenge, all seeds
	// distinct and outside the offspring evaluation band.
	if len(evalSeeds) != 3 {
		t.Fatalf("incumbent re-evaluated %d times, want 3 (once per challenge)", len(evalSeeds))
	}
	seen := map[uint64]bool{}
	for _, s := range evalSeeds {
		if seen[s] {
			t.Fatalf("challenge seed %d reused; incumbent re-evaluations must use fresh seeds", s)
		}
		seen[s] = true
		if s < mapElitesChallengeSeedBase {
			t.Fatalf("challenge seed %d inside the offspring evaluation band (< %d)", s, uint64(mapElitesChallengeSeedBase))
		}
	}
}

// TestChallengeReevaluationInvalidIncumbentLosesHistory: an incumbent whose
// re-evaluation fails Tier 0/1 is flaky -- its history resets (the engines'
// policy), its mean reads 0, and any valid challenger takes the cell.
func TestChallengeReevaluationInvalidIncumbentLosesHistory(t *testing.T) {
	e := NewMAPElitesEngine(Config{BaseSeed: 1, Workers: 1}, allSeeds())
	e.evaluate = func(g *genome.Genome, seed uint64) fitness.EvaluationResult {
		return fitness.EvaluationResult{Valid: false}
	}

	b := BehaviorDescriptor{0.15, 0.85}
	flaky := &genome.Genome{ID: "flaky", Skeleton: genome.Rummy}
	weak := &genome.Genome{ID: "weak-but-valid", Skeleton: genome.Rummy}

	e.insert(flaky, fitness.Metrics{TotalFitness: 0.90}, b)
	if !e.insert(weak, fitness.Metrics{TotalFitness: 0.30}, b) {
		t.Fatal("a valid challenger must beat an incumbent that failed re-qualification")
	}
	row, col := b.GridCell(GridSize)
	cell := e.Archives[genome.Rummy].Cells[row][col]
	if cell.Individual.Genome.ID != "weak-but-valid" {
		t.Fatalf("cell held by %q, want weak-but-valid", cell.Individual.Genome.ID)
	}
	archiveQDConsistent(t, e, genome.Rummy)
}

// TestChallengeReevaluationRealPipeline drives one challenge through the
// real fitness.Evaluate seam: the incumbent's EvalCount must advance (or
// reset, if Tier 1 kills that seed -- decided deterministically up front so
// the test never flakes).
func TestChallengeReevaluationRealPipeline(t *testing.T) {
	cfg := Config{BaseSeed: 1, Workers: 1}
	e := NewMAPElitesEngine(cfg, allSeeds())

	g := seeds.CrazyEights()
	b := BehaviorDescriptor{0.35, 0.35}
	e.insert(g, fitness.Metrics{TotalFitness: 0.95}, b)

	// The first challenge re-evaluates at this exact seed (counter starts
	// at 0); precompute the reference to know which branch to assert.
	refSeed := cfg.BaseSeed + mapElitesChallengeSeedBase
	ref := fitness.Evaluate(g, refSeed)

	e.insert(&genome.Genome{ID: "ch", Skeleton: genome.Shedding}, fitness.Metrics{TotalFitness: 0.01}, b)

	row, col := b.GridCell(GridSize)
	cell := e.Archives[genome.Shedding].Cells[row][col]
	if ref.Valid {
		wantMean := (0.95 + ref.Metrics.TotalFitness) / 2
		if cell.Individual.EvalCount != 2 {
			t.Fatalf("incumbent EvalCount = %d, want 2 after one real re-evaluation", cell.Individual.EvalCount)
		}
		if math.Abs(cell.Individual.Fitness.TotalFitness-wantMean) > 1e-9 {
			t.Fatalf("incumbent published mean = %.6f, want %.6f", cell.Individual.Fitness.TotalFitness, wantMean)
		}
		if cell.Individual.Genome.ID != g.ID {
			t.Fatalf("0.01 challenger must not take the cell from a valid incumbent")
		}
	} else {
		// Flaky at refSeed: history reset, weak challenger takes the cell.
		if cell.Individual.Genome.ID != "ch" {
			t.Fatalf("incumbent failed re-qualification but kept the cell")
		}
	}
	archiveQDConsistent(t, e, genome.Shedding)
}
