package engine

import (
	"testing"
)

// Helper to check if a slice contains a specific BettingAction
func containsAction(moves []BettingAction, action BettingAction) bool {
	for _, m := range moves {
		if m == action {
			return true
		}
	}
	return false
}

func TestBettingMoves_NoCurrentBet(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

	moves := GenerateBettingMoves(gs, phase, 0)

	// Should have CHECK and BET, but not FOLD
	if !containsAction(moves, BettingCheck) {
		t.Error("Expected CHECK to be available when no current bet")
	}
	if !containsAction(moves, BettingBet) {
		t.Error("Expected BET to be available when player has enough chips")
	}
	if containsAction(moves, BettingFold) {
		t.Error("Expected FOLD to NOT be available when no current bet")
	}
	if containsAction(moves, BettingCall) {
		t.Error("Expected CALL to NOT be available when no current bet")
	}
}

func TestBettingMoves_WithCurrentBet(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.CurrentBet = 20
	phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

	moves := GenerateBettingMoves(gs, phase, 0)

	// Should have CALL, RAISE, and FOLD, but not CHECK
	if !containsAction(moves, BettingCall) {
		t.Error("Expected CALL to be available when there's a bet to match")
	}
	if !containsAction(moves, BettingRaise) {
		t.Error("Expected RAISE to be available when player has enough chips")
	}
	if !containsAction(moves, BettingFold) {
		t.Error("Expected FOLD to be available when there's a bet")
	}
	if containsAction(moves, BettingCheck) {
		t.Error("Expected CHECK to NOT be available when there's a bet to match")
	}
}

func TestBettingMoves_CantAffordCall(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 5
	gs.CurrentBet = 10
	phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

	moves := GenerateBettingMoves(gs, phase, 0)

	// Can't call, should have ALL_IN and FOLD
	if !containsAction(moves, BettingAllIn) {
		t.Error("Expected ALL_IN to be available when can't afford call but have chips")
	}
	if !containsAction(moves, BettingFold) {
		t.Error("Expected FOLD to be available")
	}
	if containsAction(moves, BettingCall) {
		t.Error("Expected CALL to NOT be available when can't afford it")
	}
}

func TestBettingMoves_CantAffordMinBet(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 5
	gs.CurrentBet = 0
	phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

	moves := GenerateBettingMoves(gs, phase, 0)

	// Should have CHECK and ALL_IN, but not BET
	if !containsAction(moves, BettingCheck) {
		t.Error("Expected CHECK to be available")
	}
	if !containsAction(moves, BettingAllIn) {
		t.Error("Expected ALL_IN to be available when can't afford min bet")
	}
	if containsAction(moves, BettingBet) {
		t.Error("Expected BET to NOT be available when can't afford min bet")
	}
}

func TestBettingMoves_MaxRaisesReached(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.CurrentBet = 20
	gs.RaiseCount = 3
	phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

	moves := GenerateBettingMoves(gs, phase, 0)

	// Should NOT have RAISE when max raises reached
	if containsAction(moves, BettingRaise) {
		t.Error("Expected RAISE to NOT be available when max raises reached")
	}
	if !containsAction(moves, BettingCall) {
		t.Error("Expected CALL to still be available")
	}
}

func TestBettingMoves_Folded(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.Players[0].HasFolded = true
	phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

	moves := GenerateBettingMoves(gs, phase, 0)

	// Folded player should have no moves
	if len(moves) != 0 {
		t.Errorf("Expected no moves for folded player, got %d", len(moves))
	}
}

func TestBettingMoves_AllIn(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.Players[0].IsAllIn = true
	phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

	moves := GenerateBettingMoves(gs, phase, 0)

	// All-in player should have no moves
	if len(moves) != 0 {
		t.Errorf("Expected no moves for all-in player, got %d", len(moves))
	}
}

func TestBettingMoves_NoChips(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 0
	phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

	moves := GenerateBettingMoves(gs, phase, 0)

	// Player with no chips should have no moves
	if len(moves) != 0 {
		t.Errorf("Expected no moves for player with no chips, got %d", len(moves))
	}
}

func TestBettingMoves_CanCallCantRaise(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 25
	gs.CurrentBet = 20
	phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

	moves := GenerateBettingMoves(gs, phase, 0)

	// Can afford to call (20), but not call+raise (20+10=30)
	if !containsAction(moves, BettingCall) {
		t.Error("Expected CALL to be available")
	}
	if containsAction(moves, BettingRaise) {
		t.Error("Expected RAISE to NOT be available when can't afford call+min_bet")
	}
	if !containsAction(moves, BettingFold) {
		t.Error("Expected FOLD to be available")
	}
}

