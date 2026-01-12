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

// Tests for round resolution functions

func TestCountActivePlayers(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	// All players active by default (4 players in pool)
	count := CountActivePlayers(gs)
	if count != 4 {
		t.Errorf("Expected 4 active players by default, got %d", count)
	}

	// Fold one player
	gs.Players[0].HasFolded = true
	count = CountActivePlayers(gs)
	if count != 3 {
		t.Errorf("Expected 3 active players after one fold, got %d", count)
	}

	// Fold another
	gs.Players[2].HasFolded = true
	count = CountActivePlayers(gs)
	if count != 2 {
		t.Errorf("Expected 2 active players after two folds, got %d", count)
	}
}

func TestCountActingPlayers(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	// Give players chips so they can act
	gs.Players[0].Chips = 100
	gs.Players[1].Chips = 100
	gs.Players[2].Chips = 100
	gs.Players[3].Chips = 100

	count := CountActingPlayers(gs)
	if count != 4 {
		t.Errorf("Expected 4 acting players, got %d", count)
	}

	// Fold one player
	gs.Players[0].HasFolded = true
	count = CountActingPlayers(gs)
	if count != 3 {
		t.Errorf("Expected 3 acting players after one fold, got %d", count)
	}

	// One player goes all-in
	gs.Players[1].IsAllIn = true
	count = CountActingPlayers(gs)
	if count != 2 {
		t.Errorf("Expected 2 acting players after one all-in, got %d", count)
	}

	// One player has no chips
	gs.Players[2].Chips = 0
	count = CountActingPlayers(gs)
	if count != 1 {
		t.Errorf("Expected 1 acting player after one has no chips, got %d", count)
	}
}

func TestAllBetsMatched(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.Players[1].Chips = 100
	gs.CurrentBet = 20

	// Both players have matched the current bet
	gs.Players[0].CurrentBet = 20
	gs.Players[1].CurrentBet = 20
	// Fold other players so they don't affect the test
	gs.Players[2].HasFolded = true
	gs.Players[3].HasFolded = true

	if !AllBetsMatched(gs) {
		t.Error("Expected bets to be matched when all players have CurrentBet == gs.CurrentBet")
	}
}

func TestAllBetsMatched_UnmatchedBet(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.Players[1].Chips = 100
	gs.CurrentBet = 20

	// Player 0 has matched, player 1 has not
	gs.Players[0].CurrentBet = 20
	gs.Players[1].CurrentBet = 10

	if AllBetsMatched(gs) {
		t.Error("Expected bets to NOT be matched when player 1 hasn't matched")
	}
}

func TestAllBetsMatched_FoldedPlayerIgnored(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.Players[1].Chips = 100
	gs.CurrentBet = 20

	// Player 0 matched, player 1 folded with unmatched bet
	gs.Players[0].CurrentBet = 20
	gs.Players[1].CurrentBet = 10
	gs.Players[1].HasFolded = true
	// Fold other players so they don't affect the test
	gs.Players[2].HasFolded = true
	gs.Players[3].HasFolded = true

	if !AllBetsMatched(gs) {
		t.Error("Expected bets to be matched when unmatched player has folded")
	}
}

func TestAllBetsMatched_AllInPlayerIgnored(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 100
	gs.Players[1].Chips = 0
	gs.CurrentBet = 20

	// Player 0 matched, player 1 is all-in with less than current bet
	gs.Players[0].CurrentBet = 20
	gs.Players[1].CurrentBet = 15
	gs.Players[1].IsAllIn = true
	// Fold other players so they don't affect the test
	gs.Players[2].HasFolded = true
	gs.Players[3].HasFolded = true

	if !AllBetsMatched(gs) {
		t.Error("Expected bets to be matched when unmatched player is all-in")
	}
}

func TestResolveShowdown_SingleWinner(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	// All but one player folded
	gs.Players[0].HasFolded = true
	gs.Players[1].HasFolded = false
	gs.Players[2].HasFolded = true
	gs.Players[3].HasFolded = true

	winners := ResolveShowdown(gs)

	if len(winners) != 1 {
		t.Errorf("Expected 1 winner, got %d", len(winners))
	}
	if winners[0] != 1 {
		t.Errorf("Expected player 1 to be winner, got player %d", winners[0])
	}
}

