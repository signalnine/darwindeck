package operators

import (
	"math/rand"
	"testing"

	"github.com/signalnine/darwindeck/gosim/genome"
)

func TestCloneGenome(t *testing.T) {
	original := genome.CreateWarGenome()

	clone := CloneGenome(original)

	// Verify names match
	if clone.Name != original.Name {
		t.Errorf("Expected name %s, got %s", original.Name, clone.Name)
	}

	// Verify deep copy - modify clone shouldn't affect original
	clone.Setup.CardsPerPlayer = 100
	if original.Setup.CardsPerPlayer == 100 {
		t.Error("Clone is not a deep copy - modifying clone affected original")
	}
}

func TestCloneGenomeWithPhases(t *testing.T) {
	original := genome.CreateHeartsGenome()

	clone := CloneGenome(original)

	// Verify phases were cloned
	if len(clone.TurnStructure.Phases) != len(original.TurnStructure.Phases) {
		t.Errorf("Expected %d phases, got %d",
			len(original.TurnStructure.Phases), len(clone.TurnStructure.Phases))
	}

	// Verify trick phase was cloned correctly
	if len(clone.TurnStructure.Phases) > 0 {
		trickPhase, ok := clone.TurnStructure.Phases[0].(*genome.TrickPhase)
		if !ok {
			t.Error("Expected TrickPhase")
		}
		if trickPhase.TrumpSuit != 255 {
			t.Errorf("Expected no trump (255), got %d", trickPhase.TrumpSuit)
		}
	}
}

func TestRegistryApplyAll(t *testing.T) {
	registry := NewRegistry()
	RegisterSetupMutations(registry)

	original := genome.CreateWarGenome()
	rng := rand.New(rand.NewSource(12345))

	// Apply multiple times - at least one mutation should occur
	mutationOccurred := false
	for i := 0; i < 100; i++ {
		mutated := registry.ApplyAll(original, rng)
		if mutated.Setup.CardsPerPlayer != original.Setup.CardsPerPlayer ||
			mutated.TurnStructure.MaxTurns != original.TurnStructure.MaxTurns ||
			mutated.Setup.TableauSize != original.Setup.TableauSize {
			mutationOccurred = true
			break
		}
	}

	if !mutationOccurred {
		t.Error("Expected at least one mutation to occur over 100 applications")
	}
}

func TestCardsPerPlayerMutation(t *testing.T) {
	mutation := NewCardsPerPlayerMutation(1.0) // 100% probability

	original := genome.CreateWarGenome()
	rng := rand.New(rand.NewSource(12345))

	mutated := mutation.Mutate(original, rng)

	// Should have changed
	if mutated.Setup.CardsPerPlayer == original.Setup.CardsPerPlayer {
		// Try again with different seed - rare but possible to get same value
		rng = rand.New(rand.NewSource(54321))
		mutated = mutation.Mutate(original, rng)
	}

	// Value should be in valid range
	if mutated.Setup.CardsPerPlayer < 1 || mutated.Setup.CardsPerPlayer > 26 {
		t.Errorf("Cards per player out of range: %d", mutated.Setup.CardsPerPlayer)
	}
}

func TestMaxTurnsMutation(t *testing.T) {
	mutation := NewMaxTurnsMutation(1.0)

	original := genome.CreateWarGenome()
	rng := rand.New(rand.NewSource(12345))

	mutated := mutation.Mutate(original, rng)

	// Value should be in valid range
	if mutated.TurnStructure.MaxTurns < 10 || mutated.TurnStructure.MaxTurns > 2000 {
		t.Errorf("Max turns out of range: %d", mutated.TurnStructure.MaxTurns)
	}
}

func TestStartingChipsMutation(t *testing.T) {
	mutation := NewStartingChipsMutation(1.0)

	// Test with no chips (should enable betting)
	original := genome.CreateWarGenome()
	rng := rand.New(rand.NewSource(12345))

	mutated := mutation.Mutate(original, rng)

	if mutated.Setup.StartingChips == 0 {
		t.Error("Expected starting chips to be set when starting from 0")
	}
}

func TestDealToTableauMutation(t *testing.T) {
	mutation := NewDealToTableauMutation(1.0)

	original := genome.CreateWarGenome()
	rng := rand.New(rand.NewSource(12345))

	originalValue := original.Setup.DealToTableau
	mutated := mutation.Mutate(original, rng)

	if mutated.Setup.DealToTableau == originalValue {
		t.Error("Expected DealToTableau to be toggled")
	}
}

