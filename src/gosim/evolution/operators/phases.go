// Package operators provides genetic mutation operators for evolving card game genomes.
package operators

import (
	"math/rand"

	"github.com/signalnine/darwindeck/gosim/genome"
)

// AddDrawPhaseMutation adds a new draw phase to the turn structure.
type AddDrawPhaseMutation struct {
	BaseMutation
	maxPhases int
}

// NewAddDrawPhaseMutation creates a new add draw phase mutation.
func NewAddDrawPhaseMutation(probability float64) *AddDrawPhaseMutation {
	return &AddDrawPhaseMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "AddDrawPhase",
		},
		maxPhases: 8,
	}
}

// Mutate adds a new draw phase at a random position.
func (m *AddDrawPhaseMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.TurnStructure.Phases) >= m.maxPhases {
		return clone // Too many phases already
	}

	// Create a new draw phase with random parameters
	sources := []genome.Location{
		genome.LocationDeck,
		genome.LocationDiscard,
		genome.LocationTableau,
	}

	newPhase := &genome.DrawPhase{
		Source:    sources[rng.Intn(len(sources))],
		Count:     rng.Intn(3) + 1,    // 1-3 cards
		Mandatory: rng.Float64() < 0.7, // 70% chance mandatory
	}

	// Insert at random position
	pos := rng.Intn(len(clone.TurnStructure.Phases) + 1)
	clone.TurnStructure.Phases = insertPhase(clone.TurnStructure.Phases, pos, newPhase)

	return clone
}

// AddPlayPhaseMutation adds a new play phase to the turn structure.
type AddPlayPhaseMutation struct {
	BaseMutation
	maxPhases int
}

// NewAddPlayPhaseMutation creates a new add play phase mutation.
func NewAddPlayPhaseMutation(probability float64) *AddPlayPhaseMutation {
	return &AddPlayPhaseMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "AddPlayPhase",
		},
		maxPhases: 8,
	}
}

// Mutate adds a new play phase at a random position.
func (m *AddPlayPhaseMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.TurnStructure.Phases) >= m.maxPhases {
		return clone // Too many phases already
	}

	// Create a new play phase with random parameters
	targets := []genome.Location{
		genome.LocationDiscard,
		genome.LocationTableau,
	}

	newPhase := &genome.PlayPhase{
		Target:       targets[rng.Intn(len(targets))],
		MinCards:     1,
		MaxCards:     rng.Intn(3) + 1,   // 1-3 cards
		PassIfUnable: rng.Float64() < 0.5, // 50% chance can pass
	}

	// Insert at random position
	pos := rng.Intn(len(clone.TurnStructure.Phases) + 1)
	clone.TurnStructure.Phases = insertPhase(clone.TurnStructure.Phases, pos, newPhase)

	return clone
}

// AddDiscardPhaseMutation adds a new discard phase to the turn structure.
type AddDiscardPhaseMutation struct {
	BaseMutation
	maxPhases int
}

// NewAddDiscardPhaseMutation creates a new add discard phase mutation.
func NewAddDiscardPhaseMutation(probability float64) *AddDiscardPhaseMutation {
	return &AddDiscardPhaseMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "AddDiscardPhase",
		},
		maxPhases: 8,
	}
}

// Mutate adds a new discard phase at a random position.
func (m *AddDiscardPhaseMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.TurnStructure.Phases) >= m.maxPhases {
		return clone // Too many phases already
	}

	newPhase := &genome.DiscardPhase{
		Target:    genome.LocationDiscard,
		Count:     rng.Intn(3) + 1, // 1-3 cards
		Mandatory: rng.Float64() < 0.7,
	}

	pos := rng.Intn(len(clone.TurnStructure.Phases) + 1)
	clone.TurnStructure.Phases = insertPhase(clone.TurnStructure.Phases, pos, newPhase)

	return clone
}

// AddTrickPhaseMutation adds a new trick-taking phase to the turn structure.
type AddTrickPhaseMutation struct {
	BaseMutation
	maxPhases int
}

// NewAddTrickPhaseMutation creates a new add trick phase mutation.
func NewAddTrickPhaseMutation(probability float64) *AddTrickPhaseMutation {
	return &AddTrickPhaseMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "AddTrickPhase",
		},
		maxPhases: 8,
	}
}

