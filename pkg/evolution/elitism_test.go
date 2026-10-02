package evolution

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// Elitism must never lose the best individual. Elites were the top EliteSize
// by SharedFitness alone -- the niche-shared (and, in the hybrid engine,
// novelty-blended) selection score -- so the highest RAW-fitness genome, when
// it sat in a crowded niche, was dropped: measured 2/6 generations on the
// baseline engine and 4/6 on the hybrid at population 60 (2026-10 bughunt).
// That contradicted updateBestFitness's own doc ("Elitism keeps the best
// genome in the population, so the current best is the honest best").

// elitismFixture is a four-individual population whose raw-best member ("raw-
// best", TotalFitness 0.87) ranks THIRD by SharedFitness, i.e. outside the
// two elite slots a shared-fitness-only rule fills.
func elitismFixture() (pop []*Individual, rawBest *genome.Genome) {
	mk := func(g *genome.Genome, id string, raw, shared float64, evals int) *Individual {
		g.ID = id
		return &Individual{
			Genome: g, Valid: true, EvalCount: evals, FitnessSum: raw * float64(evals),
			Fitness: fitness.Metrics{TotalFitness: raw, SharedFitness: shared},
		}
	}
	best := mk(seeds.SimplePoker(), "raw-best", 0.87, 0.30, 3)
	pop = []*Individual{
		mk(seeds.CrazyEights(), "top-shared", 0.50, 0.90, 4),
		mk(seeds.Whist(), "second-shared", 0.45, 0.80, 2),
		best,
		mk(seeds.GinRummy(), "low", 0.40, 0.20, 1),
	}
	return pop, best.Genome
}

func elitismConfig(elite int) Config {
	return Config{PopulationSize: 4, EliteSize: elite, TournamentSize: 1, Workers: 1, BaseSeed: 1}
}

func TestSelectCarriesRawBestAsElite(t *testing.T) {
	engine := NewEngine(elitismConfig(2), allSeeds())
	pop, rawBest := elitismFixture()
	engine.Population = pop

	next := engine.Select()

	if next[0].Genome.ID != "top-shared" {
		t.Errorf("next[0] = %q, want top-shared (the remaining elite slots still fill by shared fitness)", next[0].Genome.ID)
	}
	var carried *Individual
	for _, ind := range next[:2] {
		if ind.Genome == rawBest {
			carried = ind
		}
	}
	if carried == nil {
		ids := []string{next[0].Genome.ID, next[1].Genome.ID}
		t.Fatalf("raw-best individual (0.87) was not carried as an elite; elites = %v", ids)
	}
	if !carried.Valid || carried.EvalCount != 3 || carried.FitnessSum != 0.87*3 {
		t.Errorf("raw-best elite must carry its running-mean state: valid=%v evals=%d sum=%v",
			carried.Valid, carried.EvalCount, carried.FitnessSum)
	}
	if engine.BestGenome != rawBest {
		t.Errorf("BestGenome = %v, want the raw-best genome", engine.BestGenome)
	}
}

func TestNoveltySelectNextCarriesRawBestAsElite(t *testing.T) {
	e := NewNoveltyEngine(elitismConfig(2), allSeeds())
	pop, rawBest := elitismFixture()
	for i, ind := range pop {
		e.Population = append(e.Population, &NoveltyIndividual{
			Individual: *ind,
			Behavior:   BehaviorDescriptor{float64(i+1) / 8, 0.25},
		})
	}

	next := e.selectNext()

	if next[0].Genome.ID != "top-shared" {
		t.Errorf("next[0] = %q, want top-shared", next[0].Genome.ID)
	}
	var carried *NoveltyIndividual
	for _, ind := range next[:2] {
		if ind.Genome == rawBest {
			carried = ind
		}
	}
	if carried == nil {
		ids := []string{next[0].Genome.ID, next[1].Genome.ID}
		t.Fatalf("raw-best individual (0.87) was not carried as an elite; elites = %v", ids)
	}
	if !carried.Valid || carried.EvalCount != 3 {
		t.Errorf("raw-best elite must carry its running-mean state: valid=%v evals=%d", carried.Valid, carried.EvalCount)
	}
	if carried.Behavior != (BehaviorDescriptor{3.0 / 8, 0.25}) {
		t.Errorf("raw-best elite must carry its Behavior, got %v", carried.Behavior)
	}
}

// EliteSize 0 is an explicit "no elitism" request: the raw-best guarantee
// claims one of the EXISTING elite slots, it never adds a slot of its own.
func TestSelectEliteSizeZeroCarriesNothing(t *testing.T) {
	engine := NewEngine(elitismConfig(0), allSeeds())
	pop, _ := elitismFixture()
	engine.Population = pop
	parents := map[*genome.Genome]bool{}
	for _, ind := range pop {
		parents[ind.Genome] = true
	}

	for i, ind := range engine.Select() {
		if parents[ind.Genome] || ind.Valid || ind.EvalCount != 0 {
			t.Errorf("next[%d] (%s) was carried unchanged with EliteSize 0", i, ind.Genome.ID)
		}
	}
}

// A raw-best individual that already ranks inside the elite band must not be
// carried twice (and must not evict another elite).
func TestSelectRawBestAlreadyEliteIsNotDuplicated(t *testing.T) {
	engine := NewEngine(elitismConfig(2), allSeeds())
	pop, rawBest := elitismFixture()
	for _, ind := range pop {
		if ind.Genome == rawBest {
			ind.Fitness.SharedFitness = 0.95 // now the top-shared individual too
		}
	}
	engine.Population = pop

	next := engine.Select()

	if next[0].Genome != rawBest || next[1].Genome.ID != "top-shared" {
		t.Errorf("elites = [%s, %s], want [raw-best, top-shared]", next[0].Genome.ID, next[1].Genome.ID)
	}
}
