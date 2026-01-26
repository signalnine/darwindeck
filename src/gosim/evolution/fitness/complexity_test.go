package fitness

import (
	"testing"

	"github.com/signalnine/darwindeck/gosim/genome"
)

func TestCalculateComplexityWar(t *testing.T) {
	g := genome.CreateWarGenome()
	breakdown := CalculateComplexity(g)

	// War should be very simple
	if breakdown.TotalComplexity > 0.4 {
		t.Errorf("Expected low complexity for War, got %f", breakdown.TotalComplexity)
	}

	// Inverted score should be high (simpler = better)
	if breakdown.InvertedScore() < 0.6 {
		t.Errorf("Expected high inverted score for War, got %f", breakdown.InvertedScore())
	}
}

func TestCalculateComplexityHearts(t *testing.T) {
	g := genome.CreateHeartsGenome()
	breakdown := CalculateComplexity(g)

	// Hearts (trick-taking) should be moderate complexity
	if breakdown.TotalComplexity < 0.2 || breakdown.TotalComplexity > 0.7 {
		t.Errorf("Expected moderate complexity for Hearts, got %f", breakdown.TotalComplexity)
	}

	// Should get familiarity discount for trick-taking
	if breakdown.FamiliarPatternDiscount < 0.1 {
		t.Errorf("Expected familiarity discount for trick-taking, got %f", breakdown.FamiliarPatternDiscount)
	}
}

func TestCalculateComplexityPoker(t *testing.T) {
	// Create a poker-like game
	g := &genome.GameGenome{
		Name: "Test Poker",
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.DrawPhase{Source: genome.LocationDeck, Count: 5},
				&genome.BettingPhase{MinBet: 10, MaxRaises: 3},
				&genome.DiscardPhase{Target: genome.LocationDiscard, Count: 3},
				&genome.BettingPhase{MinBet: 10, MaxRaises: 3},
			},
			MaxTurns: 100,
		},
		WinConditions: []genome.WinCondition{
			{Type: genome.WinTypeBestHand},
		},
	}

	breakdown := CalculateComplexity(g)

	// Poker should be higher complexity due to hand rankings
	if breakdown.TotalComplexity < 0.4 {
		t.Errorf("Expected higher complexity for poker, got %f", breakdown.TotalComplexity)
	}

	// Memory cost should be significant (poker hand rankings)
	if breakdown.MemoryRequirements < 0.3 {
		t.Errorf("Expected high memory requirements for poker, got %f", breakdown.MemoryRequirements)
	}
}

func TestPhaseCostTrickTaking(t *testing.T) {
	g := &genome.GameGenome{
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.TrickPhase{LeadSuitRequired: true, TrumpSuit: 255},
			},
		},
	}

	cost := calculatePhaseCost(g)

	// Trick phase should have significant cost
	if cost < 0.4 {
		t.Errorf("Expected significant phase cost for trick-taking, got %f", cost)
	}
}

func TestPhaseCostSimple(t *testing.T) {
	g := &genome.GameGenome{
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.DrawPhase{Source: genome.LocationDeck, Count: 1},
				&genome.PlayPhase{Target: genome.LocationDiscard},
			},
		},
	}

	cost := calculatePhaseCost(g)

	// Simple draw-play should have low cost
	if cost > 0.5 {
		t.Errorf("Expected low phase cost for simple game, got %f", cost)
	}
}

func TestConditionComplexityNone(t *testing.T) {
	g := &genome.GameGenome{
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.DrawPhase{Source: genome.LocationDeck, Count: 1},
			},
		},
	}

	cost := calculateConditionComplexity(g)

	// No conditions should have zero cost
	if cost > 0.0 {
		t.Errorf("Expected zero condition complexity, got %f", cost)
	}
}

func TestConditionComplexityWithConditions(t *testing.T) {
	g := &genome.GameGenome{
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.DrawPhase{
					Source:    genome.LocationDeck,
					Count:     1,
					Condition: &genome.Condition{OpCode: 1, Value: 5},
				},
				&genome.PlayPhase{
					Target:             genome.LocationDiscard,
					ValidPlayCondition: &genome.Condition{OpCode: 2, Value: 3},
				},
			},
		},
	}

	cost := calculateConditionComplexity(g)

	// Conditions should add complexity
	if cost < 0.2 {
		t.Errorf("Expected condition complexity, got %f", cost)
	}
}

