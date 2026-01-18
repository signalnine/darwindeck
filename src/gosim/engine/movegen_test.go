package engine

import (
	"testing"
)

// TestApplyMoveTableauModeNone verifies that with NONE mode, cards stay on tableau
// without any battle resolution
func TestApplyMoveTableauModeNone(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 0 // NONE
	state.NumPlayers = 2

	// Setup: give player cards
	state.Players[0].Hand = []Card{{Rank: 10, Suit: 0}}
	state.Players[1].Hand = []Card{{Rank: 5, Suit: 0}}

	// Initialize tableau
	state.Tableau = make([][]Card, 1)
	state.Tableau[0] = []Card{}

	// Create minimal genome with play phase
	genome := minimalPlayPhaseGenome()

	// Play cards to tableau
	state.CurrentPlayer = 0
	move := LegalMove{PhaseIndex: 0, CardIndex: 0, TargetLoc: LocationTableau}
	ApplyMove(state, &move, genome)

	// Reset current player to 1 (ApplyMove advances turn)
	state.CurrentPlayer = 1
	state.Players[1].Hand = []Card{{Rank: 5, Suit: 0}}
	ApplyMove(state, &move, genome)

	// With NONE mode, cards should stay on tableau (no battle resolution)
	if len(state.Tableau[0]) != 2 {
		t.Errorf("Expected 2 cards on tableau with NONE mode, got %d", len(state.Tableau[0]))
	}
}

// TestApplyMoveTableauModeWar verifies War-style battle resolution where
// higher rank wins and takes both cards to hand
func TestApplyMoveTableauModeWar(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 1 // WAR
	state.NumPlayers = 2

	// Setup: give player cards
	state.Players[0].Hand = []Card{{Rank: 10, Suit: 0}}
	state.Players[1].Hand = []Card{{Rank: 5, Suit: 0}}

	// Initialize tableau
	state.Tableau = make([][]Card, 1)
	state.Tableau[0] = []Card{}

	genome := minimalPlayPhaseGenome()

	// Player 0 plays (rank 10)
	state.CurrentPlayer = 0
	move := LegalMove{PhaseIndex: 0, CardIndex: 0, TargetLoc: LocationTableau}
	ApplyMove(state, &move, genome)

	// Player 1 plays (rank 5) - should trigger war battle
	state.CurrentPlayer = 1
	state.Players[1].Hand = []Card{{Rank: 5, Suit: 0}}
	ApplyMove(state, &move, genome)

	// With WAR mode, player 0 (higher rank) should have won both cards
	// Cards are added to hand in War
	if len(state.Players[0].Hand) != 2 {
		t.Errorf("Expected player 0 to have 2 cards after winning war, got %d", len(state.Players[0].Hand))
	}
	if len(state.Tableau[0]) != 0 {
		t.Errorf("Expected empty tableau after war, got %d cards", len(state.Tableau[0]))
	}
}

// TestApplyMoveTableauModeMatchRank verifies Scopa-style capture where
// playing a card captures any tableau card with matching rank
func TestApplyMoveTableauModeMatchRank(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 2 // MATCH_RANK
	state.NumPlayers = 2

	// Setup: card on tableau to match
	state.Tableau = make([][]Card, 1)
	state.Tableau[0] = []Card{{Rank: 7, Suit: 1}} // 7 of different suit

	// Player has a 7 that can capture
	state.Players[0].Hand = []Card{{Rank: 7, Suit: 0}}
	state.CurrentPlayer = 0

	genome := minimalPlayPhaseGenome()

	move := LegalMove{PhaseIndex: 0, CardIndex: 0, TargetLoc: LocationTableau}
	ApplyMove(state, &move, genome)

	// With MATCH_RANK mode, player should have scored 2 points for the capture
	// (the current implementation uses Score to track captures)
	if state.Players[0].Score != 2 {
		t.Errorf("Expected player 0 to have score 2 for capture, got %d", state.Players[0].Score)
	}
	if len(state.Tableau[0]) != 0 {
		t.Errorf("Expected empty tableau after match capture, got %d cards", len(state.Tableau[0]))
	}
}

// TestApplyMoveTableauModeSequence verifies SEQUENCE mode where cards
// must follow sequence rules (validation in move generation, just add to pile here)
func TestApplyMoveTableauModeSequence(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 3 // SEQUENCE
	state.NumPlayers = 2

	// Setup: card on tableau
	state.Tableau = make([][]Card, 1)
	state.Tableau[0] = []Card{{Rank: 5, Suit: 0}} // 5 on tableau

	// Player plays a 6 (next in sequence)
	state.Players[0].Hand = []Card{{Rank: 6, Suit: 0}}
	state.CurrentPlayer = 0

	genome := minimalPlayPhaseGenome()

	move := LegalMove{PhaseIndex: 0, CardIndex: 0, TargetLoc: LocationTableau}
	ApplyMove(state, &move, genome)

	// With SEQUENCE mode, card should just be added to tableau (validation in move gen)
	if len(state.Tableau) == 0 || len(state.Tableau[0]) != 2 {
		tableauLen := 0
		if len(state.Tableau) > 0 {
			tableauLen = len(state.Tableau[0])
		}
		t.Fatalf("Expected 2 cards on tableau with SEQUENCE mode, got %d", tableauLen)
	}
	// Verify the cards are the 5 and 6
	if state.Tableau[0][0].Rank != 5 || state.Tableau[0][1].Rank != 6 {
		t.Errorf("Expected ranks [5, 6], got [%d, %d]", state.Tableau[0][0].Rank, state.Tableau[0][1].Rank)
	}
}

// TestApplyMoveTableauModeWarTie verifies War tie handling
func TestApplyMoveTableauModeWarTie(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 1 // WAR
	state.NumPlayers = 2
	state.TurnNumber = 0  // Even battle number = player 0 wins ties

	// Setup: both players have same rank
	state.Players[0].Hand = []Card{{Rank: 7, Suit: 0}}
	state.Players[1].Hand = []Card{{Rank: 7, Suit: 1}}

	// Initialize tableau
	state.Tableau = make([][]Card, 1)
	state.Tableau[0] = []Card{}

	genome := minimalPlayPhaseGenome()

	// Player 0 plays
	state.CurrentPlayer = 0
	move := LegalMove{PhaseIndex: 0, CardIndex: 0, TargetLoc: LocationTableau}
	ApplyMove(state, &move, genome)

	// Player 1 plays - triggers tie
	state.CurrentPlayer = 1
	state.Players[1].Hand = []Card{{Rank: 7, Suit: 1}}
	ApplyMove(state, &move, genome)

	// Player 0 should win the tie (battle 0 % 2 = 0)
	if len(state.Players[0].Hand) != 2 {
		t.Errorf("Expected player 0 to have 2 cards after tie, got %d", len(state.Players[0].Hand))
	}
}

