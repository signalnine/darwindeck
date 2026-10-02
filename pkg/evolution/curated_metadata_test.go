package evolution

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// Description (a curated one-line pitch for a served game) and VetoStable /
// StableEvals (the publication stability stamp) describe ONE specific
// published genome. Genome.Clone copies them, and neither MutateWith,
// changeSkeleton nor Crossover cleared them, so a `-seed-dir` seed's tagline
// and "5/5 stable" stamp rode along onto every mutated descendant -- games
// with different rules that were never curated or stability-checked. An
// unmodified Clone and a carried elite are the same game and keep them.

func curatedSeed(g *genome.Genome) *genome.Genome {
	g.Description = "Curated pitch for THIS exact game"
	g.VetoStable = true
	g.StableEvals = "5/5"
	return g
}

func assertNoCuratedMetadata(t *testing.T, what string, g *genome.Genome) {
	t.Helper()
	if g.Description != "" || g.VetoStable || g.StableEvals != "" {
		t.Fatalf("%s inherited curated metadata: Description=%q VetoStable=%v StableEvals=%q",
			what, g.Description, g.VetoStable, g.StableEvals)
	}
}

func TestMutationClearsCuratedMetadata(t *testing.T) {
	// Every seed is curated, so a changeSkeleton jump (which copies a seed
	// wholesale) is covered as well as the in-place mutations.
	var pool []*genome.Genome
	for _, s := range seeds.All() {
		pool = append(pool, curatedSeed(s))
	}
	rng := rand.New(rand.NewPCG(1, 1))
	for i := 0; i < 3000; i++ {
		parent := pool[rng.IntN(len(pool))]
		child := MutateWith(parent, rng, pool, true)
		assertNoCuratedMetadata(t, "mutant", child)
		if parent.Description == "" || !parent.VetoStable || parent.StableEvals != "5/5" {
			t.Fatalf("mutation %d cleared the PARENT's curated metadata", i)
		}
	}
}

func TestCrossoverClearsCuratedMetadata(t *testing.T) {
	rng := rand.New(rand.NewPCG(2, 2))
	a, b := curatedSeed(seeds.CrazyEights()), curatedSeed(seeds.MauMau())
	for i := 0; i < 50; i++ {
		assertNoCuratedMetadata(t, "same-skeleton crossover child", CrossoverWith(a, b, rng, true))
	}
	w := curatedSeed(seeds.Whist())
	for i := 0; i < 50; i++ {
		assertNoCuratedMetadata(t, "hybrid crossover child", CrossoverWith(a, w, rng, true))
	}
}

// The same game keeps its metadata: Clone is a faithful copy, and elitism
// carries the genome itself.
func TestCloneAndEliteKeepCuratedMetadata(t *testing.T) {
	g := curatedSeed(seeds.CrazyEights())
	if c := g.Clone(); c.Description != g.Description || !c.VetoStable || c.StableEvals != "5/5" {
		t.Errorf("Clone dropped curated metadata: %q %v %q", c.Description, c.VetoStable, c.StableEvals)
	}

	e := NewEngine(Config{PopulationSize: 3, EliteSize: 1, TournamentSize: 1, Workers: 1, BaseSeed: 1}, allSeeds())
	e.Population = []*Individual{
		{Genome: g, Valid: true, Fitness: fitness.Metrics{TotalFitness: 0.9, SharedFitness: 0.9}},
		{Genome: seeds.Whist(), Valid: true, Fitness: fitness.Metrics{TotalFitness: 0.5, SharedFitness: 0.5}},
		{Genome: seeds.GinRummy(), Valid: true, Fitness: fitness.Metrics{TotalFitness: 0.4, SharedFitness: 0.4}},
	}
	elite := e.Select()[0].Genome
	if elite != g || elite.Description == "" || !elite.VetoStable || elite.StableEvals != "5/5" {
		t.Errorf("carried elite lost its curated metadata: %q %v %q", elite.Description, elite.VetoStable, elite.StableEvals)
	}
}