func TestAddDrawPhaseMutation(t *testing.T) {
	mutation := NewAddDrawPhaseMutation(1.0)

	original := genome.CreateWarGenome()
	originalPhaseCount := len(original.TurnStructure.Phases)
	rng := rand.New(rand.NewSource(12345))

	mutated := mutation.Mutate(original, rng)

	if len(mutated.TurnStructure.Phases) != originalPhaseCount+1 {
		t.Errorf("Expected %d phases, got %d",
			originalPhaseCount+1, len(mutated.TurnStructure.Phases))
	}
}

func TestAddPlayPhaseMutation(t *testing.T) {
	mutation := NewAddPlayPhaseMutation(1.0)

	original := genome.CreateWarGenome()
	originalPhaseCount := len(original.TurnStructure.Phases)
	rng := rand.New(rand.NewSource(12345))

	mutated := mutation.Mutate(original, rng)

	if len(mutated.TurnStructure.Phases) != originalPhaseCount+1 {
		t.Errorf("Expected %d phases, got %d",
			originalPhaseCount+1, len(mutated.TurnStructure.Phases))
	}
}

func TestRemovePhaseMutation(t *testing.T) {
	mutation := NewRemovePhaseMutation(1.0)

	// Create genome with multiple phases
	original := genome.CreateHeartsGenome()
	// Add an extra phase so we can remove one
	original.TurnStructure.Phases = append(original.TurnStructure.Phases,
		&genome.DrawPhase{Source: genome.LocationDeck, Count: 1})
	originalPhaseCount := len(original.TurnStructure.Phases)
	rng := rand.New(rand.NewSource(12345))

	mutated := mutation.Mutate(original, rng)

	if len(mutated.TurnStructure.Phases) != originalPhaseCount-1 {
		t.Errorf("Expected %d phases, got %d",
			originalPhaseCount-1, len(mutated.TurnStructure.Phases))
	}
}

func TestRemovePhaseMutationMinPhases(t *testing.T) {
	mutation := NewRemovePhaseMutation(1.0)

	// Create genome with only 1 phase
	original := genome.CreateWarGenome()
	// Ensure only 1 phase
	original.TurnStructure.Phases = original.TurnStructure.Phases[:1]
	rng := rand.New(rand.NewSource(12345))

	mutated := mutation.Mutate(original, rng)

	// Should not remove the last phase
	if len(mutated.TurnStructure.Phases) != 1 {
		t.Errorf("Expected 1 phase (minimum), got %d", len(mutated.TurnStructure.Phases))
	}
}

func TestSwapPhaseOrderMutation(t *testing.T) {
	mutation := NewSwapPhaseOrderMutation(1.0)

	// Create genome with multiple phases
	original := &genome.GameGenome{
		Name: "Test",
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.DrawPhase{Source: genome.LocationDeck, Count: 1},
				&genome.PlayPhase{Target: genome.LocationDiscard},
				&genome.DrawPhase{Source: genome.LocationDiscard, Count: 2},
			},
		},
	}
	rng := rand.New(rand.NewSource(12345))

	// Try multiple times until we get a swap
	swapped := false
	for i := 0; i < 10; i++ {
		mutated := mutation.Mutate(original, rng)

		// Check if any phases changed position
		for j, phase := range mutated.TurnStructure.Phases {
			origPhase := original.TurnStructure.Phases[j]
			if drawPhase, ok := phase.(*genome.DrawPhase); ok {
				if origDraw, okOrig := origPhase.(*genome.DrawPhase); okOrig {
					if drawPhase.Count != origDraw.Count || drawPhase.Source != origDraw.Source {
						swapped = true
						break
					}
				} else {
					swapped = true
					break
				}
			}
		}
		if swapped {
			break
		}
	}

	if !swapped {
		t.Error("Expected at least one phase swap over 10 attempts")
	}
}

func TestModifyDrawPhaseMutation(t *testing.T) {
	mutation := NewModifyDrawPhaseMutation(1.0)

	original := &genome.GameGenome{
		Name: "Test",
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.DrawPhase{Source: genome.LocationDeck, Count: 2, Mandatory: true},
			},
		},
	}
	rng := rand.New(rand.NewSource(12345))

	// Apply multiple times to ensure modification happens
	modified := false
	for i := 0; i < 10; i++ {
		mutated := mutation.Mutate(original, rng)
		drawPhase := mutated.TurnStructure.Phases[0].(*genome.DrawPhase)
		origPhase := original.TurnStructure.Phases[0].(*genome.DrawPhase)

		if drawPhase.Source != origPhase.Source ||
			drawPhase.Count != origPhase.Count ||
			drawPhase.Mandatory != origPhase.Mandatory {
			modified = true
			break
		}
	}

	if !modified {
		t.Error("Expected draw phase to be modified")
	}
}