func TestResolveShowdown_MultipleActive(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	// Two players still active
	gs.Players[0].HasFolded = true
	gs.Players[1].HasFolded = false
	gs.Players[2].HasFolded = false
	gs.Players[3].HasFolded = true

	winners := ResolveShowdown(gs)

	if len(winners) != 2 {
		t.Errorf("Expected 2 active players, got %d", len(winners))
	}
	// Should contain players 1 and 2
	foundPlayer1 := false
	foundPlayer2 := false
	for _, w := range winners {
		if w == 1 {
			foundPlayer1 = true
		}
		if w == 2 {
			foundPlayer2 = true
		}
	}
	if !foundPlayer1 || !foundPlayer2 {
		t.Errorf("Expected winners to contain players 1 and 2, got %v", winners)
	}
}

func TestAwardPot_SingleWinner(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 50
	gs.Players[1].Chips = 50
	gs.Pot = 100

	AwardPot(gs, []int{0})

	if gs.Players[0].Chips != 150 {
		t.Errorf("Expected winner to have 150 chips, got %d", gs.Players[0].Chips)
	}
	if gs.Players[1].Chips != 50 {
		t.Errorf("Expected loser to still have 50 chips, got %d", gs.Players[1].Chips)
	}
	if gs.Pot != 0 {
		t.Errorf("Expected pot to be 0 after award, got %d", gs.Pot)
	}
}

func TestAwardPot_SplitEven(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 50
	gs.Players[1].Chips = 50
	gs.Pot = 100

	AwardPot(gs, []int{0, 1})

	// Each should get 50
	if gs.Players[0].Chips != 100 {
		t.Errorf("Expected player 0 to have 100 chips, got %d", gs.Players[0].Chips)
	}
	if gs.Players[1].Chips != 100 {
		t.Errorf("Expected player 1 to have 100 chips, got %d", gs.Players[1].Chips)
	}
	if gs.Pot != 0 {
		t.Errorf("Expected pot to be 0 after award, got %d", gs.Pot)
	}
}

func TestAwardPot_OddRemainder(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 50
	gs.Players[1].Chips = 50
	gs.Pot = 101 // Odd pot

	AwardPot(gs, []int{0, 1})

	// 101 / 2 = 50 each, remainder 1 goes to first winner
	if gs.Players[0].Chips != 101 { // 50 + 50 + 1
		t.Errorf("Expected player 0 (first winner) to have 101 chips, got %d", gs.Players[0].Chips)
	}
	if gs.Players[1].Chips != 100 { // 50 + 50
		t.Errorf("Expected player 1 to have 100 chips, got %d", gs.Players[1].Chips)
	}
	if gs.Pot != 0 {
		t.Errorf("Expected pot to be 0 after award, got %d", gs.Pot)
	}
}

func TestAwardPot_EmptyWinners(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Pot = 100
	initialPot := gs.Pot

	AwardPot(gs, []int{})

	// Pot should remain unchanged
	if gs.Pot != initialPot {
		t.Errorf("Expected pot to remain %d with no winners, got %d", initialPot, gs.Pot)
	}
}

func TestAwardPot_ThreeWayOddSplit(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	gs.Players[0].Chips = 0
	gs.Players[1].Chips = 0
	gs.Players[2].Chips = 0
	gs.Pot = 100 // 100 / 3 = 33 each, remainder 1

	AwardPot(gs, []int{0, 1, 2})

	// 100 / 3 = 33 each, remainder 1 goes to first winner
	if gs.Players[0].Chips != 34 {
		t.Errorf("Expected player 0 to have 34 chips, got %d", gs.Players[0].Chips)
	}
	if gs.Players[1].Chips != 33 {
		t.Errorf("Expected player 1 to have 33 chips, got %d", gs.Players[1].Chips)
	}
	if gs.Players[2].Chips != 33 {
		t.Errorf("Expected player 2 to have 33 chips, got %d", gs.Players[2].Chips)
	}
	if gs.Pot != 0 {
		t.Errorf("Expected pot to be 0 after award, got %d", gs.Pot)
	}
}

// ============================================================================
// AI Betting Selection Tests
// ============================================================================

func TestSelectRandomBettingAction(t *testing.T) {
	moves := []BettingAction{BettingCheck, BettingBet, BettingFold}

	// Test with deterministic rng that always returns 0
	result := SelectRandomBettingAction(moves, func(n int) int { return 0 })
	if result != BettingCheck {
		t.Errorf("Expected BettingCheck (first element), got %d", result)
	}

	// Test with deterministic rng that returns 1
	result = SelectRandomBettingAction(moves, func(n int) int { return 1 })
	if result != BettingBet {
		t.Errorf("Expected BettingBet (second element), got %d", result)
	}

	// Test with deterministic rng that returns last index
	result = SelectRandomBettingAction(moves, func(n int) int { return n - 1 })
	if result != BettingFold {
		t.Errorf("Expected BettingFold (last element), got %d", result)
	}
}

