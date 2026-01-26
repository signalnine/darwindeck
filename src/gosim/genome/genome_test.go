package genome

import (
	"encoding/json"
	"testing"

	"github.com/signalnine/darwindeck/gosim/engine"
)

// TestWarGenomeTyped creates a War game using typed structs and verifies simulation.
func TestWarGenomeTyped(t *testing.T) {
	// Create War genome using typed structs
	warGenome := &GameGenome{
		Name: "War",
		Setup: SetupRules{
			CardsPerPlayer: 26, // Split deck between 2 players
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&PlayPhase{
					Target:   LocationTableau,
					MinCards: 1,
					MaxCards: 1,
					Mandatory: true,
				},
			},
			MaxTurns:    200,
			TableauMode: TableauModeWar,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeCaptureAll},
		},
	}

	// Test JSON serialization round-trip
	jsonBytes, err := json.MarshalIndent(warGenome, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal genome: %v", err)
	}

	t.Logf("War genome JSON:\n%s", string(jsonBytes))

	// Unmarshal back
	var loadedGenome GameGenome
	if err := json.Unmarshal(jsonBytes, &loadedGenome); err != nil {
		t.Fatalf("Failed to unmarshal genome: %v", err)
	}

	// Verify fields
	if loadedGenome.Name != "War" {
		t.Errorf("Name mismatch: got %q, want %q", loadedGenome.Name, "War")
	}
	if loadedGenome.Setup.CardsPerPlayer != 26 {
		t.Errorf("CardsPerPlayer mismatch: got %d, want 26", loadedGenome.Setup.CardsPerPlayer)
	}
	if len(loadedGenome.TurnStructure.Phases) != 1 {
		t.Fatalf("Phase count mismatch: got %d, want 1", len(loadedGenome.TurnStructure.Phases))
	}
	if loadedGenome.TurnStructure.MaxTurns != 200 {
		t.Errorf("MaxTurns mismatch: got %d, want 200", loadedGenome.TurnStructure.MaxTurns)
	}

	// Verify phase type
	playPhase, ok := loadedGenome.TurnStructure.Phases[0].(*PlayPhase)
	if !ok {
		t.Fatalf("Phase 0 is not PlayPhase: got %T", loadedGenome.TurnStructure.Phases[0])
	}
	if playPhase.Target != LocationTableau {
		t.Errorf("PlayPhase target mismatch: got %d, want %d", playPhase.Target, LocationTableau)
	}
}

// TestDrawPhaseMovegen tests that typed DrawPhase generates correct moves.
func TestDrawPhaseMovegen(t *testing.T) {
	// Create a simple draw game
	genome := &GameGenome{
		Name: "SimpleDraw",
		Setup: SetupRules{
			CardsPerPlayer: 5,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&DrawPhase{
					Source:    LocationDeck,
					Count:     1,
					Mandatory: false, // Allow pass
				},
				&PlayPhase{
					Target:       LocationDiscard,
					MinCards:     1,
					MaxCards:     1,
					PassIfUnable: true,
				},
			},
			MaxTurns: 100,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}

	// Create game state manually
	state := engine.NewGameState(2)
	state.CurrentPlayer = 0

	// Give player 0 some cards
	state.Players[0].Hand = []engine.Card{
		{Rank: 2, Suit: 0},
		{Rank: 3, Suit: 0},
	}

	// Put some cards in deck
	state.Deck = []engine.Card{
		{Rank: 4, Suit: 0},
		{Rank: 5, Suit: 0},
	}

	// Generate moves using typed interpreter
	moves := GenerateLegalMovesTyped(state, genome)

	// Should have: 1 draw, 1 pass (from DrawPhase), 2 play moves (from PlayPhase)
	// Actually: draw phase generates draw + pass when !mandatory
	// play phase generates 2 card plays
	// Total: 4 moves
	if len(moves) < 2 {
		t.Errorf("Expected at least 2 moves, got %d", len(moves))
	}

	// Check that we have a draw move
	hasDrawMove := false
	for _, m := range moves {
		if m.CardIndex == engine.MoveDraw {
			hasDrawMove = true
			break
		}
	}
	if !hasDrawMove {
		t.Error("Expected a draw move to be generated")
	}

	t.Logf("Generated %d moves:", len(moves))
	for i, m := range moves {
		t.Logf("  Move %d: PhaseIndex=%d, CardIndex=%d, TargetLoc=%d",
			i, m.PhaseIndex, m.CardIndex, m.TargetLoc)
	}
}