// TestApplyMoveTableauModeMatchRankNoMatch verifies that when there's no
// matching card, the played card stays on the tableau
func TestApplyMoveTableauModeMatchRankNoMatch(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 2 // MATCH_RANK
	state.NumPlayers = 2

	// Setup: card on tableau with different rank
	state.Tableau = make([][]Card, 1)
	state.Tableau[0] = []Card{{Rank: 8, Suit: 1}} // 8 on tableau

	// Player has a 7 (no match)
	state.Players[0].Hand = []Card{{Rank: 7, Suit: 0}}
	state.CurrentPlayer = 0

	genome := minimalPlayPhaseGenome()

	move := LegalMove{PhaseIndex: 0, CardIndex: 0, TargetLoc: LocationTableau}
	ApplyMove(state, &move, genome)

	// No match - both cards should remain on tableau
	if len(state.Tableau[0]) != 2 {
		t.Errorf("Expected 2 cards on tableau (no match), got %d", len(state.Tableau[0]))
	}
	// Score should be 0 (no capture)
	if state.Players[0].Score != 0 {
		t.Errorf("Expected score 0 (no capture), got %d", state.Players[0].Score)
	}
}

// TestSequenceModeEmptyTableau verifies that on empty tableau any card is playable
func TestSequenceModeEmptyTableau(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 3       // SEQUENCE
	state.SequenceDirection = 0 // ASCENDING
	state.NumPlayers = 2

	// Empty tableau - 4 piles (one per suit)
	state.Tableau = make([][]Card, 4)
	for i := range state.Tableau {
		state.Tableau[i] = []Card{}
	}

	// Player has cards of different suits
	state.Players[0].Hand = []Card{
		{Rank: 7, Suit: 0},  // 7 of spades
		{Rank: 10, Suit: 1}, // 10 of hearts
	}
	state.CurrentPlayer = 0

	genome := sequencePhaseGenome()
	moves := GenerateLegalMoves(state, genome)

	// Both cards should be playable on empty tableau (each can start a new pile)
	tableauMoves := 0
	for _, m := range moves {
		if m.TargetLoc == LocationTableau {
			tableauMoves++
		}
	}
	if tableauMoves < 2 {
		t.Errorf("Expected at least 2 tableau moves on empty tableau, got %d", tableauMoves)
	}
}

// TestSequenceModeAscending verifies ascending sequence rules
func TestSequenceModeAscending(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 3       // SEQUENCE
	state.SequenceDirection = 0 // ASCENDING
	state.NumPlayers = 2

	// Tableau has 7 of spades as top card in pile 0
	state.Tableau = make([][]Card, 4)
	state.Tableau[0] = []Card{{Rank: 7, Suit: 0}} // Spade pile with 7
	state.Tableau[1] = []Card{}                   // Hearts pile empty
	state.Tableau[2] = []Card{}                   // Diamonds pile empty
	state.Tableau[3] = []Card{}                   // Clubs pile empty

	// Player has: 6 (invalid descending), 8 (valid ascending), 9 (invalid not adjacent), wrong suit
	state.Players[0].Hand = []Card{
		{Rank: 6, Suit: 0}, // Invalid - descending from 7
		{Rank: 8, Suit: 0}, // Valid - ascending from 7
		{Rank: 9, Suit: 0}, // Invalid - not adjacent to 7
		{Rank: 8, Suit: 1}, // Can start new hearts pile
	}
	state.CurrentPlayer = 0

	genome := sequencePhaseGenome()
	moves := GenerateLegalMoves(state, genome)

	// Count valid tableau moves
	// - 8 of spades can play on spade pile (ascending from 7)
	// - 8 of hearts can start new hearts pile
	// - 6 of spades can start new pile (on empty pile)
	// - 9 of spades can start new pile (on empty pile)
	// With 4 piles (3 empty), each card can potentially go to an empty pile
	// But the key test: 8 of spades should be the ONLY card valid on the non-empty spade pile

	// For a proper test, let's count moves that target the non-empty pile
	validOnSpades := 0
	for _, m := range moves {
		if m.TargetLoc == LocationTableau {
			// Check if this is for the 8 of spades
			card := state.Players[0].Hand[m.CardIndex]
			if card.Suit == 0 && card.Rank == 8 {
				validOnSpades++
			}
		}
	}

	// The 8 of spades should be valid (at least one move for it)
	if validOnSpades == 0 {
		t.Errorf("Expected 8 of spades to be valid in ASCENDING mode, got 0 moves")
	}
}

// TestSequenceModeDescending verifies descending sequence rules
func TestSequenceModeDescending(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 3       // SEQUENCE
	state.SequenceDirection = 1 // DESCENDING
	state.NumPlayers = 2

	// Tableau has 7 of spades
	state.Tableau = make([][]Card, 4)
	state.Tableau[0] = []Card{{Rank: 7, Suit: 0}}
	state.Tableau[1] = []Card{}
	state.Tableau[2] = []Card{}
	state.Tableau[3] = []Card{}

	// Player has: 6 (valid descending), 8 (invalid ascending)
	state.Players[0].Hand = []Card{
		{Rank: 6, Suit: 0}, // Valid - descending from 7
		{Rank: 8, Suit: 0}, // Invalid - ascending
	}
	state.CurrentPlayer = 0

	genome := sequencePhaseGenome()
	moves := GenerateLegalMoves(state, genome)

	// The 6 of spades should be valid on the spade pile (descending from 7)
	validSixOfSpades := false
	for _, m := range moves {
		if m.TargetLoc == LocationTableau {
			card := state.Players[0].Hand[m.CardIndex]
			if card.Suit == 0 && card.Rank == 6 {
				validSixOfSpades = true
			}
		}
	}

	if !validSixOfSpades {
		t.Errorf("Expected 6 of spades to be valid in DESCENDING mode")
	}
}

