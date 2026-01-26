// Package operators provides genetic mutation operators for evolving card game genomes.
package operators

import (
	"math/rand"

	"github.com/signalnine/darwindeck/gosim/genome"
)

// CardsPerPlayerMutation modifies the number of cards dealt to each player.
type CardsPerPlayerMutation struct {
	BaseMutation
	minCards int
	maxCards int
}

// NewCardsPerPlayerMutation creates a new cards per player mutation.
func NewCardsPerPlayerMutation(probability float64) *CardsPerPlayerMutation {
	return &CardsPerPlayerMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "CardsPerPlayer",
		},
		minCards: 1,
		maxCards: 26, // Half of a standard 52-card deck for 2 players
	}
}

// Mutate adjusts the cards per player within valid bounds.
func (m *CardsPerPlayerMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	// Delta mutation: +/- 1-3 cards
	delta := rng.Intn(5) - 2 // -2 to +2
	if delta == 0 {
		delta = 1 // Ensure some change
	}

	newValue := clone.Setup.CardsPerPlayer + delta

	// Clamp to valid range
	if newValue < m.minCards {
		newValue = m.minCards
	}
	if newValue > m.maxCards {
		newValue = m.maxCards
	}

	clone.Setup.CardsPerPlayer = newValue
	return clone
}

// MaxTurnsMutation modifies the maximum number of turns before a game ends.
type MaxTurnsMutation struct {
	BaseMutation
	minTurns int
	maxTurns int
}

// NewMaxTurnsMutation creates a new max turns mutation.
func NewMaxTurnsMutation(probability float64) *MaxTurnsMutation {
	return &MaxTurnsMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "MaxTurns",
		},
		minTurns: 10,
		maxTurns: 2000,
	}
}

// Mutate adjusts the maximum turns within valid bounds.
func (m *MaxTurnsMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	// Multiplicative mutation for large ranges
	factor := 0.8 + rng.Float64()*0.4 // 0.8x to 1.2x
	newValue := int(float64(clone.TurnStructure.MaxTurns) * factor)

	// Clamp to valid range
	if newValue < m.minTurns {
		newValue = m.minTurns
	}
	if newValue > m.maxTurns {
		newValue = m.maxTurns
	}

	clone.TurnStructure.MaxTurns = newValue
	return clone
}

// StartingChipsMutation modifies the starting chips for betting games.
type StartingChipsMutation struct {
	BaseMutation
	minChips int
	maxChips int
}

// NewStartingChipsMutation creates a new starting chips mutation.
func NewStartingChipsMutation(probability float64) *StartingChipsMutation {
	return &StartingChipsMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "StartingChips",
		},
		minChips: 0,    // 0 = no betting
		maxChips: 5000,
	}
}

// Mutate adjusts the starting chips.
func (m *StartingChipsMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if clone.Setup.StartingChips == 0 {
		// Enable betting with random starting chips
		clone.Setup.StartingChips = (rng.Intn(9) + 1) * 100 // 100-900 in steps of 100
	} else {
		// Multiplicative mutation
		factor := 0.7 + rng.Float64()*0.6 // 0.7x to 1.3x
		newValue := int(float64(clone.Setup.StartingChips) * factor)

		// Clamp to valid range
		if newValue < m.minChips {
			newValue = m.minChips
		}
		if newValue > m.maxChips {
			newValue = m.maxChips
		}

		clone.Setup.StartingChips = newValue
	}

	return clone
}

// TableauSizeMutation modifies the size of the tableau (shared card area).
type TableauSizeMutation struct {
	BaseMutation
	minSize int
	maxSize int
}

// NewTableauSizeMutation creates a new tableau size mutation.
func NewTableauSizeMutation(probability float64) *TableauSizeMutation {
	return &TableauSizeMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "TableauSize",
		},
		minSize: 0,
		maxSize: 10,
	}
}