// TestPlayPhaseMovegen tests that typed PlayPhase generates correct moves.
func TestPlayPhaseMovegen(t *testing.T) {
	genome := &GameGenome{
		Name: "SimplePlay",
		Setup: SetupRules{
			CardsPerPlayer: 5,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&PlayPhase{
					Target:   LocationDiscard,
					MinCards: 1,
					MaxCards: 1,
					Mandatory: true,
				},
			},
			MaxTurns: 100,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}

	state := engine.NewGameState(2)
	state.CurrentPlayer = 0

	// Give player 0 three cards
	state.Players[0].Hand = []engine.Card{
		{Rank: 2, Suit: 0},
		{Rank: 5, Suit: 1},
		{Rank: 10, Suit: 2},
	}

	moves := GenerateLegalMovesTyped(state, genome)

	// Should have 3 play moves (one per card)
	if len(moves) != 3 {
		t.Errorf("Expected 3 moves, got %d", len(moves))
	}

	// Verify each move targets discard
	for i, m := range moves {
		if m.TargetLoc != engine.LocationDiscard {
			t.Errorf("Move %d targets wrong location: got %d, want %d",
				i, m.TargetLoc, engine.LocationDiscard)
		}
		if m.CardIndex < 0 || m.CardIndex > 2 {
			t.Errorf("Move %d has invalid card index: %d", i, m.CardIndex)
		}
	}
}

// TestTrickPhaseMovegen tests trick-taking move generation.
func TestTrickPhaseMovegen(t *testing.T) {
	genome := &GameGenome{
		Name: "SimpleTricks",
		Setup: SetupRules{
			CardsPerPlayer: 13,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&TrickPhase{
					LeadSuitRequired: true,
					TrumpSuit:        255, // No trump
					HighCardWins:     true,
					BreakingSuit:     255, // No breaking suit
				},
			},
			MaxTurns: 52,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeAllHandsEmpty},
		},
	}

	state := engine.NewGameState(2)
	state.CurrentPlayer = 0

	// Give player 0 cards of mixed suits
	state.Players[0].Hand = []engine.Card{
		{Rank: 2, Suit: 0}, // Hearts
		{Rank: 5, Suit: 0}, // Hearts
		{Rank: 10, Suit: 1}, // Diamonds
	}

	// Test leading (no current trick) - should be able to play any card
	moves := GenerateLegalMovesTyped(state, genome)
	if len(moves) != 3 {
		t.Errorf("Leading: expected 3 moves, got %d", len(moves))
	}

	// Test following - must follow suit if able
	state.CurrentTrick = []engine.TrickCard{
		{PlayerID: 1, Card: engine.Card{Rank: 7, Suit: 0}}, // Hearts led
	}

	moves = GenerateLegalMovesTyped(state, genome)

	// Player has 2 hearts, should only be able to play those
	if len(moves) != 2 {
		t.Errorf("Following suit: expected 2 moves, got %d", len(moves))
	}
	for _, m := range moves {
		card := state.Players[0].Hand[m.CardIndex]
		if card.Suit != 0 {
			t.Errorf("Expected hearts only, got suit %d", card.Suit)
		}
	}
}

// TestBettingPhaseMovegen tests betting move generation.
func TestBettingPhaseMovegen(t *testing.T) {
	genome := &GameGenome{
		Name: "SimplePoker",
		Setup: SetupRules{
			CardsPerPlayer: 5,
			StartingChips:  1000,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&BettingPhase{
					MinBet:    10,
					MaxRaises: 3,
				},
			},
			MaxTurns: 100,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeBestHand},
		},
	}

	state := engine.NewGameState(2)
	state.CurrentPlayer = 0

	// Initialize betting state
	state.Players[0].Chips = 1000
	state.Players[0].CurrentBet = 0
	state.Players[0].HasFolded = false
	state.Players[0].IsAllIn = false
	state.Players[1].Chips = 1000
	state.Players[1].CurrentBet = 0
	state.Players[1].HasFolded = false
	state.Players[1].IsAllIn = false
	state.CurrentBet = 0
	state.Pot = 0
	state.BettingComplete = false

	moves := GenerateLegalMovesTyped(state, genome)

	// Should have betting options: check, bet, all-in (at minimum)
	if len(moves) < 2 {
		t.Errorf("Expected at least 2 betting moves, got %d", len(moves))
	}

	t.Logf("Betting moves generated: %d", len(moves))
	for i, m := range moves {
		actionCode := -(m.CardIndex + 10)
		t.Logf("  Move %d: action=%d", i, actionCode)
	}
}