// Mutate adds a new trick phase at a random position.
func (m *AddTrickPhaseMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.TurnStructure.Phases) >= m.maxPhases {
		return clone
	}

	suits := []uint8{
		genome.SuitHearts,
		genome.SuitDiamonds,
		genome.SuitClubs,
		genome.SuitSpades,
	}

	trumpSuit := uint8(255) // No trump by default
	if rng.Float64() < 0.4 {
		trumpSuit = uint8(suits[rng.Intn(len(suits))])
	}

	newPhase := &genome.TrickPhase{
		LeadSuitRequired: rng.Float64() < 0.8,
		TrumpSuit:        trumpSuit,
		HighCardWins:      rng.Float64() < 0.9,
	}

	// Set breaking suit for games like Hearts
	if rng.Float64() < 0.3 {
		newPhase.BreakingSuit = suits[rng.Intn(len(suits))]
	}

	pos := rng.Intn(len(clone.TurnStructure.Phases) + 1)
	clone.TurnStructure.Phases = insertPhase(clone.TurnStructure.Phases, pos, newPhase)

	// Mark as trick-based
	clone.TurnStructure.IsTrickBased = true

	return clone
}

// AddBettingPhaseMutation adds a new betting phase to the turn structure.
type AddBettingPhaseMutation struct {
	BaseMutation
	maxPhases int
}

// NewAddBettingPhaseMutation creates a new add betting phase mutation.
func NewAddBettingPhaseMutation(probability float64) *AddBettingPhaseMutation {
	return &AddBettingPhaseMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "AddBettingPhase",
		},
		maxPhases: 8,
	}
}

// Mutate adds a new betting phase at a random position.
func (m *AddBettingPhaseMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.TurnStructure.Phases) >= m.maxPhases {
		return clone
	}

	// Ensure starting chips exist for betting
	if clone.Setup.StartingChips == 0 {
		clone.Setup.StartingChips = (rng.Intn(9) + 1) * 100 // 100-900
	}

	minBets := []int{5, 10, 20, 25, 50}
	newPhase := &genome.BettingPhase{
		MinBet:    minBets[rng.Intn(len(minBets))],
		MaxRaises: rng.Intn(4) + 1, // 1-4 raises
	}

	pos := rng.Intn(len(clone.TurnStructure.Phases) + 1)
	clone.TurnStructure.Phases = insertPhase(clone.TurnStructure.Phases, pos, newPhase)

	return clone
}

// RemovePhaseMutation removes a phase from the turn structure.
type RemovePhaseMutation struct {
	BaseMutation
	minPhases int
}

// NewRemovePhaseMutation creates a new remove phase mutation.
func NewRemovePhaseMutation(probability float64) *RemovePhaseMutation {
	return &RemovePhaseMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "RemovePhase",
		},
		minPhases: 1, // Must have at least 1 phase
	}
}

// Mutate removes a random phase from the turn structure.
func (m *RemovePhaseMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.TurnStructure.Phases) <= m.minPhases {
		return clone // Can't remove more phases
	}

	// Remove at random position
	pos := rng.Intn(len(clone.TurnStructure.Phases))
	clone.TurnStructure.Phases = removePhase(clone.TurnStructure.Phases, pos)

	return clone
}

// SwapPhaseOrderMutation swaps the order of two phases.
type SwapPhaseOrderMutation struct {
	BaseMutation
}

// NewSwapPhaseOrderMutation creates a new swap phase order mutation.
func NewSwapPhaseOrderMutation(probability float64) *SwapPhaseOrderMutation {
	return &SwapPhaseOrderMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "SwapPhaseOrder",
		},
	}
}

// Mutate swaps the position of two random phases.
func (m *SwapPhaseOrderMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.TurnStructure.Phases) < 2 {
		return clone // Need at least 2 phases to swap
	}

	// Pick two different positions
	pos1 := rng.Intn(len(clone.TurnStructure.Phases))
	pos2 := pos1
	for pos2 == pos1 {
		pos2 = rng.Intn(len(clone.TurnStructure.Phases))
	}

	// Swap
	clone.TurnStructure.Phases[pos1], clone.TurnStructure.Phases[pos2] =
		clone.TurnStructure.Phases[pos2], clone.TurnStructure.Phases[pos1]

	return clone
}

// ModifyDrawPhaseMutation modifies parameters of an existing draw phase.
type ModifyDrawPhaseMutation struct {
	BaseMutation
}