// TestSequenceModeBoth verifies bidirectional sequence rules
func TestSequenceModeBoth(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 3       // SEQUENCE
	state.SequenceDirection = 2 // BOTH
	state.NumPlayers = 2

	// Tableau has 7 of spades
	state.Tableau = make([][]Card, 4)
	state.Tableau[0] = []Card{{Rank: 7, Suit: 0}}
	state.Tableau[1] = []Card{}
	state.Tableau[2] = []Card{}
	state.Tableau[3] = []Card{}

	// Player has: 6 (valid desc), 8 (valid asc)
	state.Players[0].Hand = []Card{
		{Rank: 6, Suit: 0}, // Valid - descending from 7
		{Rank: 8, Suit: 0}, // Valid - ascending from 7
	}
	state.CurrentPlayer = 0

	genome := sequencePhaseGenome()
	moves := GenerateLegalMoves(state, genome)

	// Both 6 and 8 of spades should be valid with BOTH direction
	validSix := false
	validEight := false
	for _, m := range moves {
		if m.TargetLoc == LocationTableau {
			card := state.Players[0].Hand[m.CardIndex]
			if card.Suit == 0 && card.Rank == 6 {
				validSix = true
			}
			if card.Suit == 0 && card.Rank == 8 {
				validEight = true
			}
		}
	}

	if !validSix {
		t.Errorf("Expected 6 of spades to be valid in BOTH direction mode")
	}
	if !validEight {
		t.Errorf("Expected 8 of spades to be valid in BOTH direction mode")
	}
}

// TestSequenceModeBoundaryKing verifies that K can't go higher in ascending mode
func TestSequenceModeBoundaryKing(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 3       // SEQUENCE
	state.SequenceDirection = 0 // ASCENDING
	state.NumPlayers = 2

	// Tableau has King (rank 13) - can't go higher
	state.Tableau = make([][]Card, 4)
	state.Tableau[0] = []Card{{Rank: 13, Suit: 0}} // King of spades
	state.Tableau[1] = []Card{}
	state.Tableau[2] = []Card{}
	state.Tableau[3] = []Card{}

	// Player has Ace (rank 14 in our system) - can't play on King in ascending
	state.Players[0].Hand = []Card{
		{Rank: 14, Suit: 0}, // Ace of spades
	}
	state.CurrentPlayer = 0

	genome := sequencePhaseGenome()
	moves := GenerateLegalMoves(state, genome)

	// Count moves that play Ace on the spade pile specifically (should be 0)
	// The Ace can start a new pile on empty slots, but shouldn't wrap around King
	aceOnKingPile := false
	for _, m := range moves {
		if m.TargetLoc == LocationTableau {
			card := state.Players[0].Hand[m.CardIndex]
			// For sequence mode with pile-specific targeting, this would need to check
			// if the move is specifically for pile 0. For now, check that we have
			// at least some moves (Ace can start new piles on empty slots)
			if card.Rank == 14 && card.Suit == 0 {
				// This is tricky - the current design may not track which pile
				// Let's just verify the helper function rejects King->Ace
				_ = card
			}
		}
	}

	// Verify the helper function directly
	kingCard := Card{Rank: 13, Suit: 0}
	aceCard := Card{Rank: 14, Suit: 0}

	if isValidSequencePlay(aceCard, kingCard, 0) { // ASCENDING
		aceOnKingPile = true
	}

	if aceOnKingPile {
		t.Errorf("Ace should NOT be playable on King in ASCENDING mode (no wrapping)")
	}
}

// TestSequenceModeBoundaryAce verifies that A can't go lower in descending mode
func TestSequenceModeBoundaryAce(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 3       // SEQUENCE
	state.SequenceDirection = 1 // DESCENDING
	state.NumPlayers = 2

	// For ranks where 2 is the lowest playable rank
	// Ace (14) is high, so descending from 2 has no valid play

	// Tableau has 2 (rank 2) - can't go lower
	state.Tableau = make([][]Card, 4)
	state.Tableau[0] = []Card{{Rank: 2, Suit: 0}} // 2 of spades
	state.Tableau[1] = []Card{}
	state.Tableau[2] = []Card{}
	state.Tableau[3] = []Card{}

	// Player has Ace - in typical card games, Ace is high (14)
	// So descending from 2 would need rank 1, which doesn't exist
	state.Players[0].Hand = []Card{
		{Rank: 14, Suit: 0}, // Ace of spades (high)
	}
	state.CurrentPlayer = 0

	// Verify the helper function directly - 2 descending has no valid lower card
	twoCard := Card{Rank: 2, Suit: 0}
	aceCard := Card{Rank: 14, Suit: 0}

	// Ace (14) is not rank 1, so it shouldn't be valid descending from 2
	if isValidSequencePlay(aceCard, twoCard, 1) { // DESCENDING
		t.Errorf("Ace (rank 14) should NOT be valid descending from 2 (would need rank 1)")
	}
}

// TestSequenceModeSuitMatching verifies that cards must match suit
func TestSequenceModeSuitMatching(t *testing.T) {
	state := NewGameState(2)
	state.TableauMode = 3       // SEQUENCE
	state.SequenceDirection = 0 // ASCENDING
	state.NumPlayers = 2

	// Tableau has 7 of spades
	state.Tableau = make([][]Card, 4)
	state.Tableau[0] = []Card{{Rank: 7, Suit: 0}} // 7 of spades
	state.Tableau[1] = []Card{}
	state.Tableau[2] = []Card{}
	state.Tableau[3] = []Card{}

	// Verify the helper function - 8 of hearts should NOT play on spades pile
	sevenSpades := Card{Rank: 7, Suit: 0}
	eightHearts := Card{Rank: 8, Suit: 1}
	eightSpades := Card{Rank: 8, Suit: 0}

	if isValidSequencePlay(eightHearts, sevenSpades, 0) {
		t.Errorf("8 of hearts should NOT be valid on 7 of spades (wrong suit)")
	}

	if !isValidSequencePlay(eightSpades, sevenSpades, 0) {
		t.Errorf("8 of spades SHOULD be valid on 7 of spades (same suit, ascending)")
	}
}

