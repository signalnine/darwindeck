package fitness

import (
	"math"

	"github.com/signalnine/darwindeck/gosim/genome"
)

// ComplexityBreakdown provides detailed breakdown of cognitive complexity sources.
type ComplexityBreakdown struct {
	// Core mechanics
	PhaseExplanationCost float64 // Cost of explaining each phase type
	ConditionComplexity  float64 // Nesting depth, conjunctions
	SpecialEffectsCost   float64 // Unique rules to memorize

	// State tracking (invisible but costly)
	MemoryRequirements float64 // Cards to track, hidden info
	StateTrackingCost  float64 // Trump suit, who passed, etc.

	// Familiarity discounts
	FamiliarPatternDiscount float64 // Trick-taking, draw-play, etc.

	// Final score
	TotalComplexity        float64 // 0.0 = trivial, 1.0 = very complex
	ExplanationSentences   int     // Estimated sentences to explain
}

// InvertedScore returns 1.0 - complexity for fitness (simpler = better).
func (c *ComplexityBreakdown) InvertedScore() float64 {
	return math.Max(0.0, 1.0-c.TotalComplexity)
}

// CalculateComplexity computes cognitive complexity of a game's rules.
func CalculateComplexity(g *genome.GameGenome) *ComplexityBreakdown {
	// 1. Phase explanation cost
	phaseCost := calculatePhaseCost(g)

	// 2. Condition complexity
	conditionCost := calculateConditionComplexity(g)

	// 3. Special effects cost
	effectsCost := calculateEffectsCost(g)

	// 4. Memory requirements
	memoryCost := calculateMemoryCost(g)

	// 5. State tracking cost
	stateCost := calculateStateTrackingCost(g)

	// 6. Implicit complexity from game type
	implicitCost := calculateImplicitComplexity(g)

	// 7. Familiar pattern discounts
	discount := calculateFamiliarityDiscount(g)

	// Normalize components
	conditionCostNorm := math.Min(1.0, conditionCost/0.40)
	effectsCostNorm := math.Min(1.0, effectsCost/0.15)
	stateCostNorm := math.Min(1.0, stateCost/0.40)

	// Combine with weights
	rawComplexity := phaseCost*0.22 +
		conditionCostNorm*0.20 +
		effectsCostNorm*0.15 +
		memoryCost*0.18 +
		stateCostNorm*0.10 +
		implicitCost*0.15

	// Apply familiarity discount (multiplicative, capped at 40%)
	discountFactor := math.Min(0.40, discount*0.50)
	total := rawComplexity * (1.0 - discountFactor)

	// Power transform to spread out scores
	total = math.Pow(total, 0.6)
	total = math.Min(1.0, total)

	// Estimate explanation sentences
	sentences := estimateExplanationSentences(g)

	return &ComplexityBreakdown{
		PhaseExplanationCost:    phaseCost,
		ConditionComplexity:     conditionCost,
		SpecialEffectsCost:      effectsCost,
		MemoryRequirements:      memoryCost,
		StateTrackingCost:       stateCost,
		FamiliarPatternDiscount: discount,
		TotalComplexity:         total,
		ExplanationSentences:    sentences,
	}
}

// ComputeRulesComplexity returns the inverted complexity score for fitness.
// Returns 0.0-1.0 where 1.0 = simplest, 0.0 = most complex.
func ComputeRulesComplexity(g *genome.GameGenome) float64 {
	breakdown := CalculateComplexity(g)
	return breakdown.InvertedScore()
}