// NewModifyDrawPhaseMutation creates a new modify draw phase mutation.
func NewModifyDrawPhaseMutation(probability float64) *ModifyDrawPhaseMutation {
	return &ModifyDrawPhaseMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "ModifyDrawPhase",
		},
	}
}

// Mutate modifies parameters of a random draw phase.
func (m *ModifyDrawPhaseMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	// Find draw phases
	var drawIndices []int
	for i, phase := range clone.TurnStructure.Phases {
		if _, ok := phase.(*genome.DrawPhase); ok {
			drawIndices = append(drawIndices, i)
		}
	}

	if len(drawIndices) == 0 {
		return clone // No draw phases to modify
	}

	// Pick a random draw phase
	idx := drawIndices[rng.Intn(len(drawIndices))]
	drawPhase := clone.TurnStructure.Phases[idx].(*genome.DrawPhase)

	// Clone the phase before modifying
	newPhase := *drawPhase

	// Randomly modify one parameter
	switch rng.Intn(3) {
	case 0: // Modify source
		sources := []genome.Location{
			genome.LocationDeck,
			genome.LocationDiscard,
			genome.LocationTableau,
		}
		newPhase.Source = sources[rng.Intn(len(sources))]
	case 1: // Modify count
		delta := rng.Intn(3) - 1 // -1 to +1
		newPhase.Count += delta
		if newPhase.Count < 1 {
			newPhase.Count = 1
		}
		if newPhase.Count > 5 {
			newPhase.Count = 5
		}
	case 2: // Toggle mandatory
		newPhase.Mandatory = !newPhase.Mandatory
	}

	clone.TurnStructure.Phases[idx] = &newPhase
	return clone
}

// ModifyPlayPhaseMutation modifies parameters of an existing play phase.
type ModifyPlayPhaseMutation struct {
	BaseMutation
}

// NewModifyPlayPhaseMutation creates a new modify play phase mutation.
func NewModifyPlayPhaseMutation(probability float64) *ModifyPlayPhaseMutation {
	return &ModifyPlayPhaseMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "ModifyPlayPhase",
		},
	}
}

// Mutate modifies parameters of a random play phase.
func (m *ModifyPlayPhaseMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	// Find play phases
	var playIndices []int
	for i, phase := range clone.TurnStructure.Phases {
		if _, ok := phase.(*genome.PlayPhase); ok {
			playIndices = append(playIndices, i)
		}
	}

	if len(playIndices) == 0 {
		return clone // No play phases to modify
	}

	// Pick a random play phase
	idx := playIndices[rng.Intn(len(playIndices))]
	playPhase := clone.TurnStructure.Phases[idx].(*genome.PlayPhase)

	// Clone the phase before modifying
	newPhase := *playPhase

	// Randomly modify one parameter
	switch rng.Intn(4) {
	case 0: // Modify target
		targets := []genome.Location{
			genome.LocationDiscard,
			genome.LocationTableau,
		}
		newPhase.Target = targets[rng.Intn(len(targets))]
	case 1: // Modify min cards
		delta := rng.Intn(3) - 1
		newPhase.MinCards += delta
		if newPhase.MinCards < 0 {
			newPhase.MinCards = 0
		}
		if newPhase.MinCards > newPhase.MaxCards {
			newPhase.MinCards = newPhase.MaxCards
		}
	case 2: // Modify max cards
		delta := rng.Intn(3) - 1
		newPhase.MaxCards += delta
		if newPhase.MaxCards < 1 {
			newPhase.MaxCards = 1
		}
		if newPhase.MaxCards < newPhase.MinCards {
			newPhase.MaxCards = newPhase.MinCards
		}
	case 3: // Toggle pass if unable
		newPhase.PassIfUnable = !newPhase.PassIfUnable
	}

	clone.TurnStructure.Phases[idx] = &newPhase
	return clone
}

// ModifyTrickPhaseMutation modifies parameters of an existing trick phase.
type ModifyTrickPhaseMutation struct {
	BaseMutation
}

// NewModifyTrickPhaseMutation creates a new modify trick phase mutation.
func NewModifyTrickPhaseMutation(probability float64) *ModifyTrickPhaseMutation {
	return &ModifyTrickPhaseMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "ModifyTrickPhase",
		},
	}
}