// Mutate adjusts the tableau size.
func (m *TableauSizeMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	// Delta mutation: +/- 1-2
	delta := rng.Intn(3) - 1 // -1 to +1
	newValue := clone.Setup.TableauSize + delta

	// Clamp to valid range
	if newValue < m.minSize {
		newValue = m.minSize
	}
	if newValue > m.maxSize {
		newValue = m.maxSize
	}

	clone.Setup.TableauSize = newValue
	return clone
}

// DealToTableauMutation toggles whether cards are dealt to the tableau at game start.
type DealToTableauMutation struct {
	BaseMutation
}

// NewDealToTableauMutation creates a new deal to tableau mutation.
func NewDealToTableauMutation(probability float64) *DealToTableauMutation {
	return &DealToTableauMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "DealToTableau",
		},
	}
}

// Mutate adjusts the deal to tableau count.
func (m *DealToTableauMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	// Delta mutation: +/- 1-2 cards
	delta := rng.Intn(3) - 1 // -1 to +1
	newValue := clone.Setup.DealToTableau + delta

	// Clamp to valid range (0 to 10)
	if newValue < 0 {
		newValue = 0
	}
	if newValue > 10 {
		newValue = 10
	}

	clone.Setup.DealToTableau = newValue
	return clone
}

// TableauModeMutation changes the tableau mode (war comparison, sequence building, etc.).
type TableauModeMutation struct {
	BaseMutation
}

// NewTableauModeMutation creates a new tableau mode mutation.
func NewTableauModeMutation(probability float64) *TableauModeMutation {
	return &TableauModeMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "TableauMode",
		},
	}
}

// Mutate changes the tableau mode to a random valid value.
func (m *TableauModeMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	modes := []genome.TableauMode{
		genome.TableauModeNone,
		genome.TableauModeWar,
		genome.TableauModeMatchRank,
		genome.TableauModeSequence,
	}

	// Pick a different mode than current
	currentMode := clone.TurnStructure.TableauMode
	for {
		newMode := modes[rng.Intn(len(modes))]
		if newMode != currentMode || len(modes) == 1 {
			clone.TurnStructure.TableauMode = newMode
			break
		}
	}

	return clone
}

// SequenceDirectionMutation changes the sequence direction for sequence games.
type SequenceDirectionMutation struct {
	BaseMutation
}

// NewSequenceDirectionMutation creates a new sequence direction mutation.
func NewSequenceDirectionMutation(probability float64) *SequenceDirectionMutation {
	return &SequenceDirectionMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "SequenceDirection",
		},
	}
}

// Mutate changes the sequence direction.
func (m *SequenceDirectionMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	directions := []genome.SequenceDirection{
		genome.SequenceAscending,
		genome.SequenceDescending,
		genome.SequenceBoth,
	}

	// Pick a different direction than current
	currentDir := clone.TurnStructure.SequenceDirection
	for {
		newDir := directions[rng.Intn(len(directions))]
		if newDir != currentDir || len(directions) == 1 {
			clone.TurnStructure.SequenceDirection = newDir
			break
		}
	}

	return clone
}

// TrickBasedMutation toggles whether the game uses trick-taking mechanics.
type TrickBasedMutation struct {
	BaseMutation
}

// NewTrickBasedMutation creates a new trick-based mutation.
func NewTrickBasedMutation(probability float64) *TrickBasedMutation {
	return &TrickBasedMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "TrickBased",
		},
	}
}

// Mutate toggles the trick-based setting.
func (m *TrickBasedMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)
	clone.TurnStructure.IsTrickBased = !clone.TurnStructure.IsTrickBased
	return clone
}

// RegisterSetupMutations adds all setup-related mutations to a registry.
func RegisterSetupMutations(r *Registry) {
	r.Register(NewCardsPerPlayerMutation(0.1))
	r.Register(NewMaxTurnsMutation(0.05))
	r.Register(NewStartingChipsMutation(0.05))
	r.Register(NewTableauSizeMutation(0.08))
	r.Register(NewDealToTableauMutation(0.05))
	r.Register(NewTableauModeMutation(0.05))
	r.Register(NewSequenceDirectionMutation(0.05))
	r.Register(NewTrickBasedMutation(0.05))
}
