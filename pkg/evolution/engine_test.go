package evolution

import (
	"math/rand/v2"
	"testing"

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