func TestSelectRandomBettingAction_EmptyMoves(t *testing.T) {
	moves := []BettingAction{}

	result := SelectRandomBettingAction(moves, func(n int) int { return 0 })
	if result != BettingFold {
		t.Errorf("Expected BettingFold as fallback for empty moves, got %d", result)
	}
}

func TestSelectGreedyBettingAction_StrongHand(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	// Strong hand (0.8 > 0.7) should prefer Raise > Bet > AllIn
	strongHandStrength := 0.8

	// Test with Raise available
	moves := []BettingAction{BettingCall, BettingRaise, BettingFold}
	result := SelectGreedyBettingAction(gs, moves, strongHandStrength)
	if result != BettingRaise {
		t.Errorf("Strong hand with Raise available: expected BettingRaise, got %d", result)
	}

	// Test without Raise but with Bet
	moves = []BettingAction{BettingCheck, BettingBet}
	result = SelectGreedyBettingAction(gs, moves, strongHandStrength)
	if result != BettingBet {
		t.Errorf("Strong hand with Bet available: expected BettingBet, got %d", result)
	}

	// Test without Raise or Bet but with AllIn
	moves = []BettingAction{BettingCheck, BettingAllIn}
	result = SelectGreedyBettingAction(gs, moves, strongHandStrength)
	if result != BettingAllIn {
		t.Errorf("Strong hand with only AllIn: expected BettingAllIn, got %d", result)
	}
}

func TestSelectGreedyBettingAction_MediumHand(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	// Medium hand (0.5 > 0.3 but <= 0.7) should prefer Call > Check
	mediumHandStrength := 0.5

	// Test with Call available
	moves := []BettingAction{BettingCall, BettingRaise, BettingFold}
	result := SelectGreedyBettingAction(gs, moves, mediumHandStrength)
	if result != BettingCall {
		t.Errorf("Medium hand with Call available: expected BettingCall, got %d", result)
	}

	// Test without Call but with Check
	moves = []BettingAction{BettingCheck, BettingBet}
	result = SelectGreedyBettingAction(gs, moves, mediumHandStrength)
	if result != BettingCheck {
		t.Errorf("Medium hand with Check available: expected BettingCheck, got %d", result)
	}
}

func TestSelectGreedyBettingAction_WeakHand(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	// Weak hand (<= 0.3) should prefer Check > Fold
	weakHandStrength := 0.2

	// Test with Check available
	moves := []BettingAction{BettingCheck, BettingBet, BettingFold}
	result := SelectGreedyBettingAction(gs, moves, weakHandStrength)
	if result != BettingCheck {
		t.Errorf("Weak hand with Check available: expected BettingCheck, got %d", result)
	}

	// Test without Check - should Fold
	moves = []BettingAction{BettingCall, BettingRaise, BettingFold}
	result = SelectGreedyBettingAction(gs, moves, weakHandStrength)
	if result != BettingFold {
		t.Errorf("Weak hand without Check: expected BettingFold, got %d", result)
	}
}

func TestSelectGreedyBettingAction_VeryWeakHand(t *testing.T) {
	gs := GetState()
	defer PutState(gs)

	// Very weak hand (0.0) should check if possible, otherwise fold
	veryWeakHandStrength := 0.0

	// With check available
	moves := []BettingAction{BettingCheck, BettingBet}
	result := SelectGreedyBettingAction(gs, moves, veryWeakHandStrength)
	if result != BettingCheck {
		t.Errorf("Very weak hand with Check: expected BettingCheck, got %d", result)
	}

	// Without check available
	moves = []BettingAction{BettingCall, BettingFold}
	result = SelectGreedyBettingAction(gs, moves, veryWeakHandStrength)
	if result != BettingFold {
		t.Errorf("Very weak hand without Check: expected BettingFold, got %d", result)
	}
}

func TestEvaluateHandStrength_HighCard(t *testing.T) {
	// Low card only - should have low score
	hand := []Card{
		{Rank: 2, Suit: 0}, // 4 (rank 2 = "4" in real card)
	}
	strength := EvaluateHandStrength(hand)

	// High card of 4 (rank 2) -> 2/13 * 0.4 = ~0.062
	// No pairs -> 0
	// Total ~0.062
	if strength >= 0.2 {
		t.Errorf("High card 4 should have low strength (< 0.2), got %f", strength)
	}
}

