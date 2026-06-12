package evolution

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

func allSeeds() []*genome.Genome {
	return []*genome.Genome{
		seeds.CrazyEights(),
		seeds.MauMau(),
		seeds.Whist(),
		seeds.Hearts(),
		seeds.Spades(),
		seeds.OhHell(),
		seeds.GinRummy(),
		seeds.KnockRummy(),
	}
}

func TestMutateProducesValidGenome(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	seedList := allSeeds()

	// Mutate each seed 20 times, check all pass validation
	for _, seed := range seedList {
		for i := 0; i < 20; i++ {
			child := Mutate(seed, rng, seedList)
			errs := genome.Validate(child)
			if len(errs) > 0 {
				t.Errorf("seed %s mutation %d produced invalid genome: %v",
					seed.ID, i, errs)
			}
		}
	}
}

func TestCrossoverSameSkeleton(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 0))

	a := seeds.CrazyEights()
	b := seeds.MauMau()

	child := Crossover(a, b, rng)
	if child == nil {
		t.Fatal("crossover between same-skeleton genomes should succeed")
	}

	if child.Skeleton != genome.Shedding {
		t.Fatalf("child skeleton should be shedding, got %s", child.Skeleton)
	}

	errs := genome.Validate(child)
	if len(errs) > 0 {
		t.Errorf("crossover child invalid: %v", errs)
	}
}

func TestCrossoverDifferentSkeleton(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 0))

	a := seeds.CrazyEights()
	b := seeds.Whist()

	child := Crossover(a, b, rng)
	if child != nil {
		t.Fatal("crossover between different skeletons should return nil")
	}
}

func TestCrossoverDoesNotAliasScoringCardPoints(t *testing.T) {
	a := seeds.Hearts()
	b := seeds.Hearts()
	// Differentiate B's CardPoints so aliasing into B's backing array
	// is detectable (and distinct from A's).
	b.Scoring.CardPoints = []genome.CardScoring{
		{Suit: 2, Points: 5},
		{Rank: 12, Suit: 3, Points: 25},
	}

	aBefore := append([]genome.CardScoring(nil), a.Scoring.CardPoints...)
	bBefore := append([]genome.CardScoring(nil), b.Scoring.CardPoints...)

	// Run crossover across many seeds so both parent-A and parent-B
	// adoption paths for Scoring are exercised. Mutating the child's
	// CardPoints in place must never leak into either parent.
	for i := 0; i < 50; i++ {
		rng := rand.New(rand.NewPCG(uint64(i), 0))
		child := Crossover(a, b, rng)
		if child == nil {
			t.Fatalf("crossover unexpectedly returned nil on iter %d", i)
		}
		for j := range child.Scoring.CardPoints {
			child.Scoring.CardPoints[j].Points = 999
		}
	}

	if !cardPointsEqual(a.Scoring.CardPoints, aBefore) {
		t.Errorf("parent A CardPoints mutated by child: got %+v, want %+v",
			a.Scoring.CardPoints, aBefore)
	}
	if !cardPointsEqual(b.Scoring.CardPoints, bBefore) {
		t.Errorf("parent B CardPoints mutated by child: got %+v, want %+v",
			b.Scoring.CardPoints, bBefore)
	}
}

// TestCrossoverProducesValidGenomes pins dd-kcp: Crossover output must
// pass genome.Validate for every same-skeleton seed pair across many RNG
// seeds. Before the repair step, independent coin flips on TrumpRule and
// Scoring let Spades x Hearts produce TrumpFixed with TrumpSuit=0.
func TestCrossoverProducesValidGenomes(t *testing.T) {
	seedList := allSeeds()

	for _, a := range seedList {
		for _, b := range seedList {
			if a.Skeleton != b.Skeleton {
				continue
			}
			a, b := a, b
			name := a.ID + "_x_" + b.ID
			t.Run(name, func(t *testing.T) {
				for i := 0; i < 200; i++ {
					rng := rand.New(rand.NewPCG(uint64(i), 7))
					child := Crossover(a, b, rng)
					if child == nil {
						t.Fatalf("same-skeleton crossover returned nil (iter %d)", i)
					}
					if errs := genome.Validate(child); len(errs) > 0 {
						t.Errorf("iter %d: Crossover(%s, %s) produced invalid child: %v",
							i, a.ID, b.ID, errs)
					}
				}
			})
		}
	}
}

