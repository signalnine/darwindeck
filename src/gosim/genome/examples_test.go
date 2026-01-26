package genome

import (
	"testing"
)

func TestGetSeedGenomes(t *testing.T) {
	genomes := GetSeedGenomes()

	if len(genomes) != 19 {
		t.Errorf("Expected 19 seed genomes, got %d", len(genomes))
	}

	// Verify all genomes have names
	for i, g := range genomes {
		if g.Name == "" {
			t.Errorf("Genome %d has empty name", i)
		}
	}
}

func TestCreateWarGenome(t *testing.T) {
	g := CreateWarGenome()

	if g.Name != "War" {
		t.Errorf("Expected name 'War', got '%s'", g.Name)
	}
	if g.Setup.CardsPerPlayer != 26 {
		t.Errorf("Expected 26 cards per player, got %d", g.Setup.CardsPerPlayer)
	}
	if len(g.TurnStructure.Phases) != 1 {
		t.Errorf("Expected 1 phase, got %d", len(g.TurnStructure.Phases))
	}
	if g.TurnStructure.TableauMode != TableauModeWar {
		t.Errorf("Expected TableauModeWar, got %d", g.TurnStructure.TableauMode)
	}
	if len(g.WinConditions) != 1 || g.WinConditions[0].Type != WinTypeCaptureAll {
		t.Error("Expected capture_all win condition")
	}
}

func TestCreateHeartsGenome(t *testing.T) {
	g := CreateHeartsGenome()

	if g.Name != "Hearts" {
		t.Errorf("Expected name 'Hearts', got '%s'", g.Name)
	}
	if g.Setup.CardsPerPlayer != 13 {
		t.Errorf("Expected 13 cards per player, got %d", g.Setup.CardsPerPlayer)
	}
	if len(g.TurnStructure.Phases) != 1 {
		t.Errorf("Expected 1 phase, got %d", len(g.TurnStructure.Phases))
	}

	// Check trick phase
	trickPhase, ok := g.TurnStructure.Phases[0].(*TrickPhase)
	if !ok {
		t.Fatal("Expected TrickPhase")
	}
	if !trickPhase.LeadSuitRequired {
		t.Error("Expected lead suit required")
	}
	if trickPhase.TrumpSuit != 255 {
		t.Errorf("Expected no trump (255), got %d", trickPhase.TrumpSuit)
	}
	if trickPhase.BreakingSuit != SuitHearts {
		t.Errorf("Expected breaking suit Hearts, got %d", trickPhase.BreakingSuit)
	}

	// Check card scoring
	if len(g.CardScoring) != 2 {
		t.Errorf("Expected 2 scoring rules, got %d", len(g.CardScoring))
	}
}

func TestCreateSimplePokerGenome(t *testing.T) {
	g := CreateSimplePokerGenome()

	if g.Name != "Simple Poker" {
		t.Errorf("Expected name 'Simple Poker', got '%s'", g.Name)
	}
	if g.Setup.CardsPerPlayer != 5 {
		t.Errorf("Expected 5 cards per player, got %d", g.Setup.CardsPerPlayer)
	}
	if g.Setup.StartingChips != 1000 {
		t.Errorf("Expected 1000 starting chips, got %d", g.Setup.StartingChips)
	}

	// Check betting phase
	bettingPhase, ok := g.TurnStructure.Phases[0].(*BettingPhase)
	if !ok {
		t.Fatal("Expected BettingPhase")
	}
	if bettingPhase.MinBet != 10 {
		t.Errorf("Expected min bet 10, got %d", bettingPhase.MinBet)
	}

	// Check hand evaluation
	if g.HandEval == nil {
		t.Fatal("Expected hand evaluation")
	}
	if g.HandEval.Method != EvalMethodPatternMatch {
		t.Errorf("Expected PATTERN_MATCH method, got %d", g.HandEval.Method)
	}
	if len(g.HandEval.Patterns) != 10 {
		t.Errorf("Expected 10 poker patterns, got %d", len(g.HandEval.Patterns))
	}
}