// Mutate modifies parameters of a random trick phase.
func (m *ModifyTrickPhaseMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	// Find trick phases
	var trickIndices []int
	for i, phase := range clone.TurnStructure.Phases {
		if _, ok := phase.(*genome.TrickPhase); ok {
			trickIndices = append(trickIndices, i)
		}
	}

	if len(trickIndices) == 0 {
		return clone
	}

	idx := trickIndices[rng.Intn(len(trickIndices))]
	trickPhase := clone.TurnStructure.Phases[idx].(*genome.TrickPhase)

	newPhase := *trickPhase

	suits := []uint8{
		genome.SuitHearts,
		genome.SuitDiamonds,
		genome.SuitClubs,
		genome.SuitSpades,
	}

	switch rng.Intn(4) {
	case 0: // Toggle lead suit required
		newPhase.LeadSuitRequired = !newPhase.LeadSuitRequired
	case 1: // Change trump suit
		if rng.Float64() < 0.3 {
			newPhase.TrumpSuit = 255 // No trump
		} else {
			newPhase.TrumpSuit = uint8(suits[rng.Intn(len(suits))])
		}
	case 2: // Toggle highest wins
		newPhase.HighCardWins = !newPhase.HighCardWins
	case 3: // Change breaking suit
		if rng.Float64() < 0.5 {
			newPhase.BreakingSuit = 0 // No breaking suit
		} else {
			newPhase.BreakingSuit = suits[rng.Intn(len(suits))]
		}
	}

	clone.TurnStructure.Phases[idx] = &newPhase
	return clone
}

// ModifyBettingPhaseMutation modifies parameters of an existing betting phase.
type ModifyBettingPhaseMutation struct {
	BaseMutation
}

// NewModifyBettingPhaseMutation creates a new modify betting phase mutation.
func NewModifyBettingPhaseMutation(probability float64) *ModifyBettingPhaseMutation {
	return &ModifyBettingPhaseMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "ModifyBettingPhase",
		},
	}
}

// Mutate modifies parameters of a random betting phase.
func (m *ModifyBettingPhaseMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	// Find betting phases
	var bettingIndices []int
	for i, phase := range clone.TurnStructure.Phases {
		if _, ok := phase.(*genome.BettingPhase); ok {
			bettingIndices = append(bettingIndices, i)
		}
	}

	if len(bettingIndices) == 0 {
		return clone
	}

	idx := bettingIndices[rng.Intn(len(bettingIndices))]
	bettingPhase := clone.TurnStructure.Phases[idx].(*genome.BettingPhase)

	newPhase := *bettingPhase

	switch rng.Intn(2) {
	case 0: // Modify min bet
		minBets := []int{5, 10, 20, 25, 50, 100}
		newPhase.MinBet = minBets[rng.Intn(len(minBets))]
	case 1: // Modify max raises
		newPhase.MaxRaises += rng.Intn(3) - 1 // -1 to +1
		if newPhase.MaxRaises < 1 {
			newPhase.MaxRaises = 1
		}
		if newPhase.MaxRaises > 5 {
			newPhase.MaxRaises = 5
		}
	}

	clone.TurnStructure.Phases[idx] = &newPhase
	return clone
}

// Helper functions

func insertPhase(phases []genome.Phase, pos int, phase genome.Phase) []genome.Phase {
	result := make([]genome.Phase, len(phases)+1)
	copy(result[:pos], phases[:pos])
	result[pos] = phase
	copy(result[pos+1:], phases[pos:])
	return result
}

func removePhase(phases []genome.Phase, pos int) []genome.Phase {
	result := make([]genome.Phase, len(phases)-1)
	copy(result[:pos], phases[:pos])
	copy(result[pos:], phases[pos+1:])
	return result
}

// RegisterPhaseMutations adds all phase-related mutations to a registry.
func RegisterPhaseMutations(r *Registry) {
	r.Register(NewAddDrawPhaseMutation(0.08))
	r.Register(NewAddPlayPhaseMutation(0.08))
	r.Register(NewAddDiscardPhaseMutation(0.05))
	r.Register(NewAddTrickPhaseMutation(0.05))
	r.Register(NewAddBettingPhaseMutation(0.03))
	r.Register(NewRemovePhaseMutation(0.08))
	r.Register(NewSwapPhaseOrderMutation(0.05))
	r.Register(NewModifyDrawPhaseMutation(0.10))
	r.Register(NewModifyPlayPhaseMutation(0.10))
	r.Register(NewModifyTrickPhaseMutation(0.05))
	r.Register(NewModifyBettingPhaseMutation(0.03))
}