// Helper to create genome with SEQUENCE-compatible play phase
func sequencePhaseGenome() *Genome {
	return &Genome{
		Header: &BytecodeHeader{
			PlayerCount:       2,
			TableauMode:       3, // SEQUENCE
			SequenceDirection: 0, // ASCENDING (can be overridden by state)
		},
		TurnPhases: []PhaseDescriptor{
			{
				PhaseType: 2, // PlayPhase
				Data: []byte{
					byte(LocationTableau), // target = TABLEAU
					1,                      // min_cards = 1
					1,                      // max_cards = 1
					0,                      // mandatory = false
					1,                      // pass_if_unable = true
					0, 0, 0, 0,             // conditionLen = 0 (no condition)
				},
			},
		},
		WinConditions: []WinCondition{
			{WinType: 0, Threshold: 0}, // empty_hand
		},
	}
}

// Helper to create minimal genome with play phase targeting tableau
func minimalPlayPhaseGenome() *Genome {
	// Create a minimal genome with a play phase that targets tableau
	// Phase type 2 = PlayPhase
	// Data format: target:1 + min:1 + max:1 + mandatory:1 + pass_if_unable:1 + conditionLen:4
	genome := &Genome{
		Header: &BytecodeHeader{
			PlayerCount: 2,
		},
		TurnPhases: []PhaseDescriptor{
			{
				PhaseType: 2, // PlayPhase
				Data: []byte{
					byte(LocationTableau), // target = TABLEAU
					1,                      // min_cards = 1
					1,                      // max_cards = 1
					1,                      // mandatory = true
					0,                      // pass_if_unable = false
					0, 0, 0, 0,             // conditionLen = 0 (no condition)
				},
			},
		},
		WinConditions: []WinCondition{
			{WinType: 3, Threshold: 52}, // capture_all
		},
	}
	return genome
}

// TestCalculateTrickPointsExplicitScoring verifies that explicit CardScoring
// rules are used when available
func TestCalculateTrickPointsExplicitScoring(t *testing.T) {
	state := NewGameState(2)

	// Create genome with explicit scoring: Diamonds = 2 points
	genome := &Genome{
		CardScoring: []CardScoringRule{
			{Suit: 1, Rank: 255, Points: 2, Trigger: TriggerTrickWin}, // Diamonds = 2 points
		},
	}

	// Simulate trick with a Diamond
	state.CurrentTrick = []TrickCard{
		{Card: Card{Rank: 5, Suit: 1}, PlayerID: 0}, // 5 of Diamonds
	}

	points := calculateTrickPoints(state, genome, 255)

	if points != 2 {
		t.Errorf("Expected 2 points for Diamond, got %d", points)
	}
}

// TestCalculateTrickPointsExplicitScoringMultipleRules verifies that multiple
// scoring rules are applied correctly
func TestCalculateTrickPointsExplicitScoringMultipleRules(t *testing.T) {
	state := NewGameState(2)

	// Create genome with Hearts-style explicit scoring
	genome := &Genome{
		CardScoring: []CardScoringRule{
			{Suit: 0, Rank: 255, Points: 1, Trigger: TriggerTrickWin},  // Hearts = 1 point
			{Suit: 3, Rank: 10, Points: 13, Trigger: TriggerTrickWin},  // Queen of Spades = 13 points
		},
	}

	// Simulate trick with Queen of Spades and a Heart
	state.CurrentTrick = []TrickCard{
		{Card: Card{Rank: 10, Suit: 3}, PlayerID: 0}, // Queen of Spades
		{Card: Card{Rank: 5, Suit: 0}, PlayerID: 1},  // 5 of Hearts
	}

	points := calculateTrickPoints(state, genome, 255)

	// QS = 13, Heart = 1, total = 14
	if points != 14 {
		t.Errorf("Expected 14 points (QS + Heart), got %d", points)
	}
}

// TestCalculateTrickPointsExplicitScoringSpecificRank verifies scoring rules
// that match specific rank and suit
func TestCalculateTrickPointsExplicitScoringSpecificRank(t *testing.T) {
	state := NewGameState(2)

	// Create genome with specific card scoring: 10 of Clubs = 5 points
	genome := &Genome{
		CardScoring: []CardScoringRule{
			{Suit: 2, Rank: 8, Points: 5, Trigger: TriggerTrickWin}, // 10 of Clubs = 5 points (rank 8 = 10)
		},
	}

	// Trick with 10 of Clubs
	state.CurrentTrick = []TrickCard{
		{Card: Card{Rank: 8, Suit: 2}, PlayerID: 0}, // 10 of Clubs
	}

	points := calculateTrickPoints(state, genome, 255)

	if points != 5 {
		t.Errorf("Expected 5 points for 10 of Clubs, got %d", points)
	}

	// Trick without 10 of Clubs
	state.CurrentTrick = []TrickCard{
		{Card: Card{Rank: 7, Suit: 2}, PlayerID: 0}, // 9 of Clubs (not 10)
	}

	points = calculateTrickPoints(state, genome, 255)

	if points != 0 {
		t.Errorf("Expected 0 points for 9 of Clubs (no rule match), got %d", points)
	}
}

// TestCalculateTrickPointsFallbackScoring verifies that fallback Hearts scoring
// is used when no explicit CardScoring rules exist
func TestCalculateTrickPointsFallbackScoring(t *testing.T) {
	state := NewGameState(2)

	// Create genome with NO explicit scoring
	genome := &Genome{
		CardScoring: nil, // No explicit rules
	}

	// Trick with Queen of Spades and a Heart (suit 0)
	state.CurrentTrick = []TrickCard{
		{Card: Card{Rank: 10, Suit: 3}, PlayerID: 0}, // Queen of Spades
		{Card: Card{Rank: 5, Suit: 0}, PlayerID: 1},  // 5 of Hearts
	}

	// breakingSuit = 0 (Hearts)
	points := calculateTrickPoints(state, genome, 0)

	// QS = 13 (hardcoded), Heart = 1 (breaking suit)
	if points != 14 {
		t.Errorf("Expected 14 points from fallback scoring (QS + Heart), got %d", points)
	}
}

