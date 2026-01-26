// Package operators provides genetic mutation operators for evolving card game genomes.
package operators

import (
	"math/rand"

	"github.com/signalnine/darwindeck/gosim/genome"
)

// AddConditionMutation adds a condition to a phase that doesn't have one.
type AddConditionMutation struct {
	BaseMutation
}

// NewAddConditionMutation creates a new add condition mutation.
func NewAddConditionMutation(probability float64) *AddConditionMutation {
	return &AddConditionMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "AddCondition",
		},
	}
}

// Mutate adds a condition to a random phase.
func (m *AddConditionMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	// Find phases that can have conditions (DrawPhase, PlayPhase)
	var eligibleIndices []int
	for i, phase := range clone.TurnStructure.Phases {
		switch p := phase.(type) {
		case *genome.DrawPhase:
			if p.Condition == nil {
				eligibleIndices = append(eligibleIndices, i)
			}
		case *genome.PlayPhase:
			if p.ValidPlayCondition == nil {
				eligibleIndices = append(eligibleIndices, i)
			}
		}
	}

	if len(eligibleIndices) == 0 {
		return clone
	}

	idx := eligibleIndices[rng.Intn(len(eligibleIndices))]
	condition := randomCondition(rng)

	switch p := clone.TurnStructure.Phases[idx].(type) {
	case *genome.DrawPhase:
		newPhase := *p
		newPhase.Condition = condition
		clone.TurnStructure.Phases[idx] = &newPhase
	case *genome.PlayPhase:
		newPhase := *p
		newPhase.ValidPlayCondition = condition
		clone.TurnStructure.Phases[idx] = &newPhase
	}

	return clone
}

// RemoveConditionMutation removes a condition from a phase.
type RemoveConditionMutation struct {
	BaseMutation
}

// NewRemoveConditionMutation creates a new remove condition mutation.
func NewRemoveConditionMutation(probability float64) *RemoveConditionMutation {
	return &RemoveConditionMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "RemoveCondition",
		},
	}
}

// Mutate removes a condition from a random phase.
func (m *RemoveConditionMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	// Find phases with conditions
	var eligibleIndices []int
	for i, phase := range clone.TurnStructure.Phases {
		switch p := phase.(type) {
		case *genome.DrawPhase:
			if p.Condition != nil {
				eligibleIndices = append(eligibleIndices, i)
			}
		case *genome.PlayPhase:
			if p.ValidPlayCondition != nil {
				eligibleIndices = append(eligibleIndices, i)
			}
		}
	}

	if len(eligibleIndices) == 0 {
		return clone
	}

	idx := eligibleIndices[rng.Intn(len(eligibleIndices))]

	switch p := clone.TurnStructure.Phases[idx].(type) {
	case *genome.DrawPhase:
		newPhase := *p
		newPhase.Condition = nil
		clone.TurnStructure.Phases[idx] = &newPhase
	case *genome.PlayPhase:
		newPhase := *p
		newPhase.ValidPlayCondition = nil
		clone.TurnStructure.Phases[idx] = &newPhase
	}

	return clone
}

// ModifyConditionMutation modifies an existing condition.
type ModifyConditionMutation struct {
	BaseMutation
}

// NewModifyConditionMutation creates a new modify condition mutation.
func NewModifyConditionMutation(probability float64) *ModifyConditionMutation {
	return &ModifyConditionMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "ModifyCondition",
		},
	}
}

// Mutate modifies a condition on a random phase.
func (m *ModifyConditionMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	// Find phases with conditions
	var eligibleIndices []int
	for i, phase := range clone.TurnStructure.Phases {
		switch p := phase.(type) {
		case *genome.DrawPhase:
			if p.Condition != nil {
				eligibleIndices = append(eligibleIndices, i)
			}
		case *genome.PlayPhase:
			if p.ValidPlayCondition != nil {
				eligibleIndices = append(eligibleIndices, i)
			}
		}
	}

	if len(eligibleIndices) == 0 {
		return clone
	}

	idx := eligibleIndices[rng.Intn(len(eligibleIndices))]

	switch p := clone.TurnStructure.Phases[idx].(type) {
	case *genome.DrawPhase:
		newPhase := *p
		newPhase.Condition = mutateCondition(newPhase.Condition, rng)
		clone.TurnStructure.Phases[idx] = &newPhase
	case *genome.PlayPhase:
		newPhase := *p
		newPhase.ValidPlayCondition = mutateCondition(newPhase.ValidPlayCondition, rng)
		clone.TurnStructure.Phases[idx] = &newPhase
	}

	return clone
}