func TestApplyBettingAction_Check(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	initialChips := gs.Players[0].Chips
	initialPot := gs.Pot
	phase := &BettingPhaseData{MinBet: 10}

	ApplyBettingAction(gs, phase, 0, BettingCheck)

	if gs.Players[0].Chips != initialChips {
		t.Errorf("CHECK should not change chips, got %d", gs.Players[0].Chips)
	}
	if gs.Pot != initialPot {
		t.Errorf("CHECK should not change pot, got %d", gs.Pot)
	}
}

func TestApplyBettingAction_Bet(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	phase := &BettingPhaseData{MinBet: 10}

	ApplyBettingAction(gs, phase, 0, BettingBet)

	if gs.Players[0].Chips != 90 {
		t.Errorf("Expected 90 chips after bet, got %d", gs.Players[0].Chips)
	}
	if gs.Players[0].CurrentBet != 10 {
		t.Errorf("Expected CurrentBet to be 10, got %d", gs.Players[0].CurrentBet)
	}
	if gs.Pot != 10 {
		t.Errorf("Expected pot to be 10, got %d", gs.Pot)
	}
	if gs.CurrentBet != 10 {
		t.Errorf("Expected CurrentBet on game state to be 10, got %d", gs.CurrentBet)
	}
}

func TestApplyBettingAction_Call(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.CurrentBet = 20
	gs.Pot = 20 // Assume someone already bet
	phase := &BettingPhaseData{MinBet: 10}

	ApplyBettingAction(gs, phase, 0, BettingCall)

	if gs.Players[0].Chips != 80 {
		t.Errorf("Expected 80 chips after call, got %d", gs.Players[0].Chips)
	}
	if gs.Players[0].CurrentBet != 20 {
		t.Errorf("Expected CurrentBet to be 20, got %d", gs.Players[0].CurrentBet)
	}
	if gs.Pot != 40 {
		t.Errorf("Expected pot to be 40, got %d", gs.Pot)
	}
}

func TestApplyBettingAction_Raise(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.CurrentBet = 20
	gs.Pot = 20
	phase := &BettingPhaseData{MinBet: 10}

	ApplyBettingAction(gs, phase, 0, BettingRaise)

	// Raise = call (20) + min_bet (10) = 30
	if gs.Players[0].Chips != 70 {
		t.Errorf("Expected 70 chips after raise, got %d", gs.Players[0].Chips)
	}
	if gs.Players[0].CurrentBet != 30 {
		t.Errorf("Expected CurrentBet to be 30, got %d", gs.Players[0].CurrentBet)
	}
	if gs.Pot != 50 {
		t.Errorf("Expected pot to be 50, got %d", gs.Pot)
	}
	if gs.CurrentBet != 30 {
		t.Errorf("Expected game CurrentBet to be 30, got %d", gs.CurrentBet)
	}
	if gs.RaiseCount != 1 {
		t.Errorf("Expected RaiseCount to be 1, got %d", gs.RaiseCount)
	}
}

func TestApplyBettingAction_AllIn(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 50
	phase := &BettingPhaseData{MinBet: 10}

	ApplyBettingAction(gs, phase, 0, BettingAllIn)

	if gs.Players[0].Chips != 0 {
		t.Errorf("Expected 0 chips after all-in, got %d", gs.Players[0].Chips)
	}
	if gs.Pot != 50 {
		t.Errorf("Expected pot to be 50, got %d", gs.Pot)
	}
	if !gs.Players[0].IsAllIn {
		t.Error("Expected player to be marked as all-in")
	}
}

func TestApplyBettingAction_AllIn_RaisesCurrentBet(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.CurrentBet = 20
	phase := &BettingPhaseData{MinBet: 10}

	ApplyBettingAction(gs, phase, 0, BettingAllIn)

	// All-in of 100 should raise the current bet
	if gs.CurrentBet != 100 {
		t.Errorf("Expected CurrentBet to be 100 after big all-in, got %d", gs.CurrentBet)
	}
	if gs.Players[0].CurrentBet != 100 {
		t.Errorf("Expected player CurrentBet to be 100, got %d", gs.Players[0].CurrentBet)
	}
}

