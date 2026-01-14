package engine

import "testing"

func TestCalculateBlackjackValue_SimpleHand(t *testing.T) {
	// 10 + 7 = 17
	cards := []Card{
		{Rank: 9, Suit: 0},  // 10 (rank 9 = 10)
		{Rank: 6, Suit: 1},  // 7 (rank 6 = 7)
	}
	value := CalculateBlackjackValue(cards)
	if value != 17 {
		t.Errorf("Expected 17, got %d", value)
	}
}

func TestCalculateBlackjackValue_FaceCards(t *testing.T) {
	// K + Q = 20
	cards := []Card{
		{Rank: 12, Suit: 0}, // K (rank 12)
		{Rank: 11, Suit: 1}, // Q (rank 11)
	}
	value := CalculateBlackjackValue(cards)
	if value != 20 {
		t.Errorf("Expected 20, got %d", value)
	}
}

func TestCalculateBlackjackValue_AceAsEleven(t *testing.T) {
	// A + 7 = 18 (Ace counts as 11)
	cards := []Card{
		{Rank: 0, Suit: 0},  // Ace (rank 0)
		{Rank: 6, Suit: 1},  // 7 (rank 6 = 7)
	}
	value := CalculateBlackjackValue(cards)
	if value != 18 {
		t.Errorf("Expected 18, got %d", value)
	}
}

func TestCalculateBlackjackValue_AceAsOne(t *testing.T) {
	// A + 10 + 5 = 16 (Ace counts as 1 to avoid bust)
	cards := []Card{
		{Rank: 0, Suit: 0},  // Ace (rank 0)
		{Rank: 9, Suit: 1},  // 10 (rank 9 = 10)
		{Rank: 4, Suit: 2},  // 5 (rank 4 = 5)
	}
	value := CalculateBlackjackValue(cards)
	if value != 16 {
		t.Errorf("Expected 16, got %d", value)
	}
}

func TestCalculateBlackjackValue_Blackjack(t *testing.T) {
	// A + K = 21 (Natural blackjack)
	cards := []Card{
		{Rank: 0, Suit: 0},  // Ace (rank 0)
		{Rank: 12, Suit: 1}, // K (rank 12)
	}
	value := CalculateBlackjackValue(cards)
	if value != 21 {
		t.Errorf("Expected 21, got %d", value)
	}
}

func TestCalculateBlackjackValue_TwoAces(t *testing.T) {
	// A + A = 12 (one as 11, one as 1)
	cards := []Card{
		{Rank: 0, Suit: 0}, // Ace
		{Rank: 0, Suit: 1}, // Ace
	}
	value := CalculateBlackjackValue(cards)
	if value != 12 {
		t.Errorf("Expected 12, got %d", value)
	}
}

func TestCalculateBlackjackValue_Bust(t *testing.T) {
	// K + Q + 5 = 25 (bust)
	cards := []Card{
		{Rank: 12, Suit: 0}, // K
		{Rank: 11, Suit: 1}, // Q
		{Rank: 4, Suit: 2},  // 5 (rank 4 = 5)
	}
	value := CalculateBlackjackValue(cards)
	if value != 25 {
		t.Errorf("Expected 25 (bust), got %d", value)
	}
}

func TestFindBestBlackjackWinner_HigherWins(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.NumPlayers = 2
	// Player 0: 20
	gs.Players[0].Hand = []Card{
		{Rank: 12, Suit: 0}, // K
		{Rank: 9, Suit: 1},  // 10 (rank 9 = 10)
	}
	// Player 1: 18
	gs.Players[1].Hand = []Card{
		{Rank: 0, Suit: 0}, // A (11)
		{Rank: 6, Suit: 1}, // 7 (rank 6 = 7)
	}

	winner := FindBestBlackjackWinner(gs, 2)
	if winner != 0 {
		t.Errorf("Expected player 0 to win with 20, got player %d", winner)
	}
}

func TestFindBestBlackjackWinner_BustLoses(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.NumPlayers = 2
	// Player 0: 25 (bust)
	gs.Players[0].Hand = []Card{
		{Rank: 12, Suit: 0}, // K
		{Rank: 11, Suit: 1}, // Q
		{Rank: 4, Suit: 2},  // 5 (rank 4 = 5)
	}
	// Player 1: 18
	gs.Players[1].Hand = []Card{
		{Rank: 0, Suit: 0}, // A (11)
		{Rank: 6, Suit: 1}, // 7 (rank 6 = 7)
	}

	winner := FindBestBlackjackWinner(gs, 2)
	if winner != 1 {
		t.Errorf("Expected player 1 to win (player 0 busted), got player %d", winner)
	}
}

func TestFindBestBlackjackWinner_BothBust(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.NumPlayers = 2
	// Player 0: 25 (bust)
	gs.Players[0].Hand = []Card{
		{Rank: 12, Suit: 0}, // K
		{Rank: 11, Suit: 1}, // Q
		{Rank: 4, Suit: 2},  // 5 (rank 4 = 5)
	}
	// Player 1: 23 (bust)
	gs.Players[1].Hand = []Card{
		{Rank: 12, Suit: 0}, // K
		{Rank: 9, Suit: 1},  // 10 (rank 9 = 10)
		{Rank: 2, Suit: 2},  // 3 (rank 2 = 3)
	}

	winner := FindBestBlackjackWinner(gs, 2)
	if winner != -1 {
		t.Errorf("Expected -1 (both busted), got player %d", winner)
	}
}