// ModifyWinConditionMutation modifies the win conditions.
type ModifyWinConditionMutation struct {
	BaseMutation
}

// NewModifyWinConditionMutation creates a new modify win condition mutation.
func NewModifyWinConditionMutation(probability float64) *ModifyWinConditionMutation {
	return &ModifyWinConditionMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "ModifyWinCondition",
		},
	}
}

// Mutate modifies win conditions.
func (m *ModifyWinConditionMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.WinConditions) == 0 {
		// Add a win condition
		clone.WinConditions = []genome.WinCondition{randomWinCondition(rng)}
		return clone
	}

	// Modify existing win condition
	idx := rng.Intn(len(clone.WinConditions))
	wc := clone.WinConditions[idx]

	switch rng.Intn(2) {
	case 0: // Change type
		wc = randomWinCondition(rng)
	case 1: // Change threshold if applicable
		if wc.Type == genome.WinTypeFirstToScore {
			delta := int32(rng.Intn(21) - 10) // -10 to +10
			wc.Threshold += delta
			if wc.Threshold < 10 {
				wc.Threshold = 10
			}
		}
	}

	clone.WinConditions[idx] = wc
	return clone
}

// AddWinConditionMutation adds a new win condition.
type AddWinConditionMutation struct {
	BaseMutation
	maxConditions int
}

// NewAddWinConditionMutation creates a new add win condition mutation.
func NewAddWinConditionMutation(probability float64) *AddWinConditionMutation {
	return &AddWinConditionMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "AddWinCondition",
		},
		maxConditions: 3,
	}
}

// Mutate adds a new win condition.
func (m *AddWinConditionMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.WinConditions) >= m.maxConditions {
		return clone
	}

	clone.WinConditions = append(clone.WinConditions, randomWinCondition(rng))
	return clone
}

// RemoveWinConditionMutation removes a win condition.
type RemoveWinConditionMutation struct {
	BaseMutation
	minConditions int
}

// NewRemoveWinConditionMutation creates a new remove win condition mutation.
func NewRemoveWinConditionMutation(probability float64) *RemoveWinConditionMutation {
	return &RemoveWinConditionMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "RemoveWinCondition",
		},
		minConditions: 1,
	}
}

// Mutate removes a win condition.
func (m *RemoveWinConditionMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.WinConditions) <= m.minConditions {
		return clone
	}

	idx := rng.Intn(len(clone.WinConditions))
	clone.WinConditions = append(clone.WinConditions[:idx], clone.WinConditions[idx+1:]...)
	return clone
}

// AddCardScoringMutation adds a card scoring rule.
type AddCardScoringMutation struct {
	BaseMutation
	maxRules int
}

// NewAddCardScoringMutation creates a new add card scoring mutation.
func NewAddCardScoringMutation(probability float64) *AddCardScoringMutation {
	return &AddCardScoringMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "AddCardScoring",
		},
		maxRules: 10,
	}
}

// Mutate adds a new card scoring rule.
func (m *AddCardScoringMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.CardScoring) >= m.maxRules {
		return clone
	}

	rule := randomCardScoringRule(rng)
	clone.CardScoring = append(clone.CardScoring, rule)
	return clone
}

// RemoveCardScoringMutation removes a card scoring rule.
type RemoveCardScoringMutation struct {
	BaseMutation
}

// NewRemoveCardScoringMutation creates a new remove card scoring mutation.
func NewRemoveCardScoringMutation(probability float64) *RemoveCardScoringMutation {
	return &RemoveCardScoringMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "RemoveCardScoring",
		},
	}
}

// Mutate removes a card scoring rule.
func (m *RemoveCardScoringMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.CardScoring) == 0 {
		return clone
	}

	idx := rng.Intn(len(clone.CardScoring))
	clone.CardScoring = append(clone.CardScoring[:idx], clone.CardScoring[idx+1:]...)
	return clone
}

// ModifyCardScoringMutation modifies a card scoring rule.
type ModifyCardScoringMutation struct {
	BaseMutation
}