func TestEvaluateHandStrength_HighCardAce(t *testing.T) {
	// Ace high card - should have higher score
	hand := []Card{
		{Rank: 0, Suit: 0}, // Ace
		{Rank: 2, Suit: 1}, // 4
	}
	strength := EvaluateHandStrength(hand)

	// Ace (rank 0 -> effective 13) -> 13/13 * 0.4 = 0.4
	// No pairs -> 0
	// Total 0.4
	if strength < 0.35 || strength > 0.45 {
		t.Errorf("Ace high should have strength around 0.4, got %f", strength)
	}
}

func TestEvaluateHandStrength_Pair(t *testing.T) {
	// Pair of 7s
	hand := []Card{
		{Rank: 5, Suit: 0}, // 7
		{Rank: 5, Suit: 1}, // 7
		{Rank: 2, Suit: 2}, // 4
	}
	strength := EvaluateHandStrength(hand)

	// Pair (maxCount=2) -> (2-1) * 0.2 = 0.2
	// High card 7 (rank 5) -> 5/13 * 0.4 = ~0.154
	// Total ~0.354
	if strength < 0.3 || strength > 0.45 {
		t.Errorf("Pair of 7s should have medium strength (0.3-0.45), got %f", strength)
	}
}

func TestEvaluateHandStrength_Trips(t *testing.T) {
	// Three of a kind (trips)
	hand := []Card{
		{Rank: 8, Suit: 0}, // 10
		{Rank: 8, Suit: 1}, // 10
		{Rank: 8, Suit: 2}, // 10
		{Rank: 2, Suit: 3}, // 4
	}
	strength := EvaluateHandStrength(hand)

	// Trips (maxCount=3) -> (3-1) * 0.2 = 0.4
	// High card 10 (rank 8) -> 8/13 * 0.4 = ~0.246
	// Total ~0.646
	if strength < 0.55 || strength > 0.75 {
		t.Errorf("Trips should have high strength (0.55-0.75), got %f", strength)
	}
}

func TestEvaluateHandStrength_Quads(t *testing.T) {
	// Four of a kind (quads)
	hand := []Card{
		{Rank: 10, Suit: 0}, // Jack
		{Rank: 10, Suit: 1}, // Jack
		{Rank: 10, Suit: 2}, // Jack
		{Rank: 10, Suit: 3}, // Jack
	}
	strength := EvaluateHandStrength(hand)

	// Quads (maxCount=4) -> (4-1) * 0.2 = 0.6
	// High card Jack (rank 10) -> 10/13 * 0.4 = ~0.308
	// Total ~0.908
	if strength < 0.8 || strength > 1.0 {
		t.Errorf("Quads should have very high strength (0.8-1.0), got %f", strength)
	}
}

func TestEvaluateHandStrength_EmptyHand(t *testing.T) {
	hand := []Card{}
	strength := EvaluateHandStrength(hand)

	if strength != 0.0 {
		t.Errorf("Empty hand should have strength 0.0, got %f", strength)
	}
}

func TestEvaluateHandStrength_PairOfAces(t *testing.T) {
	// Pair of Aces - should be strong
	hand := []Card{
		{Rank: 0, Suit: 0}, // Ace
		{Rank: 0, Suit: 1}, // Ace
	}
	strength := EvaluateHandStrength(hand)

	// Pair (maxCount=2) -> (2-1) * 0.2 = 0.2
	// Ace high (rank 0 -> effective 13) -> 13/13 * 0.4 = 0.4
	// Total 0.6
	if strength < 0.55 || strength > 0.65 {
		t.Errorf("Pair of Aces should have strength around 0.6, got %f", strength)
	}
}

func TestContainsBettingAction(t *testing.T) {
	moves := []BettingAction{BettingCheck, BettingBet, BettingFold}

	if !containsBettingAction(moves, BettingCheck) {
		t.Error("Expected containsBettingAction to find BettingCheck")
	}
	if !containsBettingAction(moves, BettingBet) {
		t.Error("Expected containsBettingAction to find BettingBet")
	}
	if !containsBettingAction(moves, BettingFold) {
		t.Error("Expected containsBettingAction to find BettingFold")
	}
	if containsBettingAction(moves, BettingRaise) {
		t.Error("Expected containsBettingAction to NOT find BettingRaise")
	}
	if containsBettingAction(moves, BettingCall) {
		t.Error("Expected containsBettingAction to NOT find BettingCall")
	}
	if containsBettingAction(moves, BettingAllIn) {
		t.Error("Expected containsBettingAction to NOT find BettingAllIn")
	}
}

func TestContainsBettingAction_EmptySlice(t *testing.T) {
	moves := []BettingAction{}

	if containsBettingAction(moves, BettingCheck) {
		t.Error("Expected containsBettingAction to return false for empty slice")
	}
}