// TestCrossoverProducesValidGenomesAfterMutation extends the property test
// to mutated parents, which can drift HandSize/Players away from the seed
// values, exposing the HandSize*Players>52 case the issue describes.
func TestCrossoverProducesValidGenomesAfterMutation(t *testing.T) {
	seedList := allSeeds()

	for trial := 0; trial < 500; trial++ {
		rng := rand.New(rand.NewPCG(uint64(trial), 11))
		a := Mutate(seedList[rng.IntN(len(seedList))], rng, seedList)
		b := Mutate(seedList[rng.IntN(len(seedList))], rng, seedList)
		if a.Skeleton != b.Skeleton {
			continue
		}
		child := Crossover(a, b, rng)
		if child == nil {
			t.Fatalf("trial %d: same-skeleton crossover returned nil", trial)
		}
		if errs := genome.Validate(child); len(errs) > 0 {
			t.Errorf("trial %d: Crossover(%s, %s) produced invalid child: %v",
				trial, a.ID, b.ID, errs)
		}
	}
}

func cardPointsEqual(x, y []genome.CardScoring) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func TestCloneGenome(t *testing.T) {
	original := seeds.CrazyEights()
	clone := cloneGenome(original)

	// Modify clone
	clone.Players = 6
	clone.ID = "modified"

	// Original should be unchanged
	if original.Players == 6 {
		t.Fatal("clone modified original players")
	}
	if original.ID == "modified" {
		t.Fatal("clone modified original ID")
	}
}

func TestSmallEvolution(t *testing.T) {
	config := Config{
		PopulationSize: 20,
		Generations:    3,
		EliteSize:      2,
		TournamentSize: 3,
		Workers:        2,
		BaseSeed:       42,
		SaveTopN:       5,
	}

	engine := NewEngine(config, allSeeds())
	engine.Run(func(gen int, best, avg float64) {
		t.Logf("Gen %d: best=%.3f avg=%.3f", gen, best, avg)
	})

	top := engine.TopN(5)
	if len(top) == 0 {
		t.Fatal("no top genomes after evolution")
	}

	for i, ind := range top {
		t.Logf("Rank %d: %s fitness=%.3f skeleton=%s",
			i+1, ind.Genome.ID, ind.Fitness.TotalFitness, ind.Genome.Skeleton)
	}

	if engine.BestFitness <= 0 {
		t.Fatal("best fitness should be > 0 after evolution")
	}
}

// TestRunUpdatesBestFitnessFromFinalEvaluation pins the fix for dd-1xy. With
// Generations=0, Select never runs, so the only place BestFitness/BestGenome
// can be set is after the final EvaluatePopulation. Before the fix, both
// stayed at their zero values regardless of how good the evaluated population
// was.
func TestRunUpdatesBestFitnessFromFinalEvaluation(t *testing.T) {
	config := Config{
		PopulationSize: 10,
		Generations:    0,
		EliteSize:      2,
		TournamentSize: 3,
		Workers:        2,
		BaseSeed:       42,
	}
	engine := NewEngine(config, allSeeds())
	engine.Run(nil)

	hasValid := false
	var maxFit float64
	for _, ind := range engine.Population {
		if ind.Valid {
			hasValid = true
			if ind.Fitness.TotalFitness > maxFit {
				maxFit = ind.Fitness.TotalFitness
			}
		}
	}
	if !hasValid {
		t.Skip("no valid individuals in initial population; cannot exercise tracking")
	}

	if engine.BestGenome == nil {
		t.Fatal("BestGenome=nil after Run; final EvaluatePopulation must update best")
	}
	if engine.BestFitness != maxFit {
		t.Fatalf("BestFitness=%.6f, want %.6f (max valid TotalFitness in final population)",
			engine.BestFitness, maxFit)
	}
}

// engineSeedFor replicates the engine's per-individual seed derivation so
// tests can re-run fitness.Evaluate with the exact seeds the engine used.
func engineSeedFor(cfg Config, gen, idx int) uint64 {
	return cfg.BaseSeed + uint64(gen)*10000 + uint64(idx)
}