// TestLoadPythonFormat tests loading Python-format genome JSON.
func TestLoadPythonFormat(t *testing.T) {
	// This is the exact format Python generates
	pythonJSON := `{
		"schema_version": "1.0",
		"genome_id": "test_war",
		"generation": 1,
		"setup": {
			"cards_per_player": 26,
			"initial_deck": "standard_52",
			"initial_discard_count": 0,
			"starting_chips": 0,
			"tableau_mode": "war",
			"sequence_direction": "both"
		},
		"turn_structure": {
			"phases": [
				{
					"type": "PlayPhase",
					"target": "TABLEAU",
					"valid_play_condition": null,
					"min_cards": 1,
					"max_cards": 1,
					"mandatory": true
				}
			],
			"is_trick_based": false,
			"tricks_per_hand": null
		},
		"special_effects": [],
		"win_conditions": [
			{"type": "capture_all", "threshold": null}
		],
		"scoring_rules": [],
		"max_turns": 200,
		"min_turns": 1,
		"player_count": 2
	}`

	genome, err := LoadGenomeFromJSON([]byte(pythonJSON))
	if err != nil {
		t.Fatalf("Failed to load Python format genome: %v", err)
	}

	// Verify key fields
	if genome.Name != "test_war" {
		t.Errorf("Name mismatch: got %q, want %q", genome.Name, "test_war")
	}
	if genome.Setup.CardsPerPlayer != 26 {
		t.Errorf("CardsPerPlayer mismatch: got %d, want 26", genome.Setup.CardsPerPlayer)
	}
	if genome.TurnStructure.MaxTurns != 200 {
		t.Errorf("MaxTurns mismatch: got %d, want 200", genome.TurnStructure.MaxTurns)
	}
	if genome.TurnStructure.TableauMode != TableauModeWar {
		t.Errorf("TableauMode mismatch: got %d, want %d", genome.TurnStructure.TableauMode, TableauModeWar)
	}
	if len(genome.TurnStructure.Phases) != 1 {
		t.Fatalf("Phase count mismatch: got %d, want 1", len(genome.TurnStructure.Phases))
	}

	// Verify phase type and fields
	playPhase, ok := genome.TurnStructure.Phases[0].(*PlayPhase)
	if !ok {
		t.Fatalf("Phase 0 is not PlayPhase: got %T", genome.TurnStructure.Phases[0])
	}
	if playPhase.Target != LocationTableau {
		t.Errorf("PlayPhase target mismatch: got %d, want %d", playPhase.Target, LocationTableau)
	}
	if playPhase.MinCards != 1 || playPhase.MaxCards != 1 {
		t.Errorf("PlayPhase card counts mismatch: got min=%d, max=%d", playPhase.MinCards, playPhase.MaxCards)
	}

	// Verify win conditions
	if len(genome.WinConditions) != 1 {
		t.Fatalf("WinCondition count mismatch: got %d, want 1", len(genome.WinConditions))
	}
	if genome.WinConditions[0].Type != WinTypeCaptureAll {
		t.Errorf("WinCondition type mismatch: got %d, want %d", genome.WinConditions[0].Type, WinTypeCaptureAll)
	}

	t.Logf("Successfully loaded Python format genome: %s", genome.Name)
}

