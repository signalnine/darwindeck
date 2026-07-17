package sim

import "testing"

// TestChoiceSignatureDiscriminatesAmount pins the Meaningful probe's Amount
// sensitivity: MoveBid options differ ONLY in Move.Amount (the contract
// target), so an Amount-blind signature sampled every bid round as "all
// identical" and the marquee decision of a bidding game was never counted
// meaningful (Move.Key encodes Amount for MCTS; the probe was the only
// Amount-blind reader).
func TestChoiceSignatureDiscriminatesAmount(t *testing.T) {
	low := Move{Type: MoveBid, Amount: 0}
	high := Move{Type: MoveBid, Amount: 3}

	sigLow := choiceSignatureOf(nil, nil, nil, low, deltaModeTrickTaking, nil)
	sigHigh := choiceSignatureOf(nil, nil, nil, high, deltaModeTrickTaking, nil)
	if sigLow == sigHigh {
		t.Fatal("bids differing only in Amount must produce distinct choice signatures")
	}

	// End-to-end through turnIsMeaningful: a bid round's move list must read
	// as a meaningful decision.
	bids := []Move{
		{Type: MoveBid, Amount: 0},
		{Type: MoveBid, Amount: 1},
		{Type: MoveBid, Amount: 2},
	}
	if !turnIsMeaningful(nil, &GameState{}, nil, bids, deltaModeTrickTaking, nil) {
		t.Fatal("a multi-target bid round must be a meaningful decision")
	}
}
