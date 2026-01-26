// Package operators provides genetic mutation operators for evolving card game genomes.
package operators

import (
	"math/rand"

	"github.com/signalnine/darwindeck/gosim/genome"
)

// MutationOperator is the interface for all mutation operators.
type MutationOperator interface {
	// Mutate applies the mutation to a genome and returns a new mutated genome.
	// The original genome should not be modified.
	Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome

	// Probability returns the probability of this mutation being applied.
	Probability() float64

	// Name returns a human-readable name for this operator.
	Name() string
}

// BaseMutation provides common functionality for mutation operators.
type BaseMutation struct {
	probability float64
	name        string
}

// Probability returns the mutation probability.
func (m *BaseMutation) Probability() float64 {
	return m.probability
}

// Name returns the mutation name.
func (m *BaseMutation) Name() string {
	return m.name
}

// ShouldApply returns true if the mutation should be applied based on probability.
func (m *BaseMutation) ShouldApply(rng *rand.Rand) bool {
	return rng.Float64() < m.probability
}

// CloneGenome creates a deep copy of a genome for mutation.
// This is necessary because Go genomes use slices which share underlying arrays.
func CloneGenome(g *genome.GameGenome) *genome.GameGenome {
	clone := &genome.GameGenome{
		Name:       g.Name,
		Generation: g.Generation,
		Setup: genome.SetupRules{
			CardsPerPlayer: g.Setup.CardsPerPlayer,
			DealToTableau:  g.Setup.DealToTableau,
			StartingChips:  g.Setup.StartingChips,
			TableauSize:    g.Setup.TableauSize,
		},
		TurnStructure: genome.TurnStructure{
			MaxTurns:          g.TurnStructure.MaxTurns,
			TableauMode:       g.TurnStructure.TableauMode,
			SequenceDirection: g.TurnStructure.SequenceDirection,
			IsTrickBased:      g.TurnStructure.IsTrickBased,
		},
	}

	// Clone phases
	if len(g.TurnStructure.Phases) > 0 {
		clone.TurnStructure.Phases = make([]genome.Phase, len(g.TurnStructure.Phases))
		for i, phase := range g.TurnStructure.Phases {
			clone.TurnStructure.Phases[i] = clonePhase(phase)
		}
	}

	// Clone win conditions
	if len(g.WinConditions) > 0 {
		clone.WinConditions = make([]genome.WinCondition, len(g.WinConditions))
		copy(clone.WinConditions, g.WinConditions)
	}

	// Clone effects
	if len(g.Effects) > 0 {
		clone.Effects = make([]genome.SpecialEffect, len(g.Effects))
		copy(clone.Effects, g.Effects)
	}

	// Clone card scoring
	if len(g.CardScoring) > 0 {
		clone.CardScoring = make([]genome.CardScoringRule, len(g.CardScoring))
		copy(clone.CardScoring, g.CardScoring)
	}

	// Clone teams
	if g.Teams != nil {
		clone.Teams = &genome.TeamConfig{
			Enabled: g.Teams.Enabled,
		}
		if len(g.Teams.Teams) > 0 {
			clone.Teams.Teams = make([][]int, len(g.Teams.Teams))
			for i, team := range g.Teams.Teams {
				clone.Teams.Teams[i] = make([]int, len(team))
				copy(clone.Teams.Teams[i], team)
			}
		}
	}

	// Clone hand evaluation
	if g.HandEval != nil {
		clone.HandEval = &genome.HandEvaluation{
			Method:        g.HandEval.Method,
			TargetValue:   g.HandEval.TargetValue,
			BustThreshold: g.HandEval.BustThreshold,
		}
		if len(g.HandEval.CardValues) > 0 {
			clone.HandEval.CardValues = make([]genome.CardValue, len(g.HandEval.CardValues))
			copy(clone.HandEval.CardValues, g.HandEval.CardValues)
		}
		if len(g.HandEval.Patterns) > 0 {
			clone.HandEval.Patterns = make([]genome.HandPattern, len(g.HandEval.Patterns))
			copy(clone.HandEval.Patterns, g.HandEval.Patterns)
		}
	}

	return clone
}