// TestLoadRealPythonGenome tests loading an actual Python-generated genome file.
func TestLoadRealPythonGenome(t *testing.T) {
	// This is the exact content from rank01_GrandRite.json
	realPythonJSON := `{
		"schema_version": "1.0",
		"genome_id": "GrandRite",
		"generation": 1,
		"setup": {
			"cards_per_player": 13,
			"initial_deck": "standard_52",
			"initial_discard_count": 0
		},
		"turn_structure": {
			"phases": [
				{
					"type": "TrickPhase",
					"lead_suit_required": true,
					"trump_suit": null,
					"high_card_wins": true,
					"breaking_suit": "HEARTS"
				}
			],
			"is_trick_based": true,
			"tricks_per_hand": 13
		},
		"special_effects": [],
		"win_conditions": [
			{"type": "low_score", "threshold": 100},
			{"type": "all_hands_empty", "threshold": 0}
		],
		"scoring_rules": [],
		"max_turns": 500,
		"min_turns": 52,
		"player_count": 4
	}`

	genome, err := LoadGenomeFromJSON([]byte(realPythonJSON))
	if err != nil {
		t.Fatalf("Failed to load real Python genome: %v", err)
	}

	// Verify key fields
	if genome.Name != "GrandRite" {
		t.Errorf("Name mismatch: got %q, want %q", genome.Name, "GrandRite")
	}
	if genome.Setup.CardsPerPlayer != 13 {
		t.Errorf("CardsPerPlayer mismatch: got %d, want 13", genome.Setup.CardsPerPlayer)
	}
	if genome.TurnStructure.MaxTurns != 500 {
		t.Errorf("MaxTurns mismatch: got %d, want 500", genome.TurnStructure.MaxTurns)
	}
	if len(genome.TurnStructure.Phases) != 1 {
		t.Fatalf("Phase count mismatch: got %d, want 1", len(genome.TurnStructure.Phases))
	}

	// Verify trick phase
	trickPhase, ok := genome.TurnStructure.Phases[0].(*TrickPhase)
	if !ok {
		t.Fatalf("Phase 0 is not TrickPhase: got %T", genome.TurnStructure.Phases[0])
	}
	if !trickPhase.LeadSuitRequired {
		t.Error("TrickPhase LeadSuitRequired should be true")
	}
	if !trickPhase.HighCardWins {
		t.Error("TrickPhase HighCardWins should be true")
	}
	if trickPhase.BreakingSuit != 0 { // HEARTS = 0
		t.Errorf("TrickPhase BreakingSuit mismatch: got %d, want 0 (HEARTS)", trickPhase.BreakingSuit)
	}

	// Verify win conditions
	if len(genome.WinConditions) != 2 {
		t.Fatalf("WinCondition count mismatch: got %d, want 2", len(genome.WinConditions))
	}
	if genome.WinConditions[0].Type != WinTypeLowScore {
		t.Errorf("WinCondition 0 type mismatch: got %d, want %d", genome.WinConditions[0].Type, WinTypeLowScore)
	}
	if genome.WinConditions[1].Type != WinTypeAllHandsEmpty {
		t.Errorf("WinCondition 1 type mismatch: got %d, want %d", genome.WinConditions[1].Type, WinTypeAllHandsEmpty)
	}

	t.Logf("Successfully loaded real Python genome: %s", genome.Name)
}

// TestLoadPythonCondition tests loading Python-format conditions.
func TestLoadPythonCondition(t *testing.T) {
	pythonJSON := `{
		"schema_version": "1.0",
		"genome_id": "test_condition",
		"generation": 1,
		"setup": {
			"cards_per_player": 7,
			"initial_deck": "standard_52",
			"initial_discard_count": 0
		},
		"turn_structure": {
			"phases": [
				{
					"type": "PlayPhase",
					"target": "DISCARD",
					"valid_play_condition": {
						"type": "simple",
						"condition_type": "MATCH_SUIT",
						"operator": "EQ",
						"value": null,
						"reference": "HEARTS"
					},
					"min_cards": 1,
					"max_cards": 1,
					"mandatory": false
				}
			],
			"is_trick_based": false
		},
		"win_conditions": [
			{"type": "empty_hand", "threshold": null}
		],
		"max_turns": 100,
		"player_count": 2
	}`

	genome, err := LoadGenomeFromJSON([]byte(pythonJSON))
	if err != nil {
		t.Fatalf("Failed to load Python format genome with condition: %v", err)
	}

	playPhase, ok := genome.TurnStructure.Phases[0].(*PlayPhase)
	if !ok {
		t.Fatalf("Phase 0 is not PlayPhase: got %T", genome.TurnStructure.Phases[0])
	}
	if playPhase.ValidPlayCondition == nil {
		t.Fatal("ValidPlayCondition is nil, expected condition")
	}

	// Verify condition was parsed
	cond := playPhase.ValidPlayCondition
	t.Logf("Condition: OpCode=%d, Operator=%d, Value=%d", cond.OpCode, cond.Operator, cond.Value)

	// MATCH_SUIT should map to opcode 13
	if cond.OpCode != 13 {
		t.Errorf("Condition OpCode mismatch: got %d, want 13 (MATCH_SUIT)", cond.OpCode)
	}
	// HEARTS should map to suit 0
	if cond.Value != 0 {
		t.Errorf("Condition Value mismatch: got %d, want 0 (HEARTS)", cond.Value)
	}
}