// TestEliteRunningMeanFitness pins Task 13.3 (winner's curse): an individual
// kept in the population across 5 generations is re-evaluated each
// generation with a fresh seed, and its reported TotalFitness is the MEAN of
// the 5 evaluations, not the max (and not the first).
func TestEliteRunningMeanFitness(t *testing.T) {
	cfg := Config{
		PopulationSize: 1,
		EliteSize:      1,
		TournamentSize: 1,
		Workers:        1,
		BaseSeed:       42,
	}
	engine := NewEngine(cfg, allSeeds())
	g := seeds.CrazyEights()
	ind := &Individual{Genome: g}
	engine.Population = []*Individual{ind}

	var raws []float64
	for gen := 0; gen < 5; gen++ {
		engine.Generation = gen
		engine.EvaluatePopulation()

		r := fitness.Evaluate(g, engineSeedFor(cfg, gen, 0))
		if !r.Valid {
			t.Fatalf("gen %d: reference evaluation invalid", gen)
		}
		raws = append(raws, r.Metrics.TotalFitness)
	}

	if ind.EvalCount != 5 {
		t.Fatalf("EvalCount = %d after 5 evaluations, want 5", ind.EvalCount)
	}

	sum, max := 0.0, raws[0]
	for _, v := range raws {
		sum += v
		if v > max {
			max = v
		}
	}
	mean := sum / float64(len(raws))

	if math.Abs(ind.Fitness.TotalFitness-mean) > 1e-9 {
		t.Fatalf("TotalFitness = %.6f, want mean of 5 evals %.6f (raws=%v)",
			ind.Fitness.TotalFitness, mean, raws)
	}
	if max > mean+1e-9 && ind.Fitness.TotalFitness >= max {
		t.Fatalf("TotalFitness = %.6f reports max %.6f, not mean %.6f (winner's curse)",
			ind.Fitness.TotalFitness, max, mean)
	}
}

// TestBestFitnessConvergesToMeanNotMax pins Task 13.3 for the engine-level
// stat: with a noisy constant-genome population evaluated across 10
// generations, BestFitness must converge toward the true mean fitness of
// that genome -- within 1 sd of the 10-eval mean -- rather than locking onto
// the max single evaluation as the old sticky-max bookkeeping did.
func TestBestFitnessConvergesToMeanNotMax(t *testing.T) {
	cfg := Config{
		PopulationSize: 1,
		EliteSize:      1,
		TournamentSize: 1,
		Workers:        1,
		BaseSeed:       42, // verified: 10/10 valid CrazyEights evals; sd ~0.02
	}
	engine := NewEngine(cfg, allSeeds())
	g := seeds.CrazyEights()
	engine.Population = []*Individual{{Genome: g}}

	const gens = 10
	var raws []float64
	for gen := 0; gen < gens; gen++ {
		engine.Generation = gen
		engine.EvaluatePopulation()
		engine.updateBestFitness()

		r := fitness.Evaluate(g, engineSeedFor(cfg, gen, 0))
		if !r.Valid {
			// The engine drops the running mean when a re-evaluation comes
			// back invalid (flaky genome); mirror that policy so the
			// reference mean tracks the engine's post-reset window.
			t.Logf("gen %d: evaluation invalid; running mean reset", gen)
			raws = raws[:0]
			continue
		}
		raws = append(raws, r.Metrics.TotalFitness)
	}
	if len(raws) < 5 {
		t.Fatalf("only %d consecutive valid evals at tail of run; pick a BaseSeed with stabler evals", len(raws))
	}

	sum, max := 0.0, raws[0]
	for _, v := range raws {
		sum += v
		if v > max {
			max = v
		}
	}
	mean := sum / float64(len(raws))
	variance := 0.0
	for _, v := range raws {
		variance += (v - mean) * (v - mean)
	}
	sd := math.Sqrt(variance / float64(len(raws)))

	if math.Abs(engine.BestFitness-mean) > sd+1e-9 {
		t.Fatalf("BestFitness = %.6f not within 1 sd (%.6f) of 10-eval mean %.6f (raws=%v)",
			engine.BestFitness, sd, mean, raws)
	}
	if max > mean+1e-9 && engine.BestFitness >= max {
		t.Fatalf("BestFitness = %.6f equals max single eval %.6f instead of converging to mean %.6f",
			engine.BestFitness, max, mean)
	}
	if sd == 0 {
		t.Logf("note: zero eval noise across %d seeds (raws=%v); max assertion vacuous", gens, raws)
	}
}

