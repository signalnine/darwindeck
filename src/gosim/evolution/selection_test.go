package evolution

import (
	"math/rand"
	"testing"

	"github.com/signalnine/darwindeck/gosim/genome"
)

func createTestPopulation(n int) *Population {
	individuals := make([]*Individual, n)
	for i := 0; i < n; i++ {
		individuals[i] = &Individual{
			Genome:    genome.CreateWarGenome(),
			Fitness:   float64(i) / float64(n-1), // 0.0 to 1.0
			Evaluated: true,
		}
	}
	return NewPopulation(individuals)
}

func TestTournamentSelectionBasic(t *testing.T) {
	pop := createTestPopulation(10)
	rng := rand.New(rand.NewSource(42))

	selected := TournamentSelection(pop, 3, rng)

	if selected == nil {
		t.Fatal("TournamentSelection returned nil")
	}

	// Verify selected is from population
	found := false
	for _, ind := range pop.Individuals {
		if ind == selected {
			found = true
			break
		}
	}
	if !found {
		t.Error("Selected individual not in population")
	}
}

func TestTournamentSelectionEmpty(t *testing.T) {
	pop := NewPopulation([]*Individual{})
	rng := rand.New(rand.NewSource(42))

	selected := TournamentSelection(pop, 3, rng)

	if selected != nil {
		t.Error("Expected nil for empty population")
	}
}

func TestTournamentSelectionNil(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	selected := TournamentSelection(nil, 3, rng)

	if selected != nil {
		t.Error("Expected nil for nil population")
	}
}

func TestTournamentSelectionLargeTournament(t *testing.T) {
	pop := createTestPopulation(5)
	rng := rand.New(rand.NewSource(42))

	// Tournament size larger than population
	selected := TournamentSelection(pop, 10, rng)

	if selected == nil {
		t.Fatal("TournamentSelection returned nil")
	}

	// Should always select the best when tournament >= population
	if selected.Fitness < 0.9 {
		t.Logf("With full tournament, expected best (0.9+), got %f", selected.Fitness)
	}
}

func TestSelectEliteBasic(t *testing.T) {
	pop := createTestPopulation(10)

	elite := SelectElite(pop, 3)

	if len(elite) != 3 {
		t.Fatalf("Expected 3 elite, got %d", len(elite))
	}

	// Should be top 3 by fitness
	for i, ind := range elite {
		expectedMin := 1.0 - float64(i+1)/9.0
		if ind.Fitness < expectedMin-0.1 {
			t.Errorf("Elite %d fitness too low: %f", i, ind.Fitness)
		}
	}
}

func TestSelectEliteMoreThanPopulation(t *testing.T) {
	pop := createTestPopulation(5)

	elite := SelectElite(pop, 10)

	// Should return entire population
	if len(elite) != 5 {
		t.Errorf("Expected 5 elite (full pop), got %d", len(elite))
	}
}

func TestSelectEliteByRate(t *testing.T) {
	pop := createTestPopulation(20)

	elite := SelectEliteByRate(pop, 0.2) // 20% = 4

	if len(elite) != 4 {
		t.Errorf("Expected 4 elite (20%%), got %d", len(elite))
	}
}

func TestSelectEliteByRateSmall(t *testing.T) {
	pop := createTestPopulation(10)

	// Very small rate should still select at least 1
	elite := SelectEliteByRate(pop, 0.05)

	if len(elite) < 1 {
		t.Error("Should select at least 1 individual")
	}
}

func TestRouletteWheelSelection(t *testing.T) {
	pop := createTestPopulation(10)
	rng := rand.New(rand.NewSource(42))

	selected := RouletteWheelSelection(pop, rng)

	if selected == nil {
		t.Fatal("RouletteWheelSelection returned nil")
	}

	// Verify in population
	found := false
	for _, ind := range pop.Individuals {
		if ind == selected {
			found = true
			break
		}
	}
	if !found {
		t.Error("Selected individual not in population")
	}
}

func TestRouletteWheelSelectionBias(t *testing.T) {
	// Create population with one very high fitness
	individuals := make([]*Individual, 10)
	for i := 0; i < 10; i++ {
		individuals[i] = &Individual{
			Genome:    genome.CreateWarGenome(),
			Fitness:   0.1,
			Evaluated: true,
		}
	}
	individuals[0].Fitness = 10.0 // Much higher than others

	pop := NewPopulation(individuals)
	rng := rand.New(rand.NewSource(42))

	// Count how often the high-fitness individual is selected
	highCount := 0
	trials := 100
	for i := 0; i < trials; i++ {
		selected := RouletteWheelSelection(pop, rng)
		if selected.Fitness == 10.0 {
			highCount++
		}
	}

	// High-fitness individual should be selected significantly more often
	// It has ~92% of total fitness, so should be selected ~92% of time
	if highCount < 70 {
		t.Errorf("High-fitness individual selected only %d/%d times (expected ~90%%)", highCount, trials)
	}
}

func TestRankSelection(t *testing.T) {
	pop := createTestPopulation(10)
	rng := rand.New(rand.NewSource(42))

	selected := RankSelection(pop, rng)

	if selected == nil {
		t.Fatal("RankSelection returned nil")
	}
}

func TestTruncationSelection(t *testing.T) {
	pop := createTestPopulation(20)

	truncated := TruncationSelection(pop, 0.3) // Top 30%

	expected := 6 // 30% of 20
	if len(truncated) != expected {
		t.Errorf("Expected %d individuals, got %d", expected, len(truncated))
	}

	// Should be sorted by fitness
	for i := 0; i < len(truncated)-1; i++ {
		if truncated[i].Fitness < truncated[i+1].Fitness {
			t.Errorf("Not sorted at index %d", i)
		}
	}
}

func TestSelectDiverse(t *testing.T) {
	// Create population from diverse seed genomes
	seeds := genome.GetSeedGenomes()
	individuals := make([]*Individual, len(seeds))
	for i, g := range seeds {
		individuals[i] = &Individual{
			Genome:    g.Clone(),
			Fitness:   float64(len(seeds)-i) / float64(len(seeds)), // Higher fitness for earlier genomes
			Evaluated: true,
		}
	}
	pop := NewPopulation(individuals)

	diverse := SelectDiverse(pop, 5)

	if len(diverse) != 5 {
		t.Fatalf("Expected 5 diverse individuals, got %d", len(diverse))
	}

	// First should be the best
	if diverse[0].Fitness != 1.0 {
		t.Errorf("First diverse individual should be best, got fitness %f", diverse[0].Fitness)
	}
}

func TestSelectDiverseMoreThanPopulation(t *testing.T) {
	pop := createTestPopulation(3)

	diverse := SelectDiverse(pop, 10)

	// Should return entire population
	if len(diverse) != 3 {
		t.Errorf("Expected 3 (full pop), got %d", len(diverse))
	}
}

func TestSelectDiverseEmpty(t *testing.T) {
	pop := NewPopulation([]*Individual{})

	diverse := SelectDiverse(pop, 5)

	if diverse != nil && len(diverse) != 0 {
		t.Error("Expected empty/nil result for empty population")
	}
}