func calculatePhaseCost(g *genome.GameGenome) float64 {
	// Phase type costs (in "explanation units")
	phaseCosts := map[uint8]float64{
		genome.PhaseTypeDraw:    0.08, // "Draw a card"
		genome.PhaseTypePlay:    0.15, // May have conditions
		genome.PhaseTypeDiscard: 0.10, // Simple
		genome.PhaseTypeTrick:   0.45, // Lead, follow suit, trump, highest wins, scoring
		genome.PhaseTypeBetting: 0.50, // Check, bet, call, raise, fold, all-in, pot
		genome.PhaseTypeClaim:   0.55, // Claim, lie option, challenge, truth check
		genome.PhaseTypeBidding: 0.40, // Contract bidding
	}

	cost := 0.0
	distinctTypes := make(map[uint8]bool)

	for _, p := range g.TurnStructure.Phases {
		phaseType := p.PhaseType()
		distinctTypes[phaseType] = true
		baseCost := phaseCosts[phaseType]
		if baseCost == 0 {
			baseCost = 0.10
		}

		// Additional complexity for phase parameters
		switch phase := p.(type) {
		case *genome.DrawPhase:
			if phase.Source == genome.LocationOpponentHand {
				baseCost += 0.15
			}
			if !phase.Mandatory {
				baseCost += 0.05
			}
			if phase.Condition != nil {
				baseCost += 0.12
			}

		case *genome.PlayPhase:
			if phase.ValidPlayCondition != nil {
				baseCost += 0.15
			}

		case *genome.DiscardPhase:
			if phase.Count > 1 {
				baseCost += 0.10
			}
		}

		cost += baseCost
	}

	// Discount duplicate phases
	numPhases := len(g.TurnStructure.Phases)
	numDistinct := len(distinctTypes)
	numDuplicates := numPhases - numDistinct

	if numDuplicates > 0 {
		duplicateDiscount := float64(numDuplicates) * 0.10
		cost = math.Max(0.1, cost-duplicateDiscount)
	}

	// Bonus for many distinct phase types
	distinctBonus := float64(numDistinct) * 0.06
	cost += distinctBonus

	return math.Min(1.0, cost)
}

func calculateConditionComplexity(g *genome.GameGenome) float64 {
	totalClauses := 0
	conditionCount := 0

	for _, p := range g.TurnStructure.Phases {
		switch phase := p.(type) {
		case *genome.DrawPhase:
			if phase.Condition != nil {
				conditionCount++
				totalClauses++
			}
		case *genome.PlayPhase:
			if phase.ValidPlayCondition != nil {
				conditionCount++
				totalClauses++
			}
		}
	}

	// Count special effects as implicit conditions
	totalClauses += len(g.Effects)

	if conditionCount == 0 && len(g.Effects) == 0 {
		return 0.0
	}

	// Presence score
	presenceScore := math.Min(0.4, 0.15+float64(conditionCount)*0.08)

	// Clause score
	clauseScore := math.Min(1.0, float64(totalClauses)/8.0)

	return presenceScore*0.50 + clauseScore*0.50
}

func calculateEffectsCost(g *genome.GameGenome) float64 {
	if len(g.Effects) == 0 {
		return 0.0
	}

	// Group by effect type
	effectTypes := make(map[genome.EffectType]bool)
	for _, effect := range g.Effects {
		effectTypes[effect.Effect] = true
	}

	uniqueTypes := len(effectTypes)
	totalEffects := len(g.Effects)

	// Base cost per unique effect type
	typeCost := float64(uniqueTypes) * 0.15

	// Additional cost for many triggers
	var exceptionCost float64
	if totalEffects > uniqueTypes {
		exceptionCost = float64(totalEffects-uniqueTypes) * 0.05
	}

	return math.Min(1.0, typeCost+exceptionCost)
}

func calculateMemoryCost(g *genome.GameGenome) float64 {
	cost := 0.0

	// Check win conditions for memory-heavy types
	for _, wc := range g.WinConditions {
		switch wc.Type {
		case genome.WinTypeMostCaptured:
			cost += 0.20
		case genome.WinTypeLowScore:
			cost += 0.15
		case genome.WinTypeBestHand:
			cost += 0.35 // Poker hand rankings
		}
	}

	// Check for memory-heavy phase types
	for _, p := range g.TurnStructure.Phases {
		switch p.(type) {
		case *genome.TrickPhase:
			cost += 0.30 // Card counting
		case *genome.ClaimPhase:
			cost += 0.25 // Track claims and opponent behavior
		case *genome.BettingPhase:
			cost += 0.15 // Pot math, position
		case *genome.DiscardPhase:
			phase := p.(*genome.DiscardPhase)
			if phase.Count > 1 {
				cost += 0.15 // Pair/set matching
			}
		}
	}

	// Hidden information baseline
	cost += 0.08

	return math.Min(1.0, cost)
}