func TestEffectsCost(t *testing.T) {
	g := &genome.GameGenome{
		Effects: []genome.SpecialEffect{
			{TriggerRank: 8, Effect: genome.EffectWild},
			{TriggerRank: 2, Effect: genome.EffectDrawTwo},
			{TriggerRank: 0, Effect: genome.EffectSkipNext},
		},
	}

	cost := calculateEffectsCost(g)

	// Multiple effect types should have significant cost
	if cost < 0.3 {
		t.Errorf("Expected effects cost for 3 unique effects, got %f", cost)
	}
}

func TestMemoryCostTrickTaking(t *testing.T) {
	g := &genome.GameGenome{
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.TrickPhase{LeadSuitRequired: true},
			},
		},
		WinConditions: []genome.WinCondition{
			{Type: genome.WinTypeMostCaptured},
		},
	}

	cost := calculateMemoryCost(g)

	// Trick-taking with captures should have high memory cost
	if cost < 0.4 {
		t.Errorf("Expected high memory cost for trick-taking, got %f", cost)
	}
}

func TestFamiliarityDiscountTrickTaking(t *testing.T) {
	g := &genome.GameGenome{
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.TrickPhase{LeadSuitRequired: true},
			},
		},
	}

	discount := calculateFamiliarityDiscount(g)

	// Trick-taking should get familiarity discount
	if discount < 0.1 {
		t.Errorf("Expected familiarity discount for trick-taking, got %f", discount)
	}
}

func TestFamiliarityDiscountDrawPlay(t *testing.T) {
	g := &genome.GameGenome{
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.DrawPhase{Source: genome.LocationDeck, Count: 1},
				&genome.PlayPhase{Target: genome.LocationDiscard},
			},
		},
	}

	discount := calculateFamiliarityDiscount(g)

	// Simple draw-play should get familiarity discount
	if discount < 0.05 {
		t.Errorf("Expected familiarity discount for draw-play, got %f", discount)
	}
}

func TestEstimateExplanationSentences(t *testing.T) {
	// Simple game
	simple := &genome.GameGenome{
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.DrawPhase{Source: genome.LocationDeck, Count: 1},
				&genome.PlayPhase{Target: genome.LocationDiscard},
			},
		},
		WinConditions: []genome.WinCondition{
			{Type: genome.WinTypeEmptyHand},
		},
	}

	simpleSentences := estimateExplanationSentences(simple)

	// Complex game
	complex := &genome.GameGenome{
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.TrickPhase{LeadSuitRequired: true},
				&genome.BettingPhase{MinBet: 10},
			},
		},
		WinConditions: []genome.WinCondition{
			{Type: genome.WinTypeBestHand},
			{Type: genome.WinTypeHighScore},
		},
		Effects: []genome.SpecialEffect{
			{TriggerRank: 8, Effect: genome.EffectWild},
		},
	}

	complexSentences := estimateExplanationSentences(complex)

	// Complex should require more sentences
	if complexSentences <= simpleSentences {
		t.Errorf("Expected complex game to require more sentences (simple=%d, complex=%d)",
			simpleSentences, complexSentences)
	}
}

func TestComputeRulesComplexity(t *testing.T) {
	genomes := genome.GetSeedGenomes()

	for _, g := range genomes {
		t.Run(g.Name, func(t *testing.T) {
			score := ComputeRulesComplexity(g)

			// Score should be in valid range
			if score < 0.0 || score > 1.0 {
				t.Errorf("Expected complexity score in [0,1], got %f", score)
			}
		})
	}
}

func TestComplexityOrderingSimpleToComplex(t *testing.T) {
	war := genome.CreateWarGenome()
	hearts := genome.CreateHeartsGenome()

	warScore := ComputeRulesComplexity(war)
	heartsScore := ComputeRulesComplexity(hearts)

	// War should be simpler than Hearts
	if warScore < heartsScore {
		t.Errorf("Expected War (score=%f) to be simpler than Hearts (score=%f)",
			warScore, heartsScore)
	}
}
