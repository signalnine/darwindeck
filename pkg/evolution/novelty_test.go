package evolution

import (
	"fmt"
	"math"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// synthNovelty builds a synthetic valid NoveltyIndividual for archive and
// admission tests. The bare genome carries only the fields computeNovelty
// touches (Skeleton, ID, Fitness).
func synthNovelty(skel genome.SkeletonType, id string, fit float64, b BehaviorDescriptor) *NoveltyIndividual {
	return &NoveltyIndividual{
		Individual: Individual{
			Genome:  &genome.Genome{ID: id, Skeleton: skel},
			Valid:   true,
			Fitness: fitness.Metrics{TotalFitness: fit},
		},
		Behavior: b,
	}
}

// TestNoveltyEliteRunningMeanFitness pins the winner's-curse fix for the
// novelty engine (carried finding (a) of the 2026-06-11 checkpoint: engine.go
// got the fix in f882a67, novelty.go kept the `if ind.Valid { continue }`
// skip). An individual kept across 5 generations must be re-evaluated each
// generation with a fresh seed, report the MEAN of the 5 evaluations (not the
// first, not the max), and BestFitness must track that mean instead of a
// sticky historical max.
func TestNoveltyEliteRunningMeanFitness(t *testing.T) {
	cfg := Config{
		PopulationSize: 1,
		EliteSize:      1,
		TournamentSize: 1,
		Workers:        1,
		BaseSeed:       42, // same derivation as engine.go; verified valid evals for CrazyEights
	}
	e := NewNoveltyEngine(cfg, allSeeds())
	g := seeds.CrazyEights()
	ind := &NoveltyIndividual{Individual: Individual{Genome: g}}
	e.Population = []*NoveltyIndividual{ind}

	var raws []float64
	for gen := 0; gen < 5; gen++ {
		e.Generation = gen
		e.evaluatePopulation()

		r := fitness.Evaluate(g, cfg.BaseSeed+uint64(gen)*10000)
		if !r.Valid {
			t.Fatalf("gen %d: reference evaluation invalid", gen)
		}
		raws = append(raws, r.Metrics.TotalFitness)
	}

	if ind.EvalCount != 5 {
		t.Fatalf("EvalCount = %d after 5 generations, want 5 (Valid individuals must be re-evaluated)", ind.EvalCount)
	}

	sum, maxRaw := 0.0, raws[0]
	for _, v := range raws {
		sum += v
		if v > maxRaw {
			maxRaw = v
		}
	}
	mean := sum / float64(len(raws))

	if math.Abs(ind.Fitness.TotalFitness-mean) > 1e-9 {
		t.Fatalf("TotalFitness = %.6f, want mean of 5 evals %.6f (raws=%v)",
			ind.Fitness.TotalFitness, mean, raws)
	}
	if math.Abs(e.BestFitness-mean) > 1e-9 {
		t.Fatalf("BestFitness = %.6f, want current running mean %.6f (sticky max is the winner's curse)",
			e.BestFitness, mean)
	}
	if maxRaw > mean+1e-9 && e.BestFitness >= maxRaw {
		t.Fatalf("BestFitness = %.6f locked onto max single eval %.6f instead of mean %.6f",
			e.BestFitness, maxRaw, mean)
	}
	if ind.Behavior == (BehaviorDescriptor{}) {
		t.Error("Behavior not computed during evaluation")
	}
}

// TestNoveltySelectNextCarriesEvalState verifies elites carry their
// running-mean bookkeeping (EvalCount/FitnessSum) and Behavior into the next
// generation, while offspring start from zero -- their genome changed, so
// prior evaluations are meaningless.
func TestNoveltySelectNextCarriesEvalState(t *testing.T) {
	e := NewNoveltyEngine(Config{
		PopulationSize: 3,
		EliteSize:      1,
		TournamentSize: 1,
		Workers:        1,
		BaseSeed:       1,
	}, allSeeds())

	gA := seeds.CrazyEights()
	gA.ID = "elite"
	gB := seeds.Whist()
	gB.ID = "mid"
	gC := seeds.GinRummy()
	gC.ID = "low"

	e.Population = []*NoveltyIndividual{
		{Individual: Individual{Genome: gA, Valid: true, EvalCount: 4, FitnessSum: 2.0, MctsSum: 1.1, MctsCount: 2,
			Fitness: fitness.Metrics{TotalFitness: 0.55, SharedFitness: 0.9}},
			Behavior: BehaviorDescriptor{0.3, 0.4}},
		{Individual: Individual{Genome: gB, Valid: true, EvalCount: 2, FitnessSum: 0.8,
			Fitness: fitness.Metrics{TotalFitness: 0.4, SharedFitness: 0.6}}},
		{Individual: Individual{Genome: gC, Valid: true, EvalCount: 1, FitnessSum: 0.3,
			Fitness: fitness.Metrics{TotalFitness: 0.3, SharedFitness: 0.3}}},
	}

	nextGen := e.selectNext()

	elite := nextGen[0]
	if elite.Genome.ID != "elite" {
		t.Fatalf("nextGen[0].Genome.ID = %q, want elite (highest SharedFitness)", elite.Genome.ID)
	}
	if elite.EvalCount != 4 || elite.FitnessSum != 2.0 {
		t.Fatalf("elite must carry running-mean state: EvalCount=%d FitnessSum=%.2f, want 4/2.00",
			elite.EvalCount, elite.FitnessSum)
	}
	if elite.MctsCount != 2 || elite.MctsSum != 1.1 {
		t.Fatalf("elite must carry the MCTS accumulator too: MctsCount=%d MctsSum=%.2f, want 2/1.10",
			elite.MctsCount, elite.MctsSum)
	}
	if elite.Behavior != (BehaviorDescriptor{0.3, 0.4}) {
		t.Fatalf("elite must carry Behavior, got %v", elite.Behavior)
	}

	for i := 1; i < len(nextGen); i++ {
		if nextGen[i].EvalCount != 0 || nextGen[i].FitnessSum != 0 {
			t.Errorf("offspring %d must start with zero eval state, got EvalCount=%d FitnessSum=%.2f",
				i, nextGen[i].EvalCount, nextGen[i].FitnessSum)
		}
		if nextGen[i].MctsCount != 0 || nextGen[i].MctsSum != 0 {
			t.Errorf("offspring %d must start with zero MCTS state, got MctsCount=%d MctsSum=%.2f",
				i, nextGen[i].MctsCount, nextGen[i].MctsSum)
		}
		if nextGen[i].Valid {
			t.Errorf("offspring %d must start invalid (needs evaluation)", i)
		}
	}
}

// TestNoveltyArchiveAbsoluteAdmission pins the new admission semantics
// (audit Task 18): admit iff the behavior is farther than the absolute
// threshold from every same-skeleton archive entry, including entries
// admitted earlier in the same pass. In particular: identical descriptors
// are never double-admitted, persisting elites are NOT re-admitted on later
// generations (the old per-generation-max-normalized threshold re-admitted
// them every generation, audit finding), and sub-floor individuals stay out.
func TestNoveltyArchiveAbsoluteAdmission(t *testing.T) {
	e := NewNoveltyEngine(Config{Workers: 1, BaseSeed: 1}, allSeeds())
	e.Population = []*NoveltyIndividual{
		synthNovelty(genome.Shedding, "a", 0.9, BehaviorDescriptor{0.2, 0.2}),
		synthNovelty(genome.Shedding, "a-dup", 0.9, BehaviorDescriptor{0.2, 0.2}),
		synthNovelty(genome.Shedding, "far", 0.9, BehaviorDescriptor{0.8, 0.8}),
		synthNovelty(genome.Shedding, "sub-floor", FitnessFloor-0.2, BehaviorDescriptor{0.5, 0.9}),
	}

	e.computeNovelty()

	if len(e.Archive) != 2 {
		ids := make([]string, len(e.Archive))
		for i, a := range e.Archive {
			ids[i] = a.Genome.ID
		}
		t.Fatalf("archive = %v (len %d), want exactly [a far]: first of an identical pair admitted, duplicate rejected within the same pass, sub-floor rejected", ids, len(e.Archive))
	}

	// Same population persists (elite case): a second pass must not re-admit
	// descriptors already in the archive.
	e.computeNovelty()
	if len(e.Archive) != 2 {
		t.Fatalf("archive grew to %d on a second pass over the same population; persisting elites must not be re-admitted", len(e.Archive))
	}
}

// TestNoveltyAdaptiveThreshold pins the admission-rate controller: the
// absolute threshold doubles when admissions exceed 5% of the population in
// one pass, halves below 2%, and holds inside the band.
func TestNoveltyAdaptiveThreshold(t *testing.T) {
	mkPop := func(unique int, total int) []*NoveltyIndividual {
		pop := make([]*NoveltyIndividual, 0, total)
		for i := 0; i < total; i++ {
			b := BehaviorDescriptor{0.05, 0.05}
			id := fmt.Sprintf("dup_%d", i)
			if i < unique {
				// spacing 0.09 in x, far above the initial threshold
				b = BehaviorDescriptor{0.05 + float64(i)*0.09, 0.9}
				id = fmt.Sprintf("uniq_%d", i)
			}
			pop = append(pop, synthNovelty(genome.Shedding, id, 0.9, b))
		}
		return pop
	}

	cases := []struct {
		name   string
		unique int // admissions out of 100 (the duplicate block contributes one more when unique==0)
		want   float64
	}{
		{"doubles above 5 percent", 10, NoveltyAddThreshold * 2},
		{"halves below 2 percent", 0, NoveltyAddThreshold / 2}, // only the first duplicate admitted: 1%
		{"holds inside band", 3, NoveltyAddThreshold},          // 3 unique + duplicate cluster rejected near uniq? no: cluster at {0.05,0.05} is far from y=0.9 line -> 4%
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewNoveltyEngine(Config{Workers: 1, BaseSeed: 1}, allSeeds())
			e.Population = mkPop(tc.unique, 100)
			e.computeNovelty()
			if math.Abs(e.addThreshold-tc.want) > 1e-12 {
				t.Fatalf("addThreshold = %v after pass, want %v", e.addThreshold, tc.want)
			}
		})
	}
}