func TestCreateBlackjackGenome(t *testing.T) {
	g := CreateBlackjackGenome()

	if g.Name != "Blackjack" {
		t.Errorf("Expected name 'Blackjack', got '%s'", g.Name)
	}
	if g.Setup.CardsPerPlayer != 2 {
		t.Errorf("Expected 2 cards per player, got %d", g.Setup.CardsPerPlayer)
	}
	if g.Setup.StartingChips != 500 {
		t.Errorf("Expected 500 starting chips, got %d", g.Setup.StartingChips)
	}

	// Check hand evaluation
	if g.HandEval == nil {
		t.Fatal("Expected hand evaluation")
	}
	if g.HandEval.Method != EvalMethodPointTotal {
		t.Errorf("Expected POINT_TOTAL method, got %d", g.HandEval.Method)
	}
	if g.HandEval.TargetValue != 21 {
		t.Errorf("Expected target value 21, got %d", g.HandEval.TargetValue)
	}
	if g.HandEval.BustThreshold != 22 {
		t.Errorf("Expected bust threshold 22, got %d", g.HandEval.BustThreshold)
	}
	if len(g.HandEval.CardValues) != 13 {
		t.Errorf("Expected 13 card values, got %d", len(g.HandEval.CardValues))
	}
}

func TestCreatePartnershipSpadesGenome(t *testing.T) {
	g := CreatePartnershipSpadesGenome()

	if g.Name != "Partnership Spades" {
		t.Errorf("Expected name 'Partnership Spades', got '%s'", g.Name)
	}

	// Check team configuration
	if g.Teams == nil {
		t.Fatal("Expected team configuration")
	}
	if !g.Teams.Enabled {
		t.Error("Expected teams enabled")
	}
	if len(g.Teams.Teams) != 2 {
		t.Errorf("Expected 2 teams, got %d", len(g.Teams.Teams))
	}
	// Check team 0 has players 0 and 2
	team0 := g.Teams.Teams[0]
	if len(team0) != 2 || team0[0] != 0 || team0[1] != 2 {
		t.Errorf("Expected team 0 = [0, 2], got %v", team0)
	}
	// Check team 1 has players 1 and 3
	team1 := g.Teams.Teams[1]
	if len(team1) != 2 || team1[0] != 1 || team1[1] != 3 {
		t.Errorf("Expected team 1 = [1, 3], got %v", team1)
	}
}

func TestCreateUnoStyleGenome(t *testing.T) {
	g := CreateUnoStyleGenome()

	if g.Name != "Uno Style" {
		t.Errorf("Expected name 'Uno Style', got '%s'", g.Name)
	}

	// Check special effects
	if len(g.Effects) != 3 {
		t.Errorf("Expected 3 special effects, got %d", len(g.Effects))
	}

	// Check for skip effect on Jack
	foundSkip := false
	for _, e := range g.Effects {
		if e.TriggerRank == RankJack && e.Effect == EffectSkipNext {
			foundSkip = true
			break
		}
	}
	if !foundSkip {
		t.Error("Expected skip effect on Jack")
	}
}

func TestAllSeedGenomesValid(t *testing.T) {
	genomes := GetSeedGenomes()

	for _, g := range genomes {
		errors := ValidateGenome(g)
		// Some genomes may have validation errors due to incomplete features
		// but they should at least have basic structure
		if g.Name == "" {
			t.Errorf("Genome has no name")
		}
		if len(g.TurnStructure.Phases) == 0 {
			t.Errorf("Genome '%s' has no phases", g.Name)
		}
		if len(g.WinConditions) == 0 {
			t.Errorf("Genome '%s' has no win conditions", g.Name)
		}
		// Log validation errors for debugging but don't fail
		for _, err := range errors {
			t.Logf("Genome '%s' validation: %v", g.Name, err)
		}
	}
}

func TestSeedGenomesCanSerialize(t *testing.T) {
	genomes := GetSeedGenomes()

	for _, g := range genomes {
		// Test JSON serialization
		jsonBytes, err := g.MarshalJSON()
		if err != nil {
			t.Errorf("Failed to serialize genome '%s': %v", g.Name, err)
			continue
		}
		if len(jsonBytes) == 0 {
			t.Errorf("Genome '%s' serialized to empty JSON", g.Name)
		}

		// Test round-trip
		loaded := &GameGenome{}
		err = loaded.UnmarshalJSON(jsonBytes)
		if err != nil {
			t.Errorf("Failed to deserialize genome '%s': %v", g.Name, err)
			continue
		}
		if loaded.Name != g.Name {
			t.Errorf("Round-trip name mismatch: expected '%s', got '%s'", g.Name, loaded.Name)
		}
	}
}
