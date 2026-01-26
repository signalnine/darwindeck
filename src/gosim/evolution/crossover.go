// Package evolution provides genetic algorithm operators for evolving card game genomes.
package evolution

import (
	"math/rand"

	"github.com/signalnine/darwindeck/gosim/evolution/operators"
	"github.com/signalnine/darwindeck/gosim/genome"
)

// CrossoverOperator defines the interface for crossover operations.
type CrossoverOperator interface {
	// Crossover produces offspring from two parent genomes.
	Crossover(parent1, parent2 *genome.GameGenome, rng *rand.Rand) (*genome.GameGenome, *genome.GameGenome)

	// Probability returns the probability of crossover being applied.
	Probability() float64
}

// UniformCrossover implements uniform crossover where each gene is
// randomly selected from one of the two parents.
type UniformCrossover struct {
	probability float64
}

// NewUniformCrossover creates a new uniform crossover operator.
func NewUniformCrossover(probability float64) *UniformCrossover {
	return &UniformCrossover{probability: probability}
}

// Probability returns the crossover probability.
func (c *UniformCrossover) Probability() float64 {
	return c.probability
}

// Crossover produces two offspring by randomly selecting genes from parents.
func (c *UniformCrossover) Crossover(parent1, parent2 *genome.GameGenome, rng *rand.Rand) (*genome.GameGenome, *genome.GameGenome) {
	child1 := operators.CloneGenome(parent1)
	child2 := operators.CloneGenome(parent2)

	// Crossover setup rules
	if rng.Float64() < 0.5 {
		child1.Setup.CardsPerPlayer, child2.Setup.CardsPerPlayer =
			child2.Setup.CardsPerPlayer, child1.Setup.CardsPerPlayer
	}
	if rng.Float64() < 0.5 {
		child1.Setup.DealToTableau, child2.Setup.DealToTableau =
			child2.Setup.DealToTableau, child1.Setup.DealToTableau
	}
	if rng.Float64() < 0.5 {
		child1.Setup.StartingChips, child2.Setup.StartingChips =
			child2.Setup.StartingChips, child1.Setup.StartingChips
	}
	if rng.Float64() < 0.5 {
		child1.Setup.TableauSize, child2.Setup.TableauSize =
			child2.Setup.TableauSize, child1.Setup.TableauSize
	}

	// Crossover turn structure parameters
	if rng.Float64() < 0.5 {
		child1.TurnStructure.MaxTurns, child2.TurnStructure.MaxTurns =
			child2.TurnStructure.MaxTurns, child1.TurnStructure.MaxTurns
	}
	if rng.Float64() < 0.5 {
		child1.TurnStructure.TableauMode, child2.TurnStructure.TableauMode =
			child2.TurnStructure.TableauMode, child1.TurnStructure.TableauMode
	}
	if rng.Float64() < 0.5 {
		child1.TurnStructure.SequenceDirection, child2.TurnStructure.SequenceDirection =
			child2.TurnStructure.SequenceDirection, child1.TurnStructure.SequenceDirection
	}
	if rng.Float64() < 0.5 {
		child1.TurnStructure.IsTrickBased, child2.TurnStructure.IsTrickBased =
			child2.TurnStructure.IsTrickBased, child1.TurnStructure.IsTrickBased
	}

	// Crossover phases - use one-point crossover for phase list
	child1.TurnStructure.Phases, child2.TurnStructure.Phases =
		crossoverPhases(parent1.TurnStructure.Phases, parent2.TurnStructure.Phases, rng)

	// Crossover win conditions - swap entire list
	if rng.Float64() < 0.5 {
		child1.WinConditions, child2.WinConditions =
			child2.WinConditions, child1.WinConditions
	}

	// Crossover effects - swap entire list or mix
	if rng.Float64() < 0.5 {
		child1.Effects, child2.Effects =
			child2.Effects, child1.Effects
	}

	// Crossover card scoring - swap entire list
	if rng.Float64() < 0.5 {
		child1.CardScoring, child2.CardScoring =
			child2.CardScoring, child1.CardScoring
	}

	// Crossover hand evaluation - swap entire struct
	if rng.Float64() < 0.5 {
		child1.HandEval, child2.HandEval =
			child2.HandEval, child1.HandEval
	}

	// Crossover teams - swap entire struct
	if rng.Float64() < 0.5 {
		child1.Teams, child2.Teams =
			child2.Teams, child1.Teams
	}

	// Generate new names for children
	child1.Name = parent1.Name + "-X"
	child2.Name = parent2.Name + "-X"
	child1.Generation = max(parent1.Generation, parent2.Generation) + 1
	child2.Generation = max(parent1.Generation, parent2.Generation) + 1

	return child1, child2
}