func TestApplyBettingAction_AllIn_DoesntRaiseCurrentBet(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 15
	gs.CurrentBet = 50
	phase := &BettingPhaseData{MinBet: 10}

	ApplyBettingAction(gs, phase, 0, BettingAllIn)

	// All-in of 15 should NOT raise the current bet of 50
	if gs.CurrentBet != 50 {
		t.Errorf("Expected CurrentBet to remain 50 after small all-in, got %d", gs.CurrentBet)
	}
	if gs.Players[0].CurrentBet != 15 {
		t.Errorf("Expected player CurrentBet to be 15, got %d", gs.Players[0].CurrentBet)
	}
}

func TestApplyBettingAction_Fold(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	phase := &BettingPhaseData{MinBet: 10}

	ApplyBettingAction(gs, phase, 0, BettingFold)

	if !gs.Players[0].HasFolded {
		t.Error("Expected player to be marked as folded")
	}
	// Chips should not change on fold
	if gs.Players[0].Chips != 100 {
		t.Errorf("Expected chips to remain 100 after fold, got %d", gs.Players[0].Chips)
	}
}

func TestApplyBettingAction_RaiseIncrementsRaiseCount(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 200
	gs.Players[1].Chips = 200
	gs.CurrentBet = 10
	phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

	// First raise
	ApplyBettingAction(gs, phase, 0, BettingRaise)
	if gs.RaiseCount != 1 {
		t.Errorf("Expected RaiseCount to be 1, got %d", gs.RaiseCount)
	}

	// Second raise by different player
	ApplyBettingAction(gs, phase, 1, BettingRaise)
	if gs.RaiseCount != 2 {
		t.Errorf("Expected RaiseCount to be 2, got %d", gs.RaiseCount)
	}
}

func TestApplyBettingAction_CallWithPartialBet(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.Players[0].CurrentBet = 10 // Already put in 10
	gs.CurrentBet = 30
	gs.Pot = 40 // 10 from player 0, 30 from player 1
	phase := &BettingPhaseData{MinBet: 10}

	ApplyBettingAction(gs, phase, 0, BettingCall)

	// Only need to put in 20 more to match 30
	if gs.Players[0].Chips != 80 {
		t.Errorf("Expected 80 chips after call (was 100, paid 20), got %d", gs.Players[0].Chips)
	}
	if gs.Players[0].CurrentBet != 30 {
		t.Errorf("Expected CurrentBet to be 30, got %d", gs.Players[0].CurrentBet)
	}
	if gs.Pot != 60 {
		t.Errorf("Expected pot to be 60 (40+20), got %d", gs.Pot)
	}
}

func TestBettingMoves_MultiplePlayersWithDifferentChips(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.Players[1].Chips = 5
	gs.CurrentBet = 10
	phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

	// Player 0 with 100 chips
	moves0 := GenerateBettingMoves(gs, phase, 0)
	if !containsAction(moves0, BettingCall) {
		t.Error("Player 0 should be able to call")
	}
	if !containsAction(moves0, BettingRaise) {
		t.Error("Player 0 should be able to raise")
	}

	// Player 1 with only 5 chips
	moves1 := GenerateBettingMoves(gs, phase, 1)
	if containsAction(moves1, BettingCall) {
		t.Error("Player 1 should NOT be able to call (only 5 chips, need 10)")
	}
	if !containsAction(moves1, BettingAllIn) {
		t.Error("Player 1 should be able to go all-in")
	}
	if !containsAction(moves1, BettingFold) {
		t.Error("Player 1 should be able to fold")
	}
}

func TestBettingActionString(t *testing.T) {
	// Verify the iota values are as expected
	if BettingCheck != 0 {
		t.Errorf("Expected BettingCheck to be 0, got %d", BettingCheck)
	}
	if BettingBet != 1 {
		t.Errorf("Expected BettingBet to be 1, got %d", BettingBet)
	}
	if BettingCall != 2 {
		t.Errorf("Expected BettingCall to be 2, got %d", BettingCall)
	}
	if BettingRaise != 3 {
		t.Errorf("Expected BettingRaise to be 3, got %d", BettingRaise)
	}
	if BettingAllIn != 4 {
		t.Errorf("Expected BettingAllIn to be 4, got %d", BettingAllIn)
	}
	if BettingFold != 5 {
		t.Errorf("Expected BettingFold to be 5, got %d", BettingFold)
	}
}