func TestFindBestBlackjackWinner_SkipsFoldedPlayers(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.NumPlayers = 2
	// Player 0: 21 but folded
	gs.Players[0].Hand = []Card{
		{Rank: 0, Suit: 0},  // A (11)
		{Rank: 12, Suit: 1}, // K (10)
	}
	gs.Players[0].HasFolded = true
	// Player 1: 18
	gs.Players[1].Hand = []Card{
		{Rank: 0, Suit: 0}, // A (11)
		{Rank: 6, Suit: 1}, // 7 (rank 6 = 7)
	}

	winner := FindBestBlackjackWinner(gs, 2)
	if winner != 1 {
		t.Errorf("Expected player 1 to win (player 0 folded), got player %d", winner)
	}
}

func TestIsBlackjackGame(t *testing.T) {
	// Create a genome with high_score win condition threshold 21
	genome := &Genome{
		WinConditions: []WinCondition{
			{WinType: 1, Threshold: 21},
		},
	}
	if !IsBlackjackGame(genome) {
		t.Error("Expected IsBlackjackGame to return true for high_score with threshold 21")
	}

	// Non-blackjack: empty_hand
	genome2 := &Genome{
		WinConditions: []WinCondition{
			{WinType: 0, Threshold: 0},
		},
	}
	if IsBlackjackGame(genome2) {
		t.Error("Expected IsBlackjackGame to return false for empty_hand")
	}

	// Non-blackjack: high_score with different threshold
	genome3 := &Genome{
		WinConditions: []WinCondition{
			{WinType: 1, Threshold: 100},
		},
	}
	if IsBlackjackGame(genome3) {
		t.Error("Expected IsBlackjackGame to return false for high_score with threshold 100")
	}
}

func TestSelectBlackjackMove_HitOnLow(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.NumPlayers = 2
	// Player 0: hand value 12 (should hit)
	gs.Players[0].Hand = []Card{
		{Rank: 4, Suit: 0}, // 5
		{Rank: 6, Suit: 1}, // 7
	}
	gs.CurrentPlayer = 0

	moves := []LegalMove{
		{PhaseIndex: 0, CardIndex: MoveDraw, TargetLoc: LocationDeck},     // hit
		{PhaseIndex: 0, CardIndex: MoveDrawPass, TargetLoc: LocationDeck}, // stand
	}

	idx := SelectBlackjackMove(gs, moves)
	if idx != 0 {
		t.Errorf("Expected to hit (idx 0) on hand value 12, got idx %d", idx)
	}
}

func TestSelectBlackjackMove_StandOnHigh(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.NumPlayers = 2
	// Player 0: hand value 18 (should stand)
	gs.Players[0].Hand = []Card{
		{Rank: 9, Suit: 0},  // 10
		{Rank: 7, Suit: 1},  // 8
	}
	gs.CurrentPlayer = 0

	moves := []LegalMove{
		{PhaseIndex: 0, CardIndex: MoveDraw, TargetLoc: LocationDeck},     // hit
		{PhaseIndex: 0, CardIndex: MoveDrawPass, TargetLoc: LocationDeck}, // stand
	}

	idx := SelectBlackjackMove(gs, moves)
	if idx != 1 {
		t.Errorf("Expected to stand (idx 1) on hand value 18, got idx %d", idx)
	}
}

func TestSelectBlackjackMove_StandOn17(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.NumPlayers = 2
	// Player 0: hand value 17 (should stand on 17)
	gs.Players[0].Hand = []Card{
		{Rank: 9, Suit: 0},  // 10
		{Rank: 6, Suit: 1},  // 7
	}
	gs.CurrentPlayer = 0

	moves := []LegalMove{
		{PhaseIndex: 0, CardIndex: MoveDraw, TargetLoc: LocationDeck},     // hit
		{PhaseIndex: 0, CardIndex: MoveDrawPass, TargetLoc: LocationDeck}, // stand
	}

	idx := SelectBlackjackMove(gs, moves)
	if idx != 1 {
		t.Errorf("Expected to stand (idx 1) on hand value 17, got idx %d", idx)
	}
}

func TestSelectBlackjackMove_HitOn16(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.NumPlayers = 2
	// Player 0: hand value 16 (should hit on 16)
	gs.Players[0].Hand = []Card{
		{Rank: 9, Suit: 0},  // 10
		{Rank: 5, Suit: 1},  // 6
	}
	gs.CurrentPlayer = 0

	moves := []LegalMove{
		{PhaseIndex: 0, CardIndex: MoveDraw, TargetLoc: LocationDeck},     // hit
		{PhaseIndex: 0, CardIndex: MoveDrawPass, TargetLoc: LocationDeck}, // stand
	}

	idx := SelectBlackjackMove(gs, moves)
	if idx != 0 {
		t.Errorf("Expected to hit (idx 0) on hand value 16, got idx %d", idx)
	}
}

func TestSelectBlackjackMove_SoftAce(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.NumPlayers = 2
	// Player 0: A + 6 = soft 17 (should stand)
	gs.Players[0].Hand = []Card{
		{Rank: 0, Suit: 0},  // Ace (11)
		{Rank: 5, Suit: 1},  // 6
	}
	gs.CurrentPlayer = 0

	moves := []LegalMove{
		{PhaseIndex: 0, CardIndex: MoveDraw, TargetLoc: LocationDeck},     // hit
		{PhaseIndex: 0, CardIndex: MoveDrawPass, TargetLoc: LocationDeck}, // stand
	}

	// Soft 17 = 17, should stand
	idx := SelectBlackjackMove(gs, moves)
	if idx != 1 {
		t.Errorf("Expected to stand (idx 1) on soft 17, got idx %d", idx)
	}
}
