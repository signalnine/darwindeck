package genome

import (
	"testing"
)

func TestValidateValidWarGenome(t *testing.T) {
	genome := &GameGenome{
		Name: "War",
		Setup: SetupRules{
			CardsPerPlayer: 26,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&PlayPhase{
					Target:   LocationTableau,
					MinCards: 1,
					MaxCards: 1,
				},
			},
			MaxTurns:    200,
			TableauMode: TableauModeWar,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeCaptureAll},
		},
	}

	errors := ValidateGenome(genome)
	if len(errors) > 0 {
		t.Errorf("Valid War genome should have no errors, got: %v", errors)
	}
}

func TestValidateTooManyCards(t *testing.T) {
	genome := &GameGenome{
		Name: "TooManyCards",
		Setup: SetupRules{
			CardsPerPlayer: 30, // 30 * 2 = 60 > 52
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&PlayPhase{Target: LocationDiscard},
			},
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}

	errors := ValidateGenome(genome)
	if len(errors) == 0 {
		t.Error("Expected error for too many cards, got none")
	}
	hasCardError := false
	for _, e := range errors {
		if e.Field == "setup.cards_per_player" {
			hasCardError = true
			break
		}
	}
	if !hasCardError {
		t.Errorf("Expected cards_per_player error, got: %v", errors)
	}
}

func TestValidateScoreWinWithoutScoring(t *testing.T) {
	genome := &GameGenome{
		Name: "ScoreWin",
		Setup: SetupRules{
			CardsPerPlayer: 7,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&PlayPhase{Target: LocationDiscard},
			},
		},
		WinConditions: []WinCondition{
			{Type: WinTypeHighScore, Threshold: 100},
		},
		// No CardScoring defined
	}

	errors := ValidateGenome(genome)
	if len(errors) == 0 {
		t.Error("Expected error for score win without scoring rules, got none")
	}
	found := false
	for _, e := range errors {
		if e.Field == "win_conditions" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected win_conditions error, got: %v", errors)
	}
}

func TestValidateBettingWithoutChips(t *testing.T) {
	genome := &GameGenome{
		Name: "Poker",
		Setup: SetupRules{
			CardsPerPlayer: 5,
			StartingChips:  0, // No chips!
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&DrawPhase{Source: LocationDeck, Count: 5},
				&BettingPhase{MinBet: 10, MaxRaises: 3},
			},
		},
		WinConditions: []WinCondition{
			{Type: WinTypeBestHand},
		},
	}

	errors := ValidateGenome(genome)
	if len(errors) == 0 {
		t.Error("Expected error for betting without chips, got none")
	}
	found := false
	for _, e := range errors {
		if e.Field == "setup.starting_chips" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected starting_chips error, got: %v", errors)
	}
}

func TestValidateCaptureWinWithoutTableauMode(t *testing.T) {
	genome := &GameGenome{
		Name: "CaptureGame",
		Setup: SetupRules{
			CardsPerPlayer: 26,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&PlayPhase{Target: LocationDiscard},
			},
			TableauMode: TableauModeNone, // No capture mode
		},
		WinConditions: []WinCondition{
			{Type: WinTypeCaptureAll},
		},
	}

	errors := ValidateGenome(genome)
	if len(errors) == 0 {
		t.Error("Expected error for capture win without tableau mode, got none")
	}
	found := false
	for _, e := range errors {
		if e.Field == "turn_structure.tableau_mode" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected tableau_mode error, got: %v", errors)
	}
}

func TestValidateNoCardPlayPhases(t *testing.T) {
	genome := &GameGenome{
		Name: "OnlyBetting",
		Setup: SetupRules{
			CardsPerPlayer: 5,
			StartingChips:  1000,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&BettingPhase{MinBet: 10, MaxRaises: 3},
			},
		},
		WinConditions: []WinCondition{
			{Type: WinTypeBestHand},
		},
	}

	errors := ValidateGenome(genome)
	found := false
	for _, e := range errors {
		if e.Field == "turn_structure.phases" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected phases error for no card play, got: %v", errors)
	}
}

func TestValidateBettingMinBetTooHigh(t *testing.T) {
	genome := &GameGenome{
		Name: "HighMinBet",
		Setup: SetupRules{
			CardsPerPlayer: 5,
			StartingChips:  100,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&DrawPhase{Source: LocationDeck, Count: 5},
				&BettingPhase{MinBet: 60, MaxRaises: 3}, // 60 > 100/2
			},
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}

	errors := ValidateGenome(genome)
	found := false
	for _, e := range errors {
		if e.Field == "betting_phase.min_bet" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected min_bet error, got: %v", errors)
	}
}

func TestValidateBiddingWithoutTrick(t *testing.T) {
	genome := &GameGenome{
		Name: "BiddingNoTrick",
		Setup: SetupRules{
			CardsPerPlayer: 13,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&BiddingPhase{MinBid: 0, MaxBid: 13},
				&PlayPhase{Target: LocationDiscard}, // PlayPhase, not TrickPhase
			},
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}

	errors := ValidateGenome(genome)
	found := false
	for _, e := range errors {
		if e.Message == "BiddingPhase requires at least one TrickPhase (contracts need tricks)" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected bidding/trick error, got: %v", errors)
	}
}

func TestValidateValidTrickGame(t *testing.T) {
	genome := &GameGenome{
		Name: "Hearts",
		Setup: SetupRules{
			CardsPerPlayer: 13,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&TrickPhase{
					LeadSuitRequired: true,
					TrumpSuit:        255, // No trump
					HighCardWins:     true,
					BreakingSuit:     0, // Hearts
				},
			},
			MaxTurns: 52,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeLowScore, Threshold: 100},
		},
		CardScoring: []CardScoringRule{
			{Suit: 0, Rank: 255, Points: 1, Trigger: TriggerTrickWin}, // Hearts
		},
	}

	errors := ValidateGenome(genome)
	if len(errors) > 0 {
		t.Errorf("Valid Hearts genome should have no errors, got: %v", errors)
	}
}

func TestIsValid(t *testing.T) {
	validGenome := &GameGenome{
		Name: "SimpleGame",
		Setup: SetupRules{
			CardsPerPlayer: 7,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&PlayPhase{Target: LocationDiscard, MinCards: 1, MaxCards: 1},
			},
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}

	if !IsValid(validGenome) {
		t.Error("Expected IsValid to return true for valid genome")
	}

	invalidGenome := &GameGenome{
		Name: "NoPhases",
		Setup: SetupRules{
			CardsPerPlayer: 7,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{}, // No phases!
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}

	if IsValid(invalidGenome) {
		t.Error("Expected IsValid to return false for invalid genome")
	}
}