// NewModifyCardScoringMutation creates a new modify card scoring mutation.
func NewModifyCardScoringMutation(probability float64) *ModifyCardScoringMutation {
	return &ModifyCardScoringMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "ModifyCardScoring",
		},
	}
}

// Mutate modifies an existing card scoring rule.
func (m *ModifyCardScoringMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.CardScoring) == 0 {
		return clone
	}

	idx := rng.Intn(len(clone.CardScoring))
	rule := clone.CardScoring[idx]

	switch rng.Intn(4) {
	case 0: // Modify points
		delta := int16(rng.Intn(5) - 2) // -2 to +2
		rule.Points += delta
	case 1: // Modify trigger
		triggers := []genome.ScoringTrigger{
			genome.TriggerTrickWin,
			genome.TriggerCapture,
			genome.TriggerPlay,
			genome.TriggerHandEnd,
		}
		rule.Trigger = triggers[rng.Intn(len(triggers))]
	case 2: // Modify suit
		suits := []uint8{genome.SuitHearts, genome.SuitDiamonds, genome.SuitClubs, genome.SuitSpades, genome.SuitAny}
		rule.Suit = suits[rng.Intn(len(suits))]
	case 3: // Modify rank
		ranks := []uint8{
			genome.RankTwo, genome.RankThree, genome.RankFour, genome.RankFive,
			genome.RankSix, genome.RankSeven, genome.RankEight, genome.RankNine,
			genome.RankTen, genome.RankJack, genome.RankQueen, genome.RankKing,
			genome.RankAce, genome.RankAny,
		}
		rule.Rank = ranks[rng.Intn(len(ranks))]
	}

	clone.CardScoring[idx] = rule
	return clone
}

// AddSpecialEffectMutation adds a special effect (like skip next player).
type AddSpecialEffectMutation struct {
	BaseMutation
	maxEffects int
}

// NewAddSpecialEffectMutation creates a new add special effect mutation.
func NewAddSpecialEffectMutation(probability float64) *AddSpecialEffectMutation {
	return &AddSpecialEffectMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "AddSpecialEffect",
		},
		maxEffects: 8,
	}
}

// Mutate adds a new special effect.
func (m *AddSpecialEffectMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.Effects) >= m.maxEffects {
		return clone
	}

	effect := randomSpecialEffect(rng)
	clone.Effects = append(clone.Effects, effect)
	return clone
}

// RemoveSpecialEffectMutation removes a special effect.
type RemoveSpecialEffectMutation struct {
	BaseMutation
}

// NewRemoveSpecialEffectMutation creates a new remove special effect mutation.
func NewRemoveSpecialEffectMutation(probability float64) *RemoveSpecialEffectMutation {
	return &RemoveSpecialEffectMutation{
		BaseMutation: BaseMutation{
			probability: probability,
			name:        "RemoveSpecialEffect",
		},
	}
}

// Mutate removes a special effect.
func (m *RemoveSpecialEffectMutation) Mutate(g *genome.GameGenome, rng *rand.Rand) *genome.GameGenome {
	clone := CloneGenome(g)

	if len(clone.Effects) == 0 {
		return clone
	}

	idx := rng.Intn(len(clone.Effects))
	clone.Effects = append(clone.Effects[:idx], clone.Effects[idx+1:]...)
	return clone
}

// Helper functions

// Condition OpCode constants (matching engine bytecode)
const (
	OpCheckHandSize     uint8 = 1
	OpCheckLocationSize uint8 = 2
	OpCheckCardRank     uint8 = 3
	OpCheckCardSuit     uint8 = 4
)

// Comparison operator constants
const (
	OpEQ uint8 = 0 // Equal
	OpNE uint8 = 1 // Not equal
	OpLT uint8 = 2 // Less than
	OpLE uint8 = 3 // Less or equal
	OpGT uint8 = 4 // Greater than
	OpGE uint8 = 5 // Greater or equal
)

func randomCondition(rng *rand.Rand) *genome.Condition {
	opCodes := []uint8{
		OpCheckHandSize,
		OpCheckLocationSize,
		OpCheckCardRank,
		OpCheckCardSuit,
	}

	return &genome.Condition{
		OpCode:   opCodes[rng.Intn(len(opCodes))],
		Operator: uint8(rng.Intn(6)), // 0-5 for comparison operators
		Value:    int32(rng.Intn(10) + 1),
		RefLoc:   uint8(rng.Intn(5)), // 0-4 for locations
	}
}