// TestCalculateTrickPointsIgnoresNonTrickTriggers verifies that only TRICK_WIN
// triggered rules are applied
func TestCalculateTrickPointsIgnoresNonTrickTriggers(t *testing.T) {
	state := NewGameState(2)

	// Create genome with mixed triggers - only TRICK_WIN should apply
	genome := &Genome{
		CardScoring: []CardScoringRule{
			{Suit: 1, Rank: 255, Points: 2, Trigger: TriggerTrickWin}, // Diamonds on trick win
			{Suit: 0, Rank: 255, Points: 5, Trigger: TriggerCapture},  // Hearts on capture (should NOT apply)
		},
	}

	// Trick with Diamond and Heart
	state.CurrentTrick = []TrickCard{
		{Card: Card{Rank: 5, Suit: 1}, PlayerID: 0}, // 5 of Diamonds
		{Card: Card{Rank: 3, Suit: 0}, PlayerID: 1}, // 5 of Hearts
	}

	points := calculateTrickPoints(state, genome, 255)

	// Only Diamond rule applies (TRICK_WIN trigger), Heart rule is CAPTURE trigger
	if points != 2 {
		t.Errorf("Expected 2 points (only Diamond TRICK_WIN rule), got %d", points)
	}
}

// TestCalculateTrickPointsNegativePoints verifies that negative points work
func TestCalculateTrickPointsNegativePoints(t *testing.T) {
	state := NewGameState(2)

	// Create genome with negative scoring
	genome := &Genome{
		CardScoring: []CardScoringRule{
			{Suit: 2, Rank: 255, Points: -3, Trigger: TriggerTrickWin}, // Clubs = -3 points
		},
	}

	// Trick with two Clubs
	state.CurrentTrick = []TrickCard{
		{Card: Card{Rank: 5, Suit: 2}, PlayerID: 0}, // 7 of Clubs
		{Card: Card{Rank: 3, Suit: 2}, PlayerID: 1}, // 5 of Clubs
	}

	points := calculateTrickPoints(state, genome, 255)

	// Two clubs at -3 each = -6
	if points != -6 {
		t.Errorf("Expected -6 points for two Clubs, got %d", points)
	}
}

// =========================================================================
// Team Win Condition Tests
// =========================================================================

// TestCheckWinConditionsTeamWinEmptyHand verifies that when a player on a team
// wins by emptying their hand, the team is also marked as winner
func TestCheckWinConditionsTeamWinEmptyHand(t *testing.T) {
	state := NewGameState(4)
	state.NumPlayers = 4

	// Setup teams: Team 0 = Players 0,2; Team 1 = Players 1,3
	state.PlayerToTeam = []int8{0, 1, 0, 1}
	state.TeamScores = []int32{0, 0}
	state.WinnerID = -1
	state.WinningTeam = -1

	// Player 2 (Team 0) has empty hand
	state.Players[0].Hand = []Card{{Rank: 5, Suit: 0}}
	state.Players[1].Hand = []Card{{Rank: 6, Suit: 1}}
	state.Players[2].Hand = []Card{} // Empty - should win!
	state.Players[3].Hand = []Card{{Rank: 7, Suit: 2}}

	genome := &Genome{
		WinConditions: []WinCondition{
			{WinType: 0, Threshold: 0}, // empty_hand wins
		},
	}

	winner := CheckWinConditions(state, genome)

	// Player 2 should win
	if winner != 2 {
		t.Errorf("Expected player 2 to win (empty hand), got %d", winner)
	}

	// WinningTeam should be set to player 2's team (Team 0)
	if state.WinningTeam != 0 {
		t.Errorf("Expected WinningTeam to be 0 (player 2's team), got %d", state.WinningTeam)
	}
}

// TestCheckWinConditionsTeamWinHighScore verifies score-based team wins
func TestCheckWinConditionsTeamWinHighScore(t *testing.T) {
	state := NewGameState(4)
	state.NumPlayers = 4

	// Setup teams: Team 0 = Players 0,2; Team 1 = Players 1,3
	state.PlayerToTeam = []int8{0, 1, 0, 1}
	state.TeamScores = []int32{30, 20} // Team 0 has higher score
	state.WinnerID = -1
	state.WinningTeam = -1

	// Individual scores (team 0 members have higher total)
	state.Players[0].Score = 15
	state.Players[1].Score = 10
	state.Players[2].Score = 15
	state.Players[3].Score = 10

	// Give everyone cards so empty_hand doesn't trigger
	for i := 0; i < 4; i++ {
		state.Players[i].Hand = []Card{{Rank: uint8(i + 2), Suit: 0}}
	}

	genome := &Genome{
		WinConditions: []WinCondition{
			{WinType: 1, Threshold: 10}, // high_score with threshold 10
		},
	}

	winner := CheckWinConditions(state, genome)

	// Player 0 or 2 should win (highest individual score, both have 15)
	// The first player checked with highest score wins
	if winner != 0 {
		t.Errorf("Expected player 0 to win (highest score first checked), got %d", winner)
	}

	// WinningTeam should be set to the winner's team (Team 0)
	if state.WinningTeam != 0 {
		t.Errorf("Expected WinningTeam to be 0 (player 0's team), got %d", state.WinningTeam)
	}
}

// TestCheckWinConditionsNoTeams verifies that non-team games still work
// and WinningTeam stays -1
func TestCheckWinConditionsNoTeams(t *testing.T) {
	state := NewGameState(2)
	state.NumPlayers = 2

	// No team configuration
	state.PlayerToTeam = nil
	state.TeamScores = nil
	state.WinnerID = -1
	state.WinningTeam = -1

	// Player 1 has empty hand
	state.Players[0].Hand = []Card{{Rank: 5, Suit: 0}}
	state.Players[1].Hand = []Card{} // Empty - should win!

	genome := &Genome{
		WinConditions: []WinCondition{
			{WinType: 0, Threshold: 0}, // empty_hand wins
		},
	}

	winner := CheckWinConditions(state, genome)

	// Player 1 should win
	if winner != 1 {
		t.Errorf("Expected player 1 to win (empty hand), got %d", winner)
	}

	// WinningTeam should stay -1 (no teams configured)
	if state.WinningTeam != -1 {
		t.Errorf("Expected WinningTeam to be -1 (no teams), got %d", state.WinningTeam)
	}
}