// TestSelectCarriesEvalStateToElitesOnly verifies elites carry their
// running-mean bookkeeping (EvalCount/FitnessSum) into the next generation,
// while mutated/crossed-over offspring start from zero -- their genome
// changed, so prior evaluations are meaningless.
func TestSelectCarriesEvalStateToElitesOnly(t *testing.T) {
	engine := NewEngine(Config{
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

	engine.Population = []*Individual{
		{Genome: gA, Valid: true, EvalCount: 4, FitnessSum: 2.0,
			Fitness: fitness.Metrics{TotalFitness: 0.5, SharedFitness: 0.9}},
		{Genome: gB, Valid: true, EvalCount: 2, FitnessSum: 0.8,
			Fitness: fitness.Metrics{TotalFitness: 0.4, SharedFitness: 0.6}},
		{Genome: gC, Valid: true, EvalCount: 1, FitnessSum: 0.3,
			Fitness: fitness.Metrics{TotalFitness: 0.3, SharedFitness: 0.3}},
	}

	nextGen := engine.Select()

	elite := nextGen[0]
	if elite.Genome.ID != "elite" {
		t.Fatalf("nextGen[0].Genome.ID = %q, want elite (highest SharedFitness)", elite.Genome.ID)
	}
	if elite.EvalCount != 4 || elite.FitnessSum != 2.0 {
		t.Fatalf("elite must carry running-mean state: EvalCount=%d FitnessSum=%.2f, want 4/2.00",
			elite.EvalCount, elite.FitnessSum)
	}

	for i := 1; i < len(nextGen); i++ {
		if nextGen[i].EvalCount != 0 || nextGen[i].FitnessSum != 0 {
			t.Errorf("offspring %d must start with zero eval state, got EvalCount=%d FitnessSum=%.2f",
				i, nextGen[i].EvalCount, nextGen[i].FitnessSum)
		}
		if nextGen[i].Valid {
			t.Errorf("offspring %d must start invalid (needs evaluation)", i)
		}
	}
}

// TestDedupResetsEvalState verifies that when dedup replaces a duplicate
// genome with a fresh mutation, the running-mean bookkeeping is zeroed: the
// new genome has never been evaluated.
func TestDedupResetsEvalState(t *testing.T) {
	g := seeds.CrazyEights()
	pop := []*Individual{
		{Genome: cloneGenome(g), Valid: true, EvalCount: 3, FitnessSum: 1.5},
		{Genome: cloneGenome(g), Valid: true, EvalCount: 3, FitnessSum: 1.5},
	}

	engine := NewEngine(Config{BaseSeed: 7, Workers: 1}, allSeeds())
	engine.dedup(pop)

	replaced := 0
	for i, ind := range pop {
		if ind.Valid {
			continue
		}
		replaced++
		if ind.EvalCount != 0 || ind.FitnessSum != 0 {
			t.Errorf("replaced individual %d kept stale eval state: EvalCount=%d FitnessSum=%.2f",
				i, ind.EvalCount, ind.FitnessSum)
		}
	}
	if replaced == 0 {
		t.Fatal("dedup did not replace the duplicate; cannot exercise reset")
	}
}

func TestMutateDoesNotCrash(t *testing.T) {
	rng := rand.New(rand.NewPCG(99, 0))
	seedList := allSeeds()

	// Run 1000 mutations to catch edge cases
	for i := 0; i < 1000; i++ {
		seed := seedList[rng.IntN(len(seedList))]
		child := Mutate(seed, rng, seedList)
		if child == nil {
			t.Fatalf("mutation %d returned nil", i)
		}
	}
}

func TestSelectTracksBestRawFitnessAcrossPopulation(t *testing.T) {
	// Population where the genome with highest SharedFitness has lower
	// raw TotalFitness than another individual. Engine.BestFitness must
	// track the highest TotalFitness across the whole population, not
	// just position 0 of the SharedFitness-sorted population.
	engine := NewEngine(Config{
		PopulationSize: 3,
		EliteSize:      1,
		TournamentSize: 1,
		Workers:        1,
		BaseSeed:       1,
	}, allSeeds())

	gHighShared := seeds.CrazyEights()
	gHighShared.ID = "high-shared-low-raw"
	gHighRaw := seeds.MauMau()
	gHighRaw.ID = "low-shared-high-raw"
	gMid := seeds.Hearts()
	gMid.ID = "middle"

	engine.Population = []*Individual{
		{
			Genome:  gHighShared,
			Valid:   true,
			Fitness: fitness.Metrics{TotalFitness: 0.4, SharedFitness: 0.9},
		},
		{
			Genome:  gMid,
			Valid:   true,
			Fitness: fitness.Metrics{TotalFitness: 0.5, SharedFitness: 0.6},
		},
		{
			Genome:  gHighRaw,
			Valid:   true,
			Fitness: fitness.Metrics{TotalFitness: 0.8, SharedFitness: 0.5},
		},
	}

	engine.Select()

	if engine.BestFitness != 0.8 {
		t.Fatalf("BestFitness=%.3f, want 0.8 (highest TotalFitness in population)",
			engine.BestFitness)
	}
	if engine.BestGenome == nil || engine.BestGenome.ID != "low-shared-high-raw" {
		got := "nil"
		if engine.BestGenome != nil {
			got = engine.BestGenome.ID
		}
		t.Fatalf("BestGenome ID=%s, want low-shared-high-raw", got)
	}
}

func TestSelectIgnoresInvalidIndividualsForBestFitness(t *testing.T) {
	// An invalid individual with a fabricated TotalFitness must not be
	// chosen as BestGenome even if its TotalFitness is highest.
	engine := NewEngine(Config{
		PopulationSize: 2,
		EliteSize:      1,
		TournamentSize: 1,
		Workers:        1,
		BaseSeed:       1,
	}, allSeeds())

	gValid := seeds.CrazyEights()
	gValid.ID = "valid-best"
	gInvalid := seeds.MauMau()
	gInvalid.ID = "invalid-but-high"

	engine.Population = []*Individual{
		{
			Genome:  gValid,
			Valid:   true,
			Fitness: fitness.Metrics{TotalFitness: 0.5, SharedFitness: 0.5},
		},
		{
			Genome:  gInvalid,
			Valid:   false,
			Fitness: fitness.Metrics{TotalFitness: 0.99, SharedFitness: 0.99},
		},
	}

	engine.Select()

	if engine.BestGenome == nil || engine.BestGenome.ID != "valid-best" {
		got := "nil"
		if engine.BestGenome != nil {
			got = engine.BestGenome.ID
		}
		t.Fatalf("BestGenome ID=%s, want valid-best (invalid individuals must be ignored)", got)
	}
	if engine.BestFitness != 0.5 {
		t.Fatalf("BestFitness=%.3f, want 0.5", engine.BestFitness)
	}
}

func TestDedupPreservesDiversity(t *testing.T) {
	// Create population with duplicates
	pop := make([]*Individual, 10)
	g := seeds.CrazyEights()
	for i := range pop {
		pop[i] = &Individual{
			Genome: cloneGenome(g),
			Valid:  true,
		}
	}

	// Vary a couple
	pop[1].Genome.Players = 3
	pop[2].Genome.HandSize = 10

	engine := NewEngine(DefaultConfig(), allSeeds())
	engine.dedup(pop)
	// Should not crash, dedup modifies in place
}

func TestDedupReplacesDuplicates(t *testing.T) {
	// Build a population of 10 identical genomes. dedup should leave the
	// first one untouched and replace each later duplicate with a mutated
	// genome so the top-50 contains more than one distinct genomeHash.
	pop := make([]*Individual, 10)
	g := seeds.CrazyEights()
	g.ID = "original-sentinel"
	for i := range pop {
		pop[i] = &Individual{Genome: cloneGenome(g), Valid: true}
	}

	engine := NewEngine(Config{BaseSeed: 7, Workers: 1}, allSeeds())
	engine.dedup(pop)

	hashes := make(map[string]int)
	for _, ind := range pop {
		hashes[genomeHash(ind.Genome)]++
	}
	if len(hashes) < 2 {
		t.Fatalf("dedup left all genomes identical: %d unique hashes in %d individuals",
			len(hashes), len(pop))
	}

	if pop[0].Genome.ID != "original-sentinel" {
		t.Fatalf("dedup modified the first occurrence: ID=%q", pop[0].Genome.ID)
	}

	replaced := 0
	for i := 1; i < len(pop); i++ {
		if !pop[i].Valid {
			replaced++
		}
	}
	if replaced == 0 {
		t.Fatal("dedup did not mark any replaced individuals as Valid=false")
	}
}

// TestGenomeHashDistinguishesBorrowedScoringAndSpecialCards verifies that
// genomeHash incorporates every field mutation/crossover can change. Prior
// versions hashed only len(SpecialCards), and omitted Borrowed and Scoring
// entirely, so legitimately diverse genomes collapsed into one bucket and
// got mutated away by dedup (dd-7v8).
func TestGenomeHashDistinguishesBorrowedScoringAndSpecialCards(t *testing.T) {
	base := func() *genome.Genome {
		return &genome.Genome{
			Skeleton: genome.Shedding,
			Players:  4,
			HandSize: 7,
			Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither},
		}
	}

	t.Run("SpecialCards content (same length, different cards)", func(t *testing.T) {
		a := base()
		a.SpecialCards = []genome.SpecialCard{{Type: genome.SpecialSkip, ByRank: 7}}
		b := base()
		b.SpecialCards = []genome.SpecialCard{{Type: genome.SpecialDrawTwo, ByRank: 2}}

		if genomeHash(a) == genomeHash(b) {
			t.Fatalf("SpecialCards with different contents but equal length must hash differently")
		}
	})

	t.Run("Borrowed mechanics", func(t *testing.T) {
		a := base()
		a.Borrowed = []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechMeldBonus}}
		b := base()
		b.Borrowed = nil

		if genomeHash(a) == genomeHash(b) {
			t.Fatalf("genome with borrowed mechanic must hash differently from genome without")
		}
	})

	t.Run("Scoring CardPoints", func(t *testing.T) {
		a := base()
		a.Scoring = genome.ScoringConfig{CardPoints: []genome.CardScoring{{Rank: uint8(13), Points: 10}}}
		b := base()
		b.Scoring = genome.ScoringConfig{}

		if genomeHash(a) == genomeHash(b) {
			t.Fatalf("genome with CardPoints must hash differently from genome without")
		}
	})

	t.Run("Scoring TrumpSuit", func(t *testing.T) {
		a := base()
		a.Scoring = genome.ScoringConfig{TrumpSuit: 1}
		b := base()
		b.Scoring = genome.ScoringConfig{TrumpSuit: 4}

		if genomeHash(a) == genomeHash(b) {
			t.Fatalf("genome with different TrumpSuit must hash differently")
		}
	})
}