// crossoverPhases performs one-point crossover on phase lists.
func crossoverPhases(phases1, phases2 []genome.Phase, rng *rand.Rand) ([]genome.Phase, []genome.Phase) {
	if len(phases1) == 0 && len(phases2) == 0 {
		return nil, nil
	}
	if len(phases1) == 0 {
		return clonePhases(phases2), nil
	}
	if len(phases2) == 0 {
		return nil, clonePhases(phases1)
	}

	// Pick crossover points
	point1 := rng.Intn(len(phases1) + 1)
	point2 := rng.Intn(len(phases2) + 1)

	// Create children: child1 = phases1[:point1] + phases2[point2:]
	// child2 = phases2[:point2] + phases1[point1:]
	child1Phases := make([]genome.Phase, 0, point1+(len(phases2)-point2))
	for i := 0; i < point1; i++ {
		child1Phases = append(child1Phases, cloneSinglePhase(phases1[i]))
	}
	for i := point2; i < len(phases2); i++ {
		child1Phases = append(child1Phases, cloneSinglePhase(phases2[i]))
	}

	child2Phases := make([]genome.Phase, 0, point2+(len(phases1)-point1))
	for i := 0; i < point2; i++ {
		child2Phases = append(child2Phases, cloneSinglePhase(phases2[i]))
	}
	for i := point1; i < len(phases1); i++ {
		child2Phases = append(child2Phases, cloneSinglePhase(phases1[i]))
	}

	// Ensure at least one phase
	if len(child1Phases) == 0 {
		child1Phases = clonePhases(phases1)
	}
	if len(child2Phases) == 0 {
		child2Phases = clonePhases(phases2)
	}

	return child1Phases, child2Phases
}

func clonePhases(phases []genome.Phase) []genome.Phase {
	if phases == nil {
		return nil
	}
	result := make([]genome.Phase, len(phases))
	for i, p := range phases {
		result[i] = cloneSinglePhase(p)
	}
	return result
}

func cloneSinglePhase(p genome.Phase) genome.Phase {
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

// SinglePointCrossover implements single-point crossover on the linear genome representation.
type SinglePointCrossover struct {
	probability float64
}

// NewSinglePointCrossover creates a new single-point crossover operator.
func NewSinglePointCrossover(probability float64) *SinglePointCrossover {
	return &SinglePointCrossover{probability: probability}
}

// Probability returns the crossover probability.
func (c *SinglePointCrossover) Probability() float64 {
	return c.probability
}

// Crossover produces two offspring by selecting a random crossover point.
func (c *SinglePointCrossover) Crossover(parent1, parent2 *genome.GameGenome, rng *rand.Rand) (*genome.GameGenome, *genome.GameGenome) {
	child1 := operators.CloneGenome(parent1)
	child2 := operators.CloneGenome(parent2)

	// Decide which portion to swap based on crossover point
	point := rng.Intn(4) // 0=setup, 1=phases, 2=win conditions, 3=effects

	switch point {
	case 0:
		// Swap entire setup
		child1.Setup, child2.Setup = child2.Setup, child1.Setup
	case 1:
		// Swap phases and turn structure
		child1.TurnStructure, child2.TurnStructure = child2.TurnStructure, child1.TurnStructure
	case 2:
		// Swap win conditions and scoring
		child1.WinConditions, child2.WinConditions = child2.WinConditions, child1.WinConditions
		child1.CardScoring, child2.CardScoring = child2.CardScoring, child1.CardScoring
	case 3:
		// Swap effects and hand evaluation
		child1.Effects, child2.Effects = child2.Effects, child1.Effects
		child1.HandEval, child2.HandEval = child2.HandEval, child1.HandEval
		child1.Teams, child2.Teams = child2.Teams, child1.Teams
	}

	// Generate new names for children
	child1.Name = parent1.Name + "-X"
	child2.Name = parent2.Name + "-X"
	child1.Generation = max(parent1.Generation, parent2.Generation) + 1
	child2.Generation = max(parent1.Generation, parent2.Generation) + 1

	return child1, child2
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
