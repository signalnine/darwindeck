package evolution

import (
	"fmt"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// Genome IDs must be unique within a run. Offspring IDs used to be
// "gen<G>_<rng.IntN(100000)>", so two DIFFERENT genomes born in the same
// generation could share an ID (birthday bound: 12 colliding pairs over 10
// generations at population 500). Downstream code keys on the ID -- the
// evolve command's sortAndTrim deduplicated by it and silently dropped a
// distinct (possibly better) game -- so every engine now derives the ID from
// (birth generation, population slot), which is unique by construction and
// independent of worker count and of checkpoint boundaries.

// idTracker records every (ID -> genome) binding seen during a run and
// reports an ID that names two different genomes. A carried elite keeps both
// its ID and its genome pointer, so re-seeing the same binding is fine.
type idTracker struct {
	t     *testing.T
	owner map[string]*genome.Genome
	dupes int
}

func newIDTracker(t *testing.T) *idTracker {
	return &idTracker{t: t, owner: map[string]*genome.Genome{}}
}

func (tr *idTracker) see(where string, g *genome.Genome) {
	if prev, ok := tr.owner[g.ID]; ok && prev != g {
		tr.dupes++
		if tr.dupes <= 5 {
			tr.t.Errorf("%s: ID %q names two different genomes", where, g.ID)
		}
		return
	}
	tr.owner[g.ID] = g
}

const (
	idTestPopulation  = 500
	idTestGenerations = 10
)

func idTestConfig() Config {
	return Config{
		PopulationSize: idTestPopulation, Generations: idTestGenerations,
		EliteSize: 10, TournamentSize: 5, Workers: 1, BaseSeed: 42, CrossSkeleton: true,
	}
}

// fakeScore stands in for evaluation: selection only needs SOME deterministic
// spread of shared fitness, and skipping the simulator keeps 5000 offspring
// per engine in the millisecond range.
func fakeScore(i int) fitness.Metrics {
	f := float64((i*7919)%1000) / 1000
	return fitness.Metrics{TotalFitness: f, SharedFitness: f}
}

func TestGenomeIDsUniqueWithinRunBaseline(t *testing.T) {
	e := NewEngine(idTestConfig(), seeds.All())
	e.Initialize()
	tr := newIDTracker(t)
	for gen := 0; gen <= idTestGenerations; gen++ {
		e.Generation = gen
		for i, ind := range e.Population {
			tr.see(fmt.Sprintf("baseline gen %d", gen), ind.Genome)
			ind.Valid = true
			ind.Fitness = fakeScore(i + gen)
		}
		e.Population = e.Select()
	}
	if tr.dupes > 0 {
		t.Fatalf("baseline: %d duplicate IDs across %d generations of population %d", tr.dupes, idTestGenerations, idTestPopulation)
	}
}

func TestGenomeIDsUniqueWithinRunHybrid(t *testing.T) {
	e := NewNoveltyEngine(idTestConfig(), seeds.All())
	e.initialize()
	tr := newIDTracker(t)
	for gen := 0; gen <= idTestGenerations; gen++ {
		e.Generation = gen
		for i, ind := range e.Population {
			tr.see(fmt.Sprintf("hybrid gen %d", gen), ind.Genome)
			ind.Valid = true
			ind.Fitness = fakeScore(i + gen)
		}
		e.Population = e.selectNext()
	}
	if tr.dupes > 0 {
		t.Fatalf("hybrid: %d duplicate IDs across %d generations of population %d", tr.dupes, idTestGenerations, idTestPopulation)
	}
}

func TestGenomeIDsUniqueWithinRunMAPElites(t *testing.T) {
	e := NewMAPElitesEngine(idTestConfig(), seeds.All())
	// A same-cell insert re-evaluates the incumbent; keep that off the simulator.
	e.evaluate = func(*genome.Genome, uint64) fitness.EvaluationResult {
		return fitness.EvaluationResult{Valid: true, Metrics: fakeScore(1)}
	}
	// Occupy a few cells per skeleton so breed() takes the archive-parent and
	// same-skeleton-crossover paths, not only the empty-archive fallback.
	for i, s := range seeds.All() {
		for j := 0; j < 3; j++ {
			g := s.Clone()
			g.ID = fmt.Sprintf("occupant_%d_%d", i, j)
			e.insert(g, fakeScore(i*3+j), BehaviorDescriptor{float64(i) / 12, float64(j) / 4})
		}
	}
	tr := newIDTracker(t)
	for gen := 0; gen < idTestGenerations; gen++ {
		for _, g := range e.breed(gen) {
			tr.see(fmt.Sprintf("mapelites gen %d", gen), g)
		}
	}
	if tr.dupes > 0 {
		t.Fatalf("mapelites: %d duplicate IDs across %d generations of population %d", tr.dupes, idTestGenerations, idTestPopulation)
	}
}

// The dedup pass replaces a duplicate's genome with a fresh mutant in place;
// the replacement must get a unique ID too (it used to keep MutateWith's
// random one).
func TestDedupReplacementGetsUniqueID(t *testing.T) {
	e := NewEngine(Config{BaseSeed: 7, Workers: 1}, seeds.All())
	e.Generation = 3
	g := seeds.CrazyEights()
	pop := make([]*Individual, 40)
	for i := range pop {
		c := g.Clone()
		c.ID = fmt.Sprintf("gen4_%d", i)
		pop[i] = &Individual{Genome: c}
	}
	e.dedup(pop)
	seen := map[string]bool{}
	for i, ind := range pop {
		if want := fmt.Sprintf("gen4_%d", i); ind.Genome.ID != want {
			t.Errorf("pop[%d].Genome.ID = %q, want the slot-derived %q", i, ind.Genome.ID, want)
		}
		if seen[ind.Genome.ID] {
			t.Errorf("duplicate ID %q after dedup", ind.Genome.ID)
		}
		seen[ind.Genome.ID] = true
	}
}
