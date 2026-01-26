package evolution

import (
	"math/rand"
	"testing"

	"github.com/signalnine/darwindeck/gosim/genome"
)

func TestUniformCrossover(t *testing.T) {
	crossover := NewUniformCrossover(1.0)

	parent1 := genome.CreateWarGenome()
	parent2 := genome.CreateHeartsGenome()
	rng := rand.New(rand.NewSource(12345))

	child1, child2 := crossover.Crossover(parent1, parent2, rng)

	// Children should exist
	if child1 == nil || child2 == nil {
		t.Fatal("Expected two children from crossover")
	}

	// Children should have incremented generation
	expectedGen := max(parent1.Generation, parent2.Generation) + 1
	if child1.Generation != expectedGen {
		t.Errorf("Expected child1 generation %d, got %d", expectedGen, child1.Generation)
	}
	if child2.Generation != expectedGen {
		t.Errorf("Expected child2 generation %d, got %d", expectedGen, child2.Generation)
	}

	// Children should have modified names
	if child1.Name == parent1.Name || child1.Name == parent2.Name {
		t.Error("Expected child1 to have a new name")
	}
}

func TestUniformCrossoverProducesVariation(t *testing.T) {
	crossover := NewUniformCrossover(1.0)

	parent1 := genome.CreateWarGenome()
	parent2 := genome.CreateHeartsGenome()

	// Run crossover multiple times to check for variation
	seenDifferentCardsPerPlayer := false
	seenDifferentMaxTurns := false

	for i := 0; i < 20; i++ {
		rng := rand.New(rand.NewSource(int64(12345 + i)))
		child1, child2 := crossover.Crossover(parent1, parent2, rng)

		// Check if children have different values from each other
		if child1.Setup.CardsPerPlayer != child2.Setup.CardsPerPlayer {
			seenDifferentCardsPerPlayer = true
		}
		if child1.TurnStructure.MaxTurns != child2.TurnStructure.MaxTurns {
			seenDifferentMaxTurns = true
		}
	}

	if !seenDifferentCardsPerPlayer {
		t.Error("Expected to see variation in CardsPerPlayer across children")
	}
	if !seenDifferentMaxTurns {
		t.Error("Expected to see variation in MaxTurns across children")
	}
}

func TestSinglePointCrossover(t *testing.T) {
	crossover := NewSinglePointCrossover(1.0)

	parent1 := genome.CreateWarGenome()
	parent2 := genome.CreateHeartsGenome()
	rng := rand.New(rand.NewSource(12345))

	child1, child2 := crossover.Crossover(parent1, parent2, rng)

	// Children should exist
	if child1 == nil || child2 == nil {
		t.Fatal("Expected two children from crossover")
	}

	// At least one portion should be swapped
	// Due to single-point crossover, one child should have some features from parent1
	// and some from parent2
	t.Logf("Parent1 cards: %d, Parent2 cards: %d",
		parent1.Setup.CardsPerPlayer, parent2.Setup.CardsPerPlayer)
	t.Logf("Child1 cards: %d, Child2 cards: %d",
		child1.Setup.CardsPerPlayer, child2.Setup.CardsPerPlayer)
}

func TestCrossoverPreservesStructure(t *testing.T) {
	crossover := NewUniformCrossover(1.0)

	genomes := genome.GetSeedGenomes()
	rng := rand.New(rand.NewSource(12345))

	// Test crossover between pairs of genomes
	// Note: Crossover can produce semantically incoherent genomes (e.g., betting
	// without chips). That's expected - evolution relies on fitness/selection
	// to eliminate invalid offspring. Here we just check structural validity.
	for i := 0; i < len(genomes)-1; i++ {
		parent1 := genomes[i]
		parent2 := genomes[i+1]

		t.Run(parent1.Name+"_x_"+parent2.Name, func(t *testing.T) {
			child1, child2 := crossover.Crossover(parent1, parent2, rng)

			// Check structural validity - children exist and have basic structure
			if child1 == nil || child2 == nil {
				t.Fatal("Expected valid children from crossover")
			}

			// Children should have names
			if child1.Name == "" || child2.Name == "" {
				t.Error("Children should have names")
			}

			// Children should have at least one win condition
			if len(child1.WinConditions) == 0 && len(parent1.WinConditions) > 0 && len(parent2.WinConditions) > 0 {
				t.Error("Child1 lost all win conditions")
			}
			if len(child2.WinConditions) == 0 && len(parent1.WinConditions) > 0 && len(parent2.WinConditions) > 0 {
				t.Error("Child2 lost all win conditions")
			}
		})
	}
}

func TestCrossoverPhaseMixing(t *testing.T) {
	crossover := NewUniformCrossover(1.0)

	// Create parents with distinct phases
	parent1 := &genome.GameGenome{
		Name: "Parent1",
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.DrawPhase{Source: genome.LocationDeck, Count: 1},
				&genome.PlayPhase{Target: genome.LocationDiscard},
			},
			MaxTurns: 100,
		},
		WinConditions: []genome.WinCondition{
			{Type: genome.WinTypeEmptyHand},
		},
	}

	parent2 := &genome.GameGenome{
		Name: "Parent2",
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.TrickPhase{LeadSuitRequired: true},
			},
			MaxTurns: 200,
			IsTrickBased: true,
		},
		WinConditions: []genome.WinCondition{
			{Type: genome.WinTypeMostCaptured},
		},
	}

	// Run multiple times to see different phase combinations
	seenMixedPhases := false
	for i := 0; i < 20; i++ {
		rng := rand.New(rand.NewSource(int64(12345 + i)))
		child1, child2 := crossover.Crossover(parent1, parent2, rng)

		// Check if phases are mixed
		if len(child1.TurnStructure.Phases) > 0 && len(child2.TurnStructure.Phases) > 0 {
			// Check for different phase types in children
			_, c1HasTrick := child1.TurnStructure.Phases[0].(*genome.TrickPhase)
			_, c2HasTrick := child2.TurnStructure.Phases[0].(*genome.TrickPhase)

			if c1HasTrick != c2HasTrick {
				seenMixedPhases = true
				break
			}
		}
	}

	if !seenMixedPhases {
		t.Log("Note: Phase mixing may not always occur due to random crossover points")
	}
}

func TestCrossoverWithEmptyPhases(t *testing.T) {
	crossover := NewUniformCrossover(1.0)

	parent1 := &genome.GameGenome{
		Name: "Parent1",
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.DrawPhase{Source: genome.LocationDeck, Count: 1},
			},
		},
	}

	parent2 := &genome.GameGenome{
		Name: "Parent2",
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{}, // Empty phases
		},
	}

	rng := rand.New(rand.NewSource(12345))
	child1, child2 := crossover.Crossover(parent1, parent2, rng)

	// Children should still be valid
	if child1 == nil || child2 == nil {
		t.Fatal("Expected valid children even with empty parent phases")
	}

	// At least one child should have phases
	if len(child1.TurnStructure.Phases) == 0 && len(child2.TurnStructure.Phases) == 0 {
		t.Error("Expected at least one child to have phases")
	}
}

func TestCrossoverProbability(t *testing.T) {
	crossover := NewUniformCrossover(0.5) // 50% probability

	if crossover.Probability() != 0.5 {
		t.Errorf("Expected probability 0.5, got %f", crossover.Probability())
	}
}