func mutateCondition(c *genome.Condition, rng *rand.Rand) *genome.Condition {
	if c == nil {
		return randomCondition(rng)
	}

	newCond := *c

	switch rng.Intn(4) {
	case 0: // Change opcode
		opCodes := []uint8{
			OpCheckHandSize,
			OpCheckLocationSize,
			OpCheckCardRank,
			OpCheckCardSuit,
		}
		newCond.OpCode = opCodes[rng.Intn(len(opCodes))]
	case 1: // Change operator
		newCond.Operator = uint8(rng.Intn(6))
	case 2: // Change value
		delta := int32(rng.Intn(5) - 2) // -2 to +2
		newCond.Value += delta
		if newCond.Value < 0 {
			newCond.Value = 0
		}
	case 3: // Change reference location
		newCond.RefLoc = uint8(rng.Intn(5))
	}

	return &newCond
}

func randomWinCondition(rng *rand.Rand) genome.WinCondition {
	winTypes := []genome.WinConditionType{
		genome.WinTypeEmptyHand,
		genome.WinTypeCaptureAll,
		genome.WinTypeMostCaptured,
		genome.WinTypeHighScore,
		genome.WinTypeLowScore,
		genome.WinTypeFirstToScore,
	}

	wc := genome.WinCondition{
		Type: winTypes[rng.Intn(len(winTypes))],
	}

	if wc.Type == genome.WinTypeFirstToScore {
		wc.Threshold = int32((rng.Intn(10) + 1) * 10) // 10-100
	}

	return wc
}

func randomCardScoringRule(rng *rand.Rand) genome.CardScoringRule {
	triggers := []genome.ScoringTrigger{
		genome.TriggerTrickWin,
		genome.TriggerCapture,
		genome.TriggerPlay,
		genome.TriggerHandEnd,
	}

	suits := []uint8{genome.SuitHearts, genome.SuitDiamonds, genome.SuitClubs, genome.SuitSpades, genome.SuitAny}
	ranks := []uint8{
		genome.RankTwo, genome.RankThree, genome.RankFour, genome.RankFive,
		genome.RankSix, genome.RankSeven, genome.RankEight, genome.RankNine,
		genome.RankTen, genome.RankJack, genome.RankQueen, genome.RankKing,
		genome.RankAce, genome.RankAny,
	}

	return genome.CardScoringRule{
		Suit:    suits[rng.Intn(len(suits))],
		Rank:    ranks[rng.Intn(len(ranks))],
		Points:  int16(rng.Intn(10) + 1), // 1-10 points
		Trigger: triggers[rng.Intn(len(triggers))],
	}
}

func randomSpecialEffect(rng *rand.Rand) genome.SpecialEffect {
	effects := []genome.EffectType{
		genome.EffectSkipNext,
		genome.EffectReverse,
		genome.EffectDrawTwo,
		genome.EffectDrawFour,
		genome.EffectWild,
	}

	ranks := []uint8{
		genome.RankTwo, genome.RankSeven, genome.RankEight,
		genome.RankJack, genome.RankQueen, genome.RankKing,
	}

	return genome.SpecialEffect{
		TriggerRank: ranks[rng.Intn(len(ranks))],
		Effect:      effects[rng.Intn(len(effects))],
	}
}

// RegisterConditionMutations adds all condition-related mutations to a registry.
func RegisterConditionMutations(r *Registry) {
	r.Register(NewAddConditionMutation(0.05))
	r.Register(NewRemoveConditionMutation(0.05))
	r.Register(NewModifyConditionMutation(0.08))
	r.Register(NewModifyWinConditionMutation(0.10))
	r.Register(NewAddWinConditionMutation(0.03))
	r.Register(NewRemoveWinConditionMutation(0.03))
	r.Register(NewAddCardScoringMutation(0.05))
	r.Register(NewRemoveCardScoringMutation(0.03))
	r.Register(NewModifyCardScoringMutation(0.05))
	r.Register(NewAddSpecialEffectMutation(0.05))
	r.Register(NewRemoveSpecialEffectMutation(0.03))
}