// TestNoveltyArchiveRandomEviction pins uniform-random eviction at the cap
// (audit Task 18): FIFO eviction turned the archive into a sliding window
// that forgot early coverage. With the archive prefilled to cap and 30 new
// admissions, eviction must (a) hold the cap exactly, (b) keep some of the
// entries FIFO would have dropped first, and (c) keep some new entries.
func TestNoveltyArchiveRandomEviction(t *testing.T) {
	e := NewNoveltyEngine(Config{Workers: 1, BaseSeed: 3}, allSeeds())

	for i := 0; i < NoveltyArchiveCap; i++ {
		e.Archive = append(e.Archive,
			synthNovelty(genome.Shedding, fmt.Sprintf("old_%d", i), 0.9, BehaviorDescriptor{0.1, 0.1}))
	}

	pop := make([]*NoveltyIndividual, 0, 30)
	for i := 0; i < 30; i++ {
		// y=0.9 row, adjacent spacing 1/30 > threshold; ~0.8 from the old cluster
		pop = append(pop,
			synthNovelty(genome.Shedding, fmt.Sprintf("new_%d", i), 0.9,
				BehaviorDescriptor{float64(i) / 30.0, 0.9}))
	}
	e.Population = pop

	e.computeNovelty()

	if len(e.Archive) != NoveltyArchiveCap {
		t.Fatalf("archive len = %d after eviction, want exactly %d", len(e.Archive), NoveltyArchiveCap)
	}

	oldHead, newKept := 0, 0
	for _, a := range e.Archive {
		var idx int
		if n, _ := fmt.Sscanf(a.Genome.ID, "old_%d", &idx); n == 1 && idx < 30 {
			oldHead++
		}
		if n, _ := fmt.Sscanf(a.Genome.ID, "new_%d", &idx); n == 1 {
			newKept++
		}
	}
	if oldHead == 0 {
		t.Error("all of old_0..old_29 evicted -- that is FIFO, not uniform-random eviction")
	}
	if newKept == 0 {
		t.Error("all new admissions evicted; uniform-random eviction should keep most of them")
	}
}