func calculateStateTrackingCost(g *genome.GameGenome) float64 {
	cost := 0.0

	for _, p := range g.TurnStructure.Phases {
		switch p.(type) {
		case *genome.TrickPhase:
			cost += 0.15 // Trump suit, lead suit
		case *genome.BettingPhase:
			cost += 0.20 // Pot, current bet, who's in
		}
	}

	// Special effects that change game state
	for _, effect := range g.Effects {
		switch effect.Effect {
		case genome.EffectReverse:
			cost += 0.10
		case genome.EffectSkipNext:
			cost += 0.05
		}
	}

	return math.Min(1.0, cost)
}

func calculateImplicitComplexity(g *genome.GameGenome) float64 {
	cost := 0.0

	for _, wc := range g.WinConditions {
		switch wc.Type {
		case genome.WinTypeBestHand:
			cost += 0.50 // Poker hand rankings
		case genome.WinTypeLowScore:
			cost += 0.20 // Point counting
		case genome.WinTypeMostCaptured:
			cost += 0.15 // Capture rules
		}
	}

	// Check for flexible play (suggests meld/set formation)
	for _, p := range g.TurnStructure.Phases {
		if phase, ok := p.(*genome.PlayPhase); ok {
			if phase.Target == genome.LocationTableau && phase.MaxCards > 1 {
				cost += 0.25 // Meld/run formation
				break
			}
		}
	}

	// Card scoring rules add complexity
	cost += float64(len(g.CardScoring)) * 0.10

	return math.Min(1.0, cost)
}

func calculateFamiliarityDiscount(g *genome.GameGenome) float64 {
	discount := 0.0

	// Check phase types
	hasTrick := false
	hasDraw := false
	hasPlay := false
	hasBetting := false

	for _, p := range g.TurnStructure.Phases {
		switch p.(type) {
		case *genome.TrickPhase:
			hasTrick = true
		case *genome.DrawPhase:
			hasDraw = true
		case *genome.PlayPhase:
			hasPlay = true
		case *genome.BettingPhase:
			hasBetting = true
		}
	}

	// Trick-taking is familiar (Hearts, Spades, Bridge)
	if hasTrick {
		discount += 0.15
	}

	// Simple draw-play pattern (Crazy Eights, Uno)
	if hasDraw && hasPlay && len(g.TurnStructure.Phases) <= 3 {
		discount += 0.10
	}

	// Betting is familiar (Poker)
	if hasBetting {
		discount += 0.08
	}

	// War is trivial
	if len(g.TurnStructure.Phases) == 1 {
		if _, ok := g.TurnStructure.Phases[0].(*genome.PlayPhase); ok {
			discount += 0.25
		}
	}

	return math.Min(1.0, discount)
}

func estimateExplanationSentences(g *genome.GameGenome) int {
	sentences := 2 // Setup

	for _, p := range g.TurnStructure.Phases {
		switch p.(type) {
		case *genome.DrawPhase:
			sentences += 1
		case *genome.PlayPhase:
			sentences += 2
			if phase, ok := p.(*genome.PlayPhase); ok && phase.ValidPlayCondition != nil {
				sentences += 1
			}
		case *genome.DiscardPhase:
			sentences += 1
		case *genome.TrickPhase:
			sentences += 5 // Lead, follow, trump, resolution, scoring
		case *genome.BettingPhase:
			sentences += 4 // Check, bet, raise, fold
		case *genome.ClaimPhase:
			sentences += 3 // Claim, challenge, resolution
		case *genome.BiddingPhase:
			sentences += 3 // Bidding rules
		default:
			sentences += 1
		}
	}

	// Special effects
	if len(g.Effects) > 0 {
		effectTypes := make(map[genome.EffectType]bool)
		for _, e := range g.Effects {
			effectTypes[e.Effect] = true
		}
		sentences += len(effectTypes) * 2
	}

	// Win conditions
	sentences += len(g.WinConditions)

	return sentences
}