// TestCheckWinConditionsTeamWinLowScore verifies low score wins (like Hearts)
func TestCheckWinConditionsTeamWinLowScore(t *testing.T) {
	state := NewGameState(4)
	state.NumPlayers = 4

	// Setup teams
	state.PlayerToTeam = []int8{0, 1, 0, 1}
	state.TeamScores = []int32{10, 50} // Team 0 has lower score
	state.WinnerID = -1
	state.WinningTeam = -1

	// Individual scores - player 0 has lowest
	state.Players[0].Score = 5
	state.Players[1].Score = 25
	state.Players[2].Score = 5
	state.Players[3].Score = 25

	// One player crosses threshold to trigger win check
	state.Players[1].Score = 100 // Triggers end

	// Give everyone cards
	for i := 0; i < 4; i++ {
		state.Players[i].Hand = []Card{{Rank: uint8(i + 2), Suit: 0}}
	}

	genome := &Genome{
		WinConditions: []WinCondition{
			{WinType: 4, Threshold: 100}, // low_score wins when someone hits 100
		},
	}

	winner := CheckWinConditions(state, genome)

	// Player 0 should win (lowest score = 5)
	if winner != 0 {
		t.Errorf("Expected player 0 to win (lowest score), got %d", winner)
	}

	// WinningTeam should be player 0's team (Team 0)
	if state.WinningTeam != 0 {
		t.Errorf("Expected WinningTeam to be 0 (player 0's team), got %d", state.WinningTeam)
	}
}

// TestCheckWinConditionsTeamWinFirstToScore verifies first-to-threshold wins
func TestCheckWinConditionsTeamWinFirstToScore(t *testing.T) {
	state := NewGameState(4)
	state.NumPlayers = 4

	// Setup teams: Team 0 = Players 0,2; Team 1 = Players 1,3
	state.PlayerToTeam = []int8{0, 1, 0, 1}
	state.TeamScores = []int32{0, 0}
	state.WinnerID = -1
	state.WinningTeam = -1

	// Player 3 (Team 1) reaches threshold first
	state.Players[0].Score = 5
	state.Players[1].Score = 8
	state.Players[2].Score = 7
	state.Players[3].Score = 10 // First to 10!

	// Give everyone cards
	for i := 0; i < 4; i++ {
		state.Players[i].Hand = []Card{{Rank: uint8(i + 2), Suit: 0}}
	}

	genome := &Genome{
		WinConditions: []WinCondition{
			{WinType: 2, Threshold: 10}, // first_to_score 10
		},
	}

	winner := CheckWinConditions(state, genome)

	// Player 3 should win (first to reach 10)
	if winner != 3 {
		t.Errorf("Expected player 3 to win (first to 10), got %d", winner)
	}

	// WinningTeam should be player 3's team (Team 1)
	if state.WinningTeam != 1 {
		t.Errorf("Expected WinningTeam to be 1 (player 3's team), got %d", state.WinningTeam)
	}
}

// TestCheckWinConditionsTeamWinCaptureAll verifies capture-all wins (War)
func TestCheckWinConditionsTeamWinCaptureAll(t *testing.T) {
	state := NewGameState(4)
	state.NumPlayers = 4

	// Setup teams
	state.PlayerToTeam = []int8{0, 1, 0, 1}
	state.TeamScores = []int32{0, 0}
	state.WinnerID = -1
	state.WinningTeam = -1

	// Player 2 (Team 0) has all 52 cards
	state.Players[0].Hand = []Card{}
	state.Players[1].Hand = []Card{}
	state.Players[2].Hand = make([]Card, 52) // All 52 cards!
	state.Players[3].Hand = []Card{}

	genome := &Genome{
		WinConditions: []WinCondition{
			{WinType: 3, Threshold: 52}, // capture_all
		},
	}

	winner := CheckWinConditions(state, genome)

	// Player 2 should win
	if winner != 2 {
		t.Errorf("Expected player 2 to win (has all cards), got %d", winner)
	}

	// WinningTeam should be player 2's team (Team 0)
	if state.WinningTeam != 0 {
		t.Errorf("Expected WinningTeam to be 0 (player 2's team), got %d", state.WinningTeam)
	}
}

// TestCheckWinConditionsTeamWinAllHandsEmpty verifies trick-taking hand end
func TestCheckWinConditionsTeamWinAllHandsEmpty(t *testing.T) {
	state := NewGameState(4)
	state.NumPlayers = 4

	// Setup teams
	state.PlayerToTeam = []int8{0, 1, 0, 1}
	state.TeamScores = []int32{0, 0}
	state.WinnerID = -1
	state.WinningTeam = -1

	// All hands empty - player with lowest score wins
	state.Players[0].Hand = []Card{}
	state.Players[1].Hand = []Card{}
	state.Players[2].Hand = []Card{}
	state.Players[3].Hand = []Card{}

	// Scores: player 1 has lowest
	state.Players[0].Score = 20
	state.Players[1].Score = 5 // Lowest!
	state.Players[2].Score = 15
	state.Players[3].Score = 10

	genome := &Genome{
		WinConditions: []WinCondition{
			{WinType: 5, Threshold: 0}, // all_hands_empty
		},
	}

	winner := CheckWinConditions(state, genome)

	// Player 1 should win (lowest score when all hands empty)
	if winner != 1 {
		t.Errorf("Expected player 1 to win (lowest score), got %d", winner)
	}

	// WinningTeam should be player 1's team (Team 1)
	if state.WinningTeam != 1 {
		t.Errorf("Expected WinningTeam to be 1 (player 1's team), got %d", state.WinningTeam)
	}
}

