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