// clonePhase creates a copy of a phase.
func clonePhase(p genome.Phase) genome.Phase {
	switch phase := p.(type) {
	case *genome.DrawPhase:
		clone := *phase
		if phase.Condition != nil {
			condClone := *phase.Condition
			clone.Condition = &condClone
		}
		return &clone
	case *genome.PlayPhase:
		clone := *phase
		if phase.ValidPlayCondition != nil {
			condClone := *phase.ValidPlayCondition
			clone.ValidPlayCondition = &condClone
		}
		return &clone
	case *genome.DiscardPhase:
		clone := *phase
		return &clone
	case *genome.TrickPhase:
		clone := *phase
		return &clone
	case *genome.BettingPhase:
		clone := *phase
		return &clone
	case *genome.BiddingPhase:
		clone := *phase
		return &clone
	case *genome.ClaimPhase:
		clone := *phase
		return &clone
	default:
		return p
	}
}

// Registry holds all available mutation operators.
type Registry struct {
	operators []MutationOperator
}

// NewRegistry creates a new mutation operator registry.
func NewRegistry() *Registry {
	return &Registry{
		operators: make([]MutationOperator, 0),
	}
}

// Register adds a mutation operator to the registry.
func (r *Registry) Register(op MutationOperator) {
	r.operators = append(r.operators, op)
}

// Operators returns all registered operators.
func (r *Registry) Operators() []MutationOperator {
	return r.operators
}

// ApplyAll applies all operators to a genome based on their probabilities.
// Returns the mutated genome.
func (r *Registry) ApplyAll(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	mutated := g
	for _, op := range r.operators {
		if rng.Float64() < op.Probability() {
			mutated = op.Mutate(mutated, rng)
		}
	}
	return mutated
}

// MutationPipeline wraps a Registry and provides a convenient Apply interface.
type MutationPipeline struct {
	registry *Registry
}

// NewMutationPipeline creates a new mutation pipeline from a registry.
func NewMutationPipeline(registry *Registry) *MutationPipeline {
	return &MutationPipeline{registry: registry}
}

// Apply applies the mutation pipeline to a genome in-place.
func (p *MutationPipeline) Apply(g *genome.GameGenome, rng *rand.Rand) {
	mutated := p.registry.ApplyAll(g, rng)
	// Copy the mutated result back to the original genome
	*g = *mutated
}

// NewDefaultPipeline creates a mutation pipeline with default probabilities.
func NewDefaultPipeline(rng *rand.Rand) *MutationPipeline {
	registry := NewRegistry()

	// Setup mutations
	RegisterSetupMutations(registry)

	// Phase mutations
	RegisterPhaseMutations(registry)

	// Condition mutations
	RegisterConditionMutations(registry)

	return NewMutationPipeline(registry)
}

// NewAggressivePipeline creates a mutation pipeline with higher mutation rates.
// Used when diversity drops to inject more variation.
func NewAggressivePipeline(rng *rand.Rand) *MutationPipeline {
	registry := NewRegistry()

	// Setup mutations with higher probabilities
	registry.Register(NewCardsPerPlayerMutation(0.2))
	registry.Register(NewMaxTurnsMutation(0.1))
	registry.Register(NewStartingChipsMutation(0.1))
	registry.Register(NewTableauSizeMutation(0.15))
	registry.Register(NewDealToTableauMutation(0.1))
	registry.Register(NewTableauModeMutation(0.1))
	registry.Register(NewSequenceDirectionMutation(0.1))
	registry.Register(NewTrickBasedMutation(0.1))

	// Phase mutations with higher probabilities
	registry.Register(NewAddDrawPhaseMutation(0.15))
	registry.Register(NewRemovePhaseMutation(0.15))
	registry.Register(NewModifyPlayPhaseMutation(0.15))
	registry.Register(NewAddBettingPhaseMutation(0.1))
	registry.Register(NewModifyBettingPhaseMutation(0.1))
	registry.Register(NewAddTrickPhaseMutation(0.1))
	registry.Register(NewModifyTrickPhaseMutation(0.1))
	registry.Register(NewAddDiscardPhaseMutation(0.1))
	registry.Register(NewSwapPhaseOrderMutation(0.1))

	// Condition mutations with higher probabilities
	registry.Register(NewAddConditionMutation(0.1))
	registry.Register(NewRemoveConditionMutation(0.1))
	registry.Register(NewModifyConditionMutation(0.1))

	return NewMutationPipeline(registry)
}