// TestCheckWinConditionsTeamPlayerOutOfBounds verifies safe handling of edge cases
func TestCheckWinConditionsTeamPlayerOutOfBounds(t *testing.T) {
	state := NewGameState(2)
	state.NumPlayers = 2

	// PlayerToTeam has fewer entries than NumPlayers (edge case)
	state.PlayerToTeam = []int8{0} // Only player 0 mapped
	state.TeamScores = []int32{0}
	state.WinnerID = -1
	state.WinningTeam = -1

	// Player 1 wins but isn't in PlayerToTeam
	state.Players[0].Hand = []Card{{Rank: 5, Suit: 0}}
	state.Players[1].Hand = []Card{} // Wins!

	genome := &Genome{
		WinConditions: []WinCondition{
			{WinType: 0, Threshold: 0}, // empty_hand wins
		},
	}

	winner := CheckWinConditions(state, genome)

	// Player 1 should win
	if winner != 1 {
		t.Errorf("Expected player 1 to win, got %d", winner)
	}

	// WinningTeam should stay -1 (player 1 not in PlayerToTeam)
	if state.WinningTeam != -1 {
		t.Errorf("Expected WinningTeam to be -1 (player not in team map), got %d", state.WinningTeam)
	}
}

// =========================================================================
// Dual Scoring Tests - Individual AND Team Scores
// =========================================================================

// TestDualScoringBasic verifies that when a player scores, their team also scores
func TestDualScoringBasic(t *testing.T) {
	state := &GameState{
		NumPlayers: 4,
		Players: []PlayerState{
			{Score: 0}, {Score: 0}, {Score: 0}, {Score: 0},
		},
		TeamScores:   []int32{0, 0},
		PlayerToTeam: []int8{0, 1, 0, 1}, // Players 0,2 on team 0; Players 1,3 on team 1
		WinningTeam:  -1,
	}

	// Simulate player 0 scoring 10 points
	state.Players[0].Score += 10
	UpdateTeamScore(state, 0, 10)

	if state.TeamScores[0] != 10 {
		t.Errorf("Team 0 score should be 10, got %d", state.TeamScores[0])
	}
	if state.TeamScores[1] != 0 {
		t.Errorf("Team 1 score should be 0, got %d", state.TeamScores[1])
	}
}

// TestDualScoringMultiplePlayers verifies multiple players on same team accumulate scores
func TestDualScoringMultiplePlayers(t *testing.T) {
	state := &GameState{
		NumPlayers: 4,
		Players: []PlayerState{
			{Score: 0}, {Score: 0}, {Score: 0}, {Score: 0},
		},
		TeamScores:   []int32{0, 0},
		PlayerToTeam: []int8{0, 1, 0, 1},
		WinningTeam:  -1,
	}

	// Player 0 (team 0) scores 10
	state.Players[0].Score += 10
	UpdateTeamScore(state, 0, 10)

	// Player 2 (team 0) scores 5
	state.Players[2].Score += 5
	UpdateTeamScore(state, 2, 5)

	// Player 1 (team 1) scores 7
	state.Players[1].Score += 7
	UpdateTeamScore(state, 1, 7)

	if state.TeamScores[0] != 15 {
		t.Errorf("Team 0 score should be 15, got %d", state.TeamScores[0])
	}
	if state.TeamScores[1] != 7 {
		t.Errorf("Team 1 score should be 7, got %d", state.TeamScores[1])
	}
}

// TestDualScoringNoTeams verifies no panic when teams not configured
func TestDualScoringNoTeams(t *testing.T) {
	state := &GameState{
		NumPlayers: 2,
		Players:    []PlayerState{{Score: 0}, {Score: 0}},
		// No team fields - nil slices
	}

	// Should not panic when teams not configured
	state.Players[0].Score += 10
	UpdateTeamScore(state, 0, 10) // Should be no-op

	// Just verify no panic occurred
	if state.TeamScores != nil {
		t.Error("TeamScores should remain nil")
	}
}

// TestDualScoringNegative verifies negative points work correctly
func TestDualScoringNegative(t *testing.T) {
	state := &GameState{
		NumPlayers: 4,
		Players: []PlayerState{
			{Score: 20}, {Score: 10}, {Score: 20}, {Score: 10},
		},
		TeamScores:   []int32{40, 20}, // Team 0: 40, Team 1: 20
		PlayerToTeam: []int8{0, 1, 0, 1},
		WinningTeam:  -1,
	}

	// Player 0 (team 0) loses 5 points
	state.Players[0].Score -= 5
	UpdateTeamScore(state, 0, -5)

	if state.TeamScores[0] != 35 {
		t.Errorf("Team 0 score should be 35, got %d", state.TeamScores[0])
	}
}

// TestDualScoringPlayerOutOfBounds verifies bounds checking
func TestDualScoringPlayerOutOfBounds(t *testing.T) {
	state := &GameState{
		NumPlayers: 2,
		Players:    []PlayerState{{Score: 0}, {Score: 0}},
		TeamScores:   []int32{0, 0},
		PlayerToTeam: []int8{0, 1},
		WinningTeam:  -1,
	}

	// Should not panic with invalid player index
	UpdateTeamScore(state, -1, 10) // Negative index
	UpdateTeamScore(state, 5, 10)  // Out of bounds

	// Scores should remain unchanged
	if state.TeamScores[0] != 0 || state.TeamScores[1] != 0 {
		t.Error("Team scores should remain 0 after invalid player index")
	}
}

// TestDualScoringIntegrationTrickWin verifies trick win updates team scores
func TestDualScoringIntegrationTrickWin(t *testing.T) {
	state := NewGameState(4)
	state.NumPlayers = 4
	state.PlayerToTeam = []int8{0, 1, 0, 1}
	state.TeamScores = []int32{0, 0}
	state.WinningTeam = -1

	// Setup 4 players with cards
	state.Players[0].Hand = []Card{{Rank: 10, Suit: 0}} // 10 of Hearts - wins
	state.Players[1].Hand = []Card{{Rank: 5, Suit: 0}}  // 5 of Hearts
	state.Players[2].Hand = []Card{{Rank: 3, Suit: 0}}  // 3 of Hearts
	state.Players[3].Hand = []Card{{Rank: 2, Suit: 0}}  // 2 of Hearts

	state.TricksWon = make([]uint8, 4)

	// Create a trick with hearts (1 point each for breaking suit in fallback scoring)
	state.CurrentTrick = []TrickCard{
		{PlayerID: 0, Card: Card{Rank: 10, Suit: 0}}, // Player 0 wins
		{PlayerID: 1, Card: Card{Rank: 5, Suit: 0}},
		{PlayerID: 2, Card: Card{Rank: 3, Suit: 0}},
		{PlayerID: 3, Card: Card{Rank: 2, Suit: 0}},
	}

	// Create genome with trick phase and fallback scoring
	genome := &Genome{
		TurnPhases: []PhaseDescriptor{
			{
				PhaseType: 4, // TrickPhase
				Data:      []byte{1, 255, 1, 0}, // lead_suit_required, no trump, high wins, hearts breaking
			},
		},
	}

	// Resolve the trick - player 0 should win
	resolveTrick(state, genome, genome.TurnPhases[0])

	// Player 0 wins 4 hearts = 4 points
	if state.Players[0].Score != 4 {
		t.Errorf("Player 0 should have 4 points, got %d", state.Players[0].Score)
	}

	// Team 0 (players 0 and 2) should also have 4 points
	if state.TeamScores[0] != 4 {
		t.Errorf("Team 0 should have 4 points, got %d", state.TeamScores[0])
	}

	// Team 1 should have 0 points
	if state.TeamScores[1] != 0 {
		t.Errorf("Team 1 should have 0 points, got %d", state.TeamScores[1])
	}
}

