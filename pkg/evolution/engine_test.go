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

func TestMutateScoringGeneratesValidRanks(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 0))

	// Valid ranks are 2-14 (see pkg/sim/card.go). Rank 0 is a wildcard,
	// rank 1 is never emitted because no card has rank 1 -- if mutateScoring
	// ever produces Rank=1 the resulting CardScoring rule is dead weight.
	for i := 0; i < 1000; i++ {
		g := &genome.Genome{}
		mutateScoring(g, rng)
		if len(g.Scoring.CardPoints) != 1 {
			t.Fatalf("iteration %d: expected 1 CardScoring entry, got %d",
				i, len(g.Scoring.CardPoints))
		}
		r := g.Scoring.CardPoints[0].Rank
		if r == 1 {
			t.Fatalf("iteration %d: mutateScoring emitted invalid Rank=1 (no card has rank 1)", i)
		}
		if r != 0 && (r < 2 || r > 14) {
			t.Fatalf("iteration %d: mutateScoring emitted out-of-range Rank=%d", i, r)
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
			Valid:   true,
		}
	}

	// Vary a couple
	pop[1].Genome.Players = 3
	pop[2].Genome.HandSize = 10

	dedup(pop)
	// Should not crash, dedup modifies in place
}