// TestJSONRoundTrip tests full JSON serialization/deserialization.
func TestJSONRoundTrip(t *testing.T) {
	original := &GameGenome{
		Name: "TestGame",
		Setup: SetupRules{
			CardsPerPlayer: 7,
			TableauSize:    4,
			StartingChips:  500,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&DrawPhase{
					Source:    LocationDeck,
					Count:     1,
					Mandatory: true,
				},
				&PlayPhase{
					Target:       LocationDiscard,
					MinCards:     1,
					MaxCards:     3,
					PassIfUnable: true,
					ValidPlayCondition: &Condition{
						OpCode:   12, // check_card_matches_rank
						Operator: 50, // eq
						Value:    0,
						RefLoc:   2,  // discard
					},
				},
				&BettingPhase{
					MinBet:    25,
					MaxRaises: 4,
				},
			},
			MaxTurns:          150,
			TableauMode:       TableauModeSequence,
			SequenceDirection: SequenceBoth,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
			{Type: WinTypeHighScore, Threshold: 100},
		},
		Effects: []SpecialEffect{
			{TriggerRank: 10, Effect: EffectSkipNext, Target: 0, Value: 1},
		},
		CardScoring: []CardScoringRule{
			{Suit: 0, Rank: 255, Points: 1, Trigger: TriggerTrickWin},
		},
	}

	// Serialize
	jsonBytes, err := SaveGenomeToJSON(original)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	t.Logf("Serialized JSON:\n%s", string(jsonBytes))

	// Deserialize
	loaded, err := LoadGenomeFromJSON(jsonBytes)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	// Verify
	if loaded.Name != original.Name {
		t.Errorf("Name mismatch: got %q, want %q", loaded.Name, original.Name)
	}
	if loaded.Setup.CardsPerPlayer != original.Setup.CardsPerPlayer {
		t.Errorf("CardsPerPlayer mismatch")
	}
	if loaded.Setup.StartingChips != original.Setup.StartingChips {
		t.Errorf("StartingChips mismatch")
	}
	if len(loaded.TurnStructure.Phases) != len(original.TurnStructure.Phases) {
		t.Errorf("Phase count mismatch: got %d, want %d",
			len(loaded.TurnStructure.Phases), len(original.TurnStructure.Phases))
	}
	if loaded.TurnStructure.TableauMode != original.TurnStructure.TableauMode {
		t.Errorf("TableauMode mismatch: got %d, want %d",
			loaded.TurnStructure.TableauMode, original.TurnStructure.TableauMode)
	}
	if len(loaded.WinConditions) != len(original.WinConditions) {
		t.Errorf("WinCondition count mismatch")
	}
	if len(loaded.Effects) != len(original.Effects) {
		t.Errorf("Effects count mismatch")
	}
	if len(loaded.CardScoring) != len(original.CardScoring) {
		t.Errorf("CardScoring count mismatch")
	}

	// Verify condition survived round-trip
	playPhase := loaded.TurnStructure.Phases[1].(*PlayPhase)
	if playPhase.ValidPlayCondition == nil {
		t.Error("ValidPlayCondition lost during round-trip")
	} else if playPhase.ValidPlayCondition.OpCode != 12 {
		t.Errorf("Condition OpCode mismatch: got %d, want 12", playPhase.ValidPlayCondition.OpCode)
	}
}
