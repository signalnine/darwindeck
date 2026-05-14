package evolution

import (
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