func TestModifyWinConditionMutation(t *testing.T) {
	mutation := NewModifyWinConditionMutation(1.0)

	original := genome.CreateWarGenome()
	rng := rand.New(rand.NewSource(12345))

	// Try multiple times
	modified := false
	for i := 0; i < 10; i++ {
		mutated := mutation.Mutate(original, rng)
		if len(mutated.WinConditions) > 0 && len(original.WinConditions) > 0 {
			if mutated.WinConditions[0].Type != original.WinConditions[0].Type ||
				mutated.WinConditions[0].Threshold != original.WinConditions[0].Threshold {
				modified = true
				break
			}
		}
	}

	if !modified {
		t.Error("Expected win condition to be modified")
	}
}

func TestAddConditionMutation(t *testing.T) {
	mutation := NewAddConditionMutation(1.0)

	original := &genome.GameGenome{
		Name: "Test",
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.DrawPhase{Source: genome.LocationDeck, Count: 1},
			},
		},
	}
	rng := rand.New(rand.NewSource(12345))

	mutated := mutation.Mutate(original, rng)

	// Check if condition was added
	drawPhase := mutated.TurnStructure.Phases[0].(*genome.DrawPhase)
	if drawPhase.Condition == nil {
		t.Error("Expected condition to be added to draw phase")
	}
}

func TestAddCardScoringMutation(t *testing.T) {
	mutation := NewAddCardScoringMutation(1.0)

	original := genome.CreateWarGenome()
	rng := rand.New(rand.NewSource(12345))

	mutated := mutation.Mutate(original, rng)

	if len(mutated.CardScoring) != len(original.CardScoring)+1 {
		t.Errorf("Expected %d card scoring rules, got %d",
			len(original.CardScoring)+1, len(mutated.CardScoring))
	}
}

func TestAddSpecialEffectMutation(t *testing.T) {
	mutation := NewAddSpecialEffectMutation(1.0)

	original := genome.CreateWarGenome()
	rng := rand.New(rand.NewSource(12345))

	mutated := mutation.Mutate(original, rng)

	if len(mutated.Effects) != len(original.Effects)+1 {
		t.Errorf("Expected %d effects, got %d",
			len(original.Effects)+1, len(mutated.Effects))
	}
}

func TestMutationPreservesStructure(t *testing.T) {
	// Create a registry with all mutations
	registry := NewRegistry()
	RegisterSetupMutations(registry)
	RegisterPhaseMutations(registry)
	RegisterConditionMutations(registry)

	genomes := genome.GetSeedGenomes()
	rng := rand.New(rand.NewSource(12345))

	// Note: Mutations can produce semantically incoherent genomes (e.g.,
	// mutating tableau_mode on a capture-based game). That's expected -
	// evolution relies on fitness evaluation to penalize invalid offspring.
	// Here we just check that mutations don't corrupt basic structure.
	for _, g := range genomes {
		t.Run(g.Name, func(t *testing.T) {
			// Apply mutations multiple times
			mutated := g
			for i := 0; i < 10; i++ {
				mutated = registry.ApplyAll(mutated, rng)
			}

			// Check structural validity - genome isn't corrupted
			if mutated == nil {
				t.Fatal("Mutation returned nil")
			}

			// Name should still exist
			if mutated.Name == "" {
				t.Error("Mutation cleared genome name")
			}

			// Setup values should be in valid ranges
			if mutated.Setup.CardsPerPlayer < 0 {
				t.Error("CardsPerPlayer became negative")
			}
			if mutated.TurnStructure.MaxTurns < 0 {
				t.Error("MaxTurns became negative")
			}

			// Phases should exist (at least an empty slice, not nil)
			if mutated.TurnStructure.Phases == nil {
				t.Error("Phases became nil")
			}
		})
	}
}

func TestAllMutationsRegistered(t *testing.T) {
	registry := NewRegistry()
	RegisterSetupMutations(registry)
	RegisterPhaseMutations(registry)
	RegisterConditionMutations(registry)

	operators := registry.Operators()

	// Should have a reasonable number of mutations registered
	if len(operators) < 20 {
		t.Errorf("Expected at least 20 mutations registered, got %d", len(operators))
	}

	// Log all registered mutations
	t.Logf("Registered %d mutations:", len(operators))
	for _, op := range operators {
		t.Logf("  - %s (p=%.2f)", op.Name(), op.Probability())
	}
}
