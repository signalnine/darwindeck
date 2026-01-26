package evolution

import (
	"testing"

	"github.com/signalnine/darwindeck/gosim/genome"
)

func TestNewPopulation(t *testing.T) {
	individuals := make([]*Individual, 5)
	for i := 0; i < 5; i++ {
		individuals[i] = &Individual{
			Genome:    genome.CreateWarGenome(),
			Fitness:   float64(i),
			Evaluated: true,
		}
	}

	pop := NewPopulation(individuals)

	if pop.Size() != 5 {
		t.Errorf("Expected size 5, got %d", pop.Size())
	}
	if pop.Generation != 0 {
		t.Errorf("Expected generation 0, got %d", pop.Generation)
	}
}

func TestPopulationGetBestIndividual(t *testing.T) {
	individuals := []*Individual{
		{Genome: genome.CreateWarGenome(), Fitness: 0.3, Evaluated: true},
		{Genome: genome.CreateWarGenome(), Fitness: 0.9, Evaluated: true},
		{Genome: genome.CreateWarGenome(), Fitness: 0.5, Evaluated: true},
	}

	pop := NewPopulation(individuals)
	best := pop.GetBestIndividual()

	if best.Fitness != 0.9 {
		t.Errorf("Expected best fitness 0.9, got %f", best.Fitness)
	}
}

func TestPopulationGetAverageFitness(t *testing.T) {
	individuals := []*Individual{
		{Genome: genome.CreateWarGenome(), Fitness: 0.2, Evaluated: true},
		{Genome: genome.CreateWarGenome(), Fitness: 0.4, Evaluated: true},
		{Genome: genome.CreateWarGenome(), Fitness: 0.6, Evaluated: true},
	}

	pop := NewPopulation(individuals)
	avg := pop.GetAverageFitness()

	expected := 0.4
	if avg < expected-0.01 || avg > expected+0.01 {
		t.Errorf("Expected average fitness ~%f, got %f", expected, avg)
	}
}

func TestPopulationGetAverageFitnessPartiallyEvaluated(t *testing.T) {
	individuals := []*Individual{
		{Genome: genome.CreateWarGenome(), Fitness: 0.5, Evaluated: true},
		{Genome: genome.CreateWarGenome(), Fitness: 0.0, Evaluated: false}, // Not evaluated
		{Genome: genome.CreateWarGenome(), Fitness: 0.5, Evaluated: true},
	}

	pop := NewPopulation(individuals)
	avg := pop.GetAverageFitness()

	// Should only average evaluated individuals
	expected := 0.5
	if avg < expected-0.01 || avg > expected+0.01 {
		t.Errorf("Expected average fitness %f for evaluated only, got %f", expected, avg)
	}
}

func TestPopulationComputeDiversity(t *testing.T) {
	// Create identical genomes - should have low diversity
	identical := make([]*Individual, 5)
	war := genome.CreateWarGenome()
	for i := 0; i < 5; i++ {
		identical[i] = &Individual{
			Genome:    war.Clone(),
			Fitness:   0.5,
			Evaluated: true,
		}
	}
	identicalPop := NewPopulation(identical)
	identicalDiv := identicalPop.ComputeDiversity()

	// Create diverse genomes - should have higher diversity
	diverse := make([]*Individual, 5)
	genomes := genome.GetSeedGenomes()
	for i := 0; i < 5; i++ {
		diverse[i] = &Individual{
			Genome:    genomes[i%len(genomes)].Clone(),
			Fitness:   0.5,
			Evaluated: true,
		}
	}
	diversePop := NewPopulation(diverse)
	diverseDiv := diversePop.ComputeDiversity()

	// Diverse population should have higher diversity score
	if diverseDiv <= identicalDiv {
		t.Errorf("Expected diverse pop to have higher diversity (%f) than identical (%f)",
			diverseDiv, identicalDiv)
	}
}

func TestPopulationSortByFitness(t *testing.T) {
	individuals := []*Individual{
		{Genome: genome.CreateWarGenome(), Fitness: 0.3},
		{Genome: genome.CreateWarGenome(), Fitness: 0.9},
		{Genome: genome.CreateWarGenome(), Fitness: 0.1},
		{Genome: genome.CreateWarGenome(), Fitness: 0.7},
	}

	pop := NewPopulation(individuals)
	sorted := pop.SortByFitness()

	// Should be descending
	for i := 0; i < len(sorted)-1; i++ {
		if sorted[i].Fitness < sorted[i+1].Fitness {
			t.Errorf("Not sorted at index %d: %f < %f",
				i, sorted[i].Fitness, sorted[i+1].Fitness)
		}
	}
}

func TestPopulationGetUnevaluated(t *testing.T) {
	individuals := []*Individual{
		{Genome: genome.CreateWarGenome(), Fitness: 0.5, Evaluated: true},
		{Genome: genome.CreateWarGenome(), Fitness: 0.0, Evaluated: false},
		{Genome: genome.CreateWarGenome(), Fitness: 0.5, Evaluated: true},
		{Genome: genome.CreateWarGenome(), Fitness: 0.0, Evaluated: false},
	}

	pop := NewPopulation(individuals)
	unevaluated := pop.GetUnevaluated()

	if len(unevaluated) != 2 {
		t.Errorf("Expected 2 unevaluated, got %d", len(unevaluated))
	}
}

func TestPopulationCheckDiversityCrisis(t *testing.T) {
	// Identical genomes should trigger crisis
	identical := make([]*Individual, 10)
	war := genome.CreateWarGenome()
	for i := 0; i < 10; i++ {
		identical[i] = &Individual{Genome: war.Clone()}
	}
	identicalPop := NewPopulation(identical)

	if !identicalPop.CheckDiversityCrisis() {
		t.Error("Expected diversity crisis with identical genomes")
	}
}

func TestIndividualClone(t *testing.T) {
	original := &Individual{
		Genome:    genome.CreateWarGenome(),
		Fitness:   0.75,
		Evaluated: true,
	}

	clone := original.Clone()

	// Should have same values
	if clone.Fitness != original.Fitness {
		t.Errorf("Clone fitness mismatch: %f vs %f", clone.Fitness, original.Fitness)
	}
	if clone.Evaluated != original.Evaluated {
		t.Errorf("Clone evaluated mismatch")
	}

	// Should be independent
	clone.Fitness = 0.5
	if original.Fitness == 0.5 {
		t.Error("Modifying clone affected original")
	}
}

func TestGenomeDistance(t *testing.T) {
	war := genome.CreateWarGenome()
	hearts := genome.CreateHeartsGenome()

	// Same genome should have distance 0
	sameDist := GenomeDistance(war, war)
	if sameDist != 0.0 {
		t.Errorf("Expected distance 0 for same genome, got %f", sameDist)
	}

	// Different genomes should have positive distance
	diffDist := GenomeDistance(war, hearts)
	if diffDist <= 0.0 {
		t.Errorf("Expected positive distance for different genomes, got %f", diffDist)
	}

	// Distance should be bounded by 1.0
	if diffDist > 1.0 {
		t.Errorf("Distance should be <= 1.0, got %f", diffDist)
	}
}

func TestGenomeDistanceSymmetric(t *testing.T) {
	war := genome.CreateWarGenome()
	hearts := genome.CreateHeartsGenome()

	dist1 := GenomeDistance(war, hearts)
	dist2 := GenomeDistance(hearts, war)

	if dist1 != dist2 {
		t.Errorf("Distance not symmetric: %f vs %f", dist1, dist2)
	}
}