// TestDualScoringIntegrationMatchRankCapture verifies match rank capture updates team scores
func TestDualScoringIntegrationMatchRankCapture(t *testing.T) {
	state := NewGameState(4)
	state.NumPlayers = 4
	state.TableauMode = 2 // MATCH_RANK
	state.PlayerToTeam = []int8{0, 1, 0, 1}
	state.TeamScores = []int32{0, 0}
	state.WinningTeam = -1

	// Setup: card on tableau to match
	state.Tableau = make([][]Card, 1)
	state.Tableau[0] = []Card{{Rank: 7, Suit: 1}} // 7 of different suit

	// Player 2 (team 0) has a 7 that can capture
	state.Players[2].Hand = []Card{{Rank: 7, Suit: 0}}
	state.CurrentPlayer = 2

	genome := minimalPlayPhaseGenome()
	genome.Header.TableauMode = 2

	move := LegalMove{PhaseIndex: 0, CardIndex: 0, TargetLoc: LocationTableau}
	ApplyMove(state, &move, genome)

	// Player 2 should have scored 2 points for the capture
	if state.Players[2].Score != 2 {
		t.Errorf("Player 2 should have score 2, got %d", state.Players[2].Score)
	}

	// Team 0 should also have 2 points
	if state.TeamScores[0] != 2 {
		t.Errorf("Team 0 should have 2 points, got %d", state.TeamScores[0])
	}

	// Team 1 should have 0 points
	if state.TeamScores[1] != 0 {
		t.Errorf("Team 1 should have 0 points, got %d", state.TeamScores[1])
	}
}

// TestDualScoringIntegrationGoFishSet verifies Go Fish set scoring updates team scores
func TestDualScoringIntegrationGoFishSet(t *testing.T) {
	state := NewGameState(4)
	state.NumPlayers = 4
	state.PlayerToTeam = []int8{0, 1, 0, 1}
	state.TeamScores = []int32{0, 0}
	state.WinningTeam = -1

	// Player 1 (team 1) has 4 cards of same rank (a set)
	state.Players[1].Hand = []Card{
		{Rank: 7, Suit: 0},
		{Rank: 7, Suit: 1},
		{Rank: 7, Suit: 2},
		{Rank: 7, Suit: 3},
	}
	state.CurrentPlayer = 1

	// Create genome with multi-card play phase (Go Fish style)
	genome := &Genome{
		Header: &BytecodeHeader{PlayerCount: 4},
		TurnPhases: []PhaseDescriptor{
			{
				PhaseType: 2, // PlayPhase
				Data: []byte{
					byte(LocationDiscard), // target = DISCARD
					4,                      // min_cards = 4
					4,                      // max_cards = 4
					0,                      // mandatory = false
					0,                      // pass_if_unable = false
					0, 0, 0, 0,             // conditionLen = 0
				},
			},
		},
	}

	// Play the set (encoded as negative rank - 100)
	move := LegalMove{PhaseIndex: 0, CardIndex: -107, TargetLoc: LocationDiscard} // -107 = -(7 + 100)
	ApplyMove(state, &move, genome)

	// Player 1 should have 1 point for completing a set
	if state.Players[1].Score != 1 {
		t.Errorf("Player 1 should have 1 point for set, got %d", state.Players[1].Score)
	}

	// Team 1 should also have 1 point
	if state.TeamScores[1] != 1 {
		t.Errorf("Team 1 should have 1 point, got %d", state.TeamScores[1])
	}

	// Team 0 should have 0 points
	if state.TeamScores[0] != 0 {
		t.Errorf("Team 0 should have 0 points, got %d", state.TeamScores[0])
	}
}

// =========================================================================
// Bid Move Generation Tests
// =========================================================================

func TestGenerateBidMoves(t *testing.T) {
	phase := BiddingPhase{MinBid: 1, MaxBid: 13, AllowNil: true}
	handSize := 13

	moves := GenerateBidMoves(phase, handSize)

	// Should have 14 moves: Nil (0) + bids 1-13
	if len(moves) != 14 {
		t.Errorf("Expected 14 moves, got %d", len(moves))
	}

	// First move should be Nil
	if moves[0].Value != 0 || !moves[0].IsNil {
		t.Errorf("First move should be Nil bid")
	}

	// Last move should be bid 13
	if moves[13].Value != 13 {
		t.Errorf("Last move should be bid 13, got %d", moves[13].Value)
	}
}

func TestGenerateBidMovesNoNil(t *testing.T) {
	phase := BiddingPhase{MinBid: 1, MaxBid: 13, AllowNil: false}
	handSize := 13

	moves := GenerateBidMoves(phase, handSize)

	// Should have 13 moves: bids 1-13 (no Nil)
	if len(moves) != 13 {
		t.Errorf("Expected 13 moves, got %d", len(moves))
	}
}

func TestGenerateBidMovesHandSizeLimit(t *testing.T) {
	phase := BiddingPhase{MinBid: 1, MaxBid: 13, AllowNil: true}
	handSize := 5 // Only 5 cards in hand

	moves := GenerateBidMoves(phase, handSize)

	// Should have 6 moves: Nil (0) + bids 1-5
	if len(moves) != 6 {
		t.Errorf("Expected 6 moves, got %d", len(moves))
	}
}