// TestGenomeHashIgnoresIDGenerationFitness confirms genomeHash deliberately
// excludes identity/lineage fields so that two genomes that differ only in
// bookkeeping (and are semantically identical) still collapse during dedup.
func TestGenomeHashIgnoresIDGenerationFitness(t *testing.T) {
	mk := func(id string, gen int, fit float64) *genome.Genome {
		return &genome.Genome{
			ID:         id,
			Generation: gen,
			Fitness:    fit,
			Skeleton:   genome.Shedding,
			Players:    4,
			HandSize:   7,
			Shedding:   &genome.SheddingParams{MatchRule: genome.MatchEither},
		}
	}

	a := mk("alpha", 0, 0.1)
	b := mk("beta", 17, 0.99)

	if genomeHash(a) != genomeHash(b) {
		t.Fatalf("ID/Generation/Fitness must not affect genomeHash:\n  a=%s\n  b=%s",
			genomeHash(a), genomeHash(b))
	}
}

// TestGenomeHashStableUnderSliceReorder ensures semantically-equal genomes
// whose mutable slices have been permuted still produce the same hash. This
// keeps dedup robust against crossover/mutation that reorders Borrowed,
// SpecialCards, or Scoring.CardPoints without changing meaning.
func TestGenomeHashStableUnderSliceReorder(t *testing.T) {
	mk := func() *genome.Genome {
		return &genome.Genome{
			Skeleton: genome.Shedding,
			Players:  4,
			HandSize: 7,
			Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither},
		}
	}

	a := mk()
	a.Borrowed = []genome.BorrowedMechanic{
		{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
		{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
	}
	a.SpecialCards = []genome.SpecialCard{
		{Type: genome.SpecialSkip, ByRank: 7},
		{Type: genome.SpecialReverse, ByRank: 11},
	}
	a.Scoring = genome.ScoringConfig{
		CardPoints: []genome.CardScoring{
			{Rank: 13, Points: 10},
			{Suit: 1, Points: 1},
		},
	}

	b := mk()
	b.Borrowed = []genome.BorrowedMechanic{
		{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
	}
	b.SpecialCards = []genome.SpecialCard{
		{Type: genome.SpecialReverse, ByRank: 11},
		{Type: genome.SpecialSkip, ByRank: 7},
	}
	b.Scoring = genome.ScoringConfig{
		CardPoints: []genome.CardScoring{
			{Suit: 1, Points: 1},
			{Rank: 13, Points: 10},
		},
	}

	if genomeHash(a) != genomeHash(b) {
		t.Fatalf("semantically equal genomes with permuted slices must hash equally:\n  a=%s\n  b=%s",
			genomeHash(a), genomeHash(b))
	}
}