// TestNoveltyEngineThreeGenerationSmoke is the end-to-end engine test
// (previously 0% covered): a 3-generation run on real seeds must grow the
// archive, never admit two identical descriptors within a skeleton, track a
// best genome, and leave Generation bumped past the loop so the final
// evaluation used fresh seeds.
func TestNoveltyEngineThreeGenerationSmoke(t *testing.T) {
	cfg := Config{
		PopulationSize: 12,
		Generations:    3,
		EliteSize:      2,
		TournamentSize: 3,
		Workers:        4,
		BaseSeed:       11,
	}
	e := NewNoveltyEngine(cfg, allSeeds())

	var sizes []int
	e.Run(func(gen int, best, avg float64) {
		sizes = append(sizes, len(e.Archive))
	})

	if len(sizes) != cfg.Generations {
		t.Fatalf("progress called %d times, want %d", len(sizes), cfg.Generations)
	}
	if len(e.Archive) == 0 {
		t.Fatal("archive empty after 3 generations; novelty search never archived anything")
	}
	if len(e.Archive) > NoveltyArchiveCap {
		t.Fatalf("archive len %d exceeds cap %d", len(e.Archive), NoveltyArchiveCap)
	}
	for i := 1; i < len(sizes); i++ {
		if sizes[i] < sizes[i-1] {
			t.Fatalf("archive shrank below cap: sizes=%v", sizes)
		}
	}
	if len(e.Archive) < sizes[0] {
		t.Fatalf("final archive (%d) smaller than gen-0 archive (%d)", len(e.Archive), sizes[0])
	}

	// No re-admission of identical descriptors within a skeleton.
	for i := 0; i < len(e.Archive); i++ {
		for j := i + 1; j < len(e.Archive); j++ {
			if e.Archive[i].Genome.Skeleton != e.Archive[j].Genome.Skeleton {
				continue
			}
			if e.Archive[i].Behavior.Distance(e.Archive[j].Behavior) == 0 {
				t.Fatalf("archive entries %d (%s) and %d (%s) share descriptor %v -- identical descriptors must not be re-admitted",
					i, e.Archive[i].Genome.ID, j, e.Archive[j].Genome.ID, e.Archive[i].Behavior)
			}
		}
	}

	if e.BestGenome == nil || e.BestFitness <= 0 {
		t.Errorf("BestFitness/BestGenome not tracked: %v %v", e.BestFitness, e.BestGenome)
	}
	if e.Generation != cfg.Generations {
		t.Errorf("Generation = %d after Run, want %d (final evaluation must use fresh seeds)",
			e.Generation, cfg.Generations)
	}
}
